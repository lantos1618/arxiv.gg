package arxiv

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestAPIKeyPostgresRotationAndRollback(t *testing.T) {
	cache := apiKeyPostgresTestCache(t)
	ctx := context.Background()
	_, oldRaw, err := cache.CreateUserAPIKey(ctx, "rotation-test", "Agent access")
	if err != nil {
		t.Fatal(err)
	}
	newKey, newRaw, err := cache.RegenerateUserAPIKey(ctx, "rotation-test", "Agent access")
	if err != nil {
		t.Fatalf("rotate on PostgreSQL: %v", err)
	}
	if _, _, err := cache.UserForAPIKey(ctx, oldRaw); err != gorm.ErrRecordNotFound {
		t.Fatalf("old key must be revoked: %v", err)
	}
	if _, key, err := cache.UserForAPIKey(ctx, newRaw); err != nil || key.ID != newKey.ID {
		t.Fatalf("new key must authenticate: %v", err)
	}
	// Force creation to fail after the old key's revocation was attempted.
	if err := cache.db.Exec(`ALTER TABLE user_api_keys ADD CONSTRAINT reject_new_keys CHECK (id = '` + newKey.ID + `') NOT VALID`).Error; err != nil {
		t.Fatal(err)
	}
	if _, raw, err := cache.RegenerateUserAPIKey(ctx, "rotation-test", "Agent access"); err == nil || raw != "" {
		t.Fatal("failed rotation must return an error and no raw key")
	}
	if _, key, err := cache.UserForAPIKey(ctx, newRaw); err != nil || key.ID != newKey.ID {
		t.Fatalf("failed replacement must preserve current key: %v", err)
	}
}

func apiKeyPostgresTestCache(t *testing.T) *Cache {
	t.Helper()
	cache := quickSearchPostgresTestCache(t)
	for _, sql := range []string{
		`CREATE TEMP TABLE users (id text PRIMARY KEY)`,
		`INSERT INTO users VALUES ('rotation-test')`,
		`CREATE TEMP TABLE user_api_keys (
   id text PRIMARY KEY, user_id text, name text, key_hash text UNIQUE,
   prefix text, last_four text, created_at timestamptz, updated_at timestamptz,
   last_used_at timestamptz, revoked_at timestamptz)`,
		`CREATE UNIQUE INDEX ON user_api_keys (user_id, name) WHERE revoked_at IS NULL`,
	} {
		if err := cache.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	return cache
}

func TestAPIKeyPostgresIndependentKeys(t *testing.T) {
	cache := apiKeyPostgresTestCache(t)
	ctx := context.Background()
	first, firstRaw, err := cache.CreateUserAPIKey(ctx, "rotation-test", "Claude Desktop")
	if err != nil {
		t.Fatal(err)
	}
	second, secondRaw, err := cache.CreateUserAPIKey(ctx, "rotation-test", "Research script")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.RegenerateUserAPIKeyByID(ctx, "other-user", first.ID); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-account rotation: %v", err)
	}
	if err := cache.RevokeUserAPIKey(ctx, "other-user", second.ID); err != gorm.ErrRecordNotFound {
		t.Fatalf("cross-account revocation: %v", err)
	}
	replacement, raw, err := cache.RegenerateUserAPIKeyByID(ctx, "rotation-test", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Name != first.Name {
		t.Fatal("rotation changed key name")
	}
	if _, _, err := cache.UserForAPIKey(ctx, firstRaw); err != gorm.ErrRecordNotFound {
		t.Fatal("rotated key still authenticates")
	}
	if _, _, err := cache.UserForAPIKey(ctx, secondRaw); err != nil {
		t.Fatal("rotation affected another key")
	}
	if _, _, err := cache.RegenerateUserAPIKeyByID(ctx, "rotation-test", first.ID); err != gorm.ErrRecordNotFound {
		t.Fatal("stale rotation was accepted")
	}
	if err := cache.RevokeUserAPIKey(ctx, "rotation-test", second.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cache.UserForAPIKey(ctx, secondRaw); err != gorm.ErrRecordNotFound {
		t.Fatal("revoked key still authenticates")
	}
	if _, _, err := cache.UserForAPIKey(ctx, raw); err != nil {
		t.Fatal("revocation affected another key")
	}
	keys, err := cache.ActiveUserAPIKeys(ctx, "rotation-test")
	if err != nil || len(keys) != 1 || keys[0].ID != replacement.ID {
		t.Fatalf("incorrect active keys: %v", err)
	}
	others, err := cache.ActiveUserAPIKeys(ctx, "other-user")
	if err != nil || len(others) != 0 {
		t.Fatal("list leaked another user's keys")
	}
	// Insert failure must roll back the selected key's revocation.
	if err := cache.db.Exec(`ALTER TABLE user_api_keys ADD CONSTRAINT reject_replacement CHECK (id = '` + replacement.ID + `') NOT VALID`).Error; err != nil {
		t.Fatal(err)
	}
	if _, value, err := cache.RegenerateUserAPIKeyByID(ctx, "rotation-test", replacement.ID); err == nil || value != "" {
		t.Fatal("expected failed replacement")
	}
	if _, _, err := cache.UserForAPIKey(ctx, raw); err != nil {
		t.Fatal("failed replacement revoked the current key")
	}
}
