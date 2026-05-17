package main

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"time"
)

const adminCookieName = "arxiv_admin_token"

type adminTokenSource int

const (
	adminTokenMissing adminTokenSource = iota
	adminTokenCookie
	adminTokenHeader
	adminTokenBearer
	adminTokenQuery
)

func (s *server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		writeAdminDenied(w, r, http.StatusNotFound, "admin endpoints are disabled")
		return false
	}

	provided, source := adminTokenFromRequest(r)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
		writeAdminDenied(w, r, http.StatusUnauthorized, "admin token required")
		return false
	}

	if source == adminTokenQuery && !strings.HasPrefix(r.URL.Path, "/api/") {
		setAdminCookie(w, r, provided)
	}

	return true
}

func (s *server) hasAdminAccess(r *http.Request) bool {
	token := os.Getenv("ADMIN_TOKEN")
	if token == "" {
		return false
	}
	provided, _ := adminTokenFromRequest(r)
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

func setAdminCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     adminCookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(12 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func adminTokenFromRequest(r *http.Request) (string, adminTokenSource) {
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		return cookie.Value, adminTokenCookie
	}

	if provided := r.Header.Get("X-Admin-Token"); provided != "" {
		return provided, adminTokenHeader
	}

	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):]), adminTokenBearer
	}

	if provided := r.URL.Query().Get("admin_token"); provided != "" {
		return provided, adminTokenQuery
	}

	return "", adminTokenMissing
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
