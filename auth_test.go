package arxiv

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestNormalizeEmail(t *testing.T) {
	got, err := NormalizeEmail("  Researcher <Ada@Example.EDU> ")
	if err != nil {
		t.Fatalf("NormalizeEmail: %v", err)
	}
	if got != "ada@example.edu" {
		t.Fatalf("NormalizeEmail = %q", got)
	}

	if _, err := NormalizeEmail("not an email"); err == nil {
		t.Fatal("expected invalid email error")
	}
}

func TestLoginCodeCreatesUserAndSession(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cache, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	_, code, err := cache.CreateLoginCode(ctx, "Ada@Example.EDU", time.Minute)
	if err != nil {
		t.Fatalf("CreateLoginCode: %v", err)
	}

	if _, err := cache.ConsumeLoginCode(ctx, "ada@example.edu", "000000"); err == nil {
		t.Fatal("expected wrong code to fail")
	}

	user, err := cache.ConsumeLoginCode(ctx, "ada@example.edu", code)
	if err != nil {
		t.Fatalf("ConsumeLoginCode: %v", err)
	}
	if user.Email != "ada@example.edu" || user.Plan != "free" || user.AuthProvider != "email" {
		t.Fatalf("unexpected user: %#v", user)
	}

	if _, err := cache.ConsumeLoginCode(ctx, "ada@example.edu", code); err == nil {
		t.Fatal("expected reused code to fail")
	}

	token, err := cache.CreateUserSession(ctx, user.ID, "127.0.0.1", "test-agent", time.Minute)
	if err != nil {
		t.Fatalf("CreateUserSession: %v", err)
	}
	sessionUser, err := cache.UserForSessionToken(ctx, token)
	if err != nil {
		t.Fatalf("UserForSessionToken: %v", err)
	}
	if sessionUser.ID != user.ID {
		t.Fatalf("session user ID = %q, want %q", sessionUser.ID, user.ID)
	}

	if err := cache.RevokeUserSession(ctx, token); err != nil {
		t.Fatalf("RevokeUserSession: %v", err)
	}
	if _, err := cache.UserForSessionToken(ctx, token); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected revoked session to be missing, got %v", err)
	}
}
