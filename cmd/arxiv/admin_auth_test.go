package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdminSetsCookieOnlyForBrowserQueryLogin(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "test-admin-token")

	tests := []struct {
		name       string
		path       string
		setup      func(*http.Request)
		wantCookie bool
	}{
		{
			name: "existing cookie",
			path: "/admin/embeddings",
			setup: func(r *http.Request) {
				r.AddCookie(&http.Cookie{Name: adminCookieName, Value: "test-admin-token"})
			},
		},
		{
			name: "x admin token header",
			path: "/admin/embeddings",
			setup: func(r *http.Request) {
				r.Header.Set("X-Admin-Token", "test-admin-token")
			},
		},
		{
			name: "bearer token",
			path: "/admin/embeddings",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer test-admin-token")
			},
		},
		{
			name: "api query token",
			path: "/api/v1/embeddings/generate?admin_token=test-admin-token",
		},
		{
			name:       "browser query token",
			path:       "/admin/embeddings?admin_token=test-admin-token",
			wantCookie: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.Header.Set("X-Forwarded-Proto", "https")
			if tt.setup != nil {
				tt.setup(req)
			}

			rec := httptest.NewRecorder()
			if ok := (&server{}).requireAdmin(rec, req); !ok {
				t.Fatal("requireAdmin returned false")
			}

			cookies := rec.Result().Cookies()
			if tt.wantCookie {
				if len(cookies) != 1 {
					t.Fatalf("expected one cookie, got %d", len(cookies))
				}
				if cookies[0].Name != adminCookieName {
					t.Fatalf("expected cookie %q, got %q", adminCookieName, cookies[0].Name)
				}
				return
			}
			if len(cookies) != 0 {
				t.Fatalf("expected no cookie, got %d", len(cookies))
			}
		})
	}
}

func TestRequireAdminRedirectsBrowserToLoginWhenAdminEmailsConfigured(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("ADMIN_EMAILS", "owner@example.com")

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	rec := httptest.NewRecorder()

	if ok := (&server{}).requireAdmin(rec, req); ok {
		t.Fatal("requireAdmin returned true")
	}
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "/login?next=%2Fadmin%2Fusers" {
		t.Fatalf("unexpected redirect: %q", got)
	}
}

func TestRequireAdminKeepsAPITokenGateWhenAdminEmailsConfigured(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "")
	t.Setenv("ADMIN_EMAILS", "owner@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/embeddings/generate", nil)
	rec := httptest.NewRecorder()

	if ok := (&server{}).requireAdmin(rec, req); ok {
		t.Fatal("requireAdmin returned true")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
