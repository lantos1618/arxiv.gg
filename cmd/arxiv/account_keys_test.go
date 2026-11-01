package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

func TestMultipleAccountKeys(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	cache, err := arxiv.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	ctx := context.Background()
	owner, err := cache.FindOrCreateUser(ctx, "keys@example.test", "Reader", "", true, "google", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	other, err := cache.FindOrCreateUser(ctx, "other@example.test", "Other", "", true, "google", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	token, err := cache.CreateUserSession(ctx, owner.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	unrelated, unrelatedRaw, err := cache.CreateUserAPIKey(ctx, other.ID, "Private key")
	if err != nil {
		t.Fatal(err)
	}
	original, originalRaw, err := cache.CreateUserAPIKey(ctx, owner.ID, "Existing tool")
	if err != nil {
		t.Fatal(err)
	}
	server := &server{cache: cache}
	post := func(path string, values url.Values, cookie bool, bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if cookie {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		switch path {
		case "/account/api-key/create":
			server.handleCreateAPIKey(rec, req)
		case "/account/api-key/revoke":
			server.handleRevokeAPIKey(rec, req)
		default:
			server.handleRegenerateAPIKey(rec, req)
		}
		if rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("key response is cacheable")
		}
		return rec
	}
	for _, path := range []string{"/account/api-key/create", "/account/api-key/regenerate", "/account/api-key/revoke"} {
		if rec := post(path, url.Values{}, false, originalRaw); rec.Code != http.StatusSeeOther {
			t.Fatalf("API key managed keys at %s", path)
		}
	}
	rec := post("/account/api-key/create", url.Values{"name": {"Claude Desktop"}}, true, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Your new key is ready") {
		t.Fatal("creation failed")
	}
	keys, err := cache.ActiveUserAPIKeys(ctx, owner.ID)
	if err != nil || len(keys) != 2 {
		t.Fatal("creation did not preserve both keys")
	}
	if _, _, err := cache.UserForAPIKey(ctx, originalRaw); err != nil {
		t.Fatal("creation revoked original")
	}
	for _, name := range []string{"Claude Desktop", " ", strings.Repeat("x", 121), "bad\x00name"} {
		rec = post("/account/api-key/create", url.Values{"name": {name}}, true, "")
		if !strings.Contains(rec.Body.String(), `role="alert"`) {
			t.Fatalf("invalid or duplicate name accepted: %q", name)
		}
	}
	unicodeName := strings.Repeat("研究", 60)
	post("/account/api-key/create", url.Values{"name": {unicodeName}}, true, "")
	key, err := cache.ActiveUserAPIKey(ctx, owner.ID, unicodeName)
	if err != nil || key == nil || key.Name != unicodeName {
		t.Fatal("Unicode name was truncated")
	}
	for _, id := range []string{original.ID, unrelated.ID} {
		// Missing confirmation must never revoke an owned or unowned key.
		post("/account/api-key/revoke", url.Values{"key_id": {id}}, true, "")
	}
	post("/account/api-key/revoke", url.Values{"key_id": {unrelated.ID}, "confirm_revocation": {"revoke"}}, true, "")
	post("/account/api-key/regenerate", url.Values{"key_id": {unrelated.ID}, "confirm_rotation": {"rotate"}}, true, "")
	if _, _, err := cache.UserForAPIKey(ctx, unrelatedRaw); err != nil {
		t.Fatal("another user's key was changed")
	}
	if _, _, err := cache.UserForAPIKey(ctx, originalRaw); err != nil {
		t.Fatal("unconfirmed revocation changed original")
	}
	rec = post("/account/api-key/revoke", url.Values{"key_id": {original.ID}, "confirm_revocation": {"revoke"}}, true, "")
	if !strings.Contains(rec.Body.String(), `role="status"`) {
		t.Fatal("missing revocation success message")
	}
	if _, _, err := cache.UserForAPIKey(ctx, originalRaw); err == nil {
		t.Fatal("revoked key still works")
	}
	keys, err = cache.ActiveUserAPIKeys(ctx, owner.ID)
	if err != nil || len(keys) != 2 {
		t.Fatal("revocation affected other keys")
	}
}
