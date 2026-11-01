package arxiv

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestAPIKeyPostgresRotationAndRollback(t *testing.T) {
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
