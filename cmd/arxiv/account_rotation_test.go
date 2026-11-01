package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

func TestAccountRotationConfirmationAndOneTimeDisclosure(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	ctx := context.Background()
	user, err := cache.FindOrCreateUser(ctx, "rotation@example.test", "Reader", "", true, "google", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, oldRaw, err := cache.CreateUserAPIKey(ctx, user.ID, "Agent access")
	if err != nil {
		t.Fatal(err)
	}
	token, err := cache.CreateUserSession(ctx, user.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{cache: cache}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		rec := httptest.NewRecorder()
		if method == http.MethodPost {
			s.handleRegenerateAPIKey(rec, req)
		} else {
			s.handleAccount(rec, req)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("unexpected status %d", rec.Code)
		}
		if rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("account response must not be cached")
		}
		return rec
	}
	unconfirmed := request(http.MethodPost, "/account/api-key/regenerate", "")
	if !strings.Contains(unconfirmed.Body.String(), `role="alert"`) {
		t.Fatal("missing confirmation warning")
	}
	if _, _, err := cache.UserForAPIKey(ctx, oldRaw); err != nil {
		t.Fatal("unconfirmed request revoked existing key")
	}
	rotated := request(http.MethodPost, "/account/api-key/regenerate", "confirm_rotation=rotate")
	match := regexp.MustCompile(`id="account-api-key"[^>]*value="([^"]+)"`).FindStringSubmatch(rotated.Body.String())
	if len(match) != 2 {
		t.Fatal("replacement key was not disclosed")
	}
	if _, _, err := cache.UserForAPIKey(ctx, match[1]); err != nil {
		t.Fatal("replacement key does not authenticate")
	}
	if _, _, err := cache.UserForAPIKey(ctx, oldRaw); err == nil {
		t.Fatal("old key still authenticates")
	}
	account := request(http.MethodGet, "/account", "")
	if strings.Contains(account.Body.String(), match[1]) {
		t.Fatal("subsequent account view disclosed the raw key")
	}
}
