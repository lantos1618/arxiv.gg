package main

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"
)

const adminCookieName = "arxiv_admin_token"

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		writeAdminDenied(w, r, http.StatusNotFound, "admin endpoints are disabled")
		return false
	}

	provided, fromCookie := adminTokenFromRequest(r)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
		writeAdminDenied(w, r, http.StatusUnauthorized, "admin token required")
		return false
	}

	if !fromCookie {
		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		http.SetCookie(w, &http.Cookie{
			Name:     adminCookieName,
			Value:    provided,
			Path:     "/",
			Expires:  time.Now().Add(12 * time.Hour),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
	}

	return true
}

func adminTokenFromRequest(r *http.Request) (string, bool) {
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		return cookie.Value, true
	}

	if provided := r.Header.Get("X-Admin-Token"); provided != "" {
		return provided, false
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):]), false
	}

	return r.URL.Query().Get("admin_token"), false
}

func writeAdminDenied(w http.ResponseWriter, r *http.Request, status int, message string) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		respondJSON(w, status, APIResponse{
			Success: false,
			Error:   message,
		})
		return
	}
	http.Error(w, message, status)
}
