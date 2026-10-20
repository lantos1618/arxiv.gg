package main

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/lantos1618/arxiv.gg"
)

var adSenseClientPattern = regexp.MustCompile(`^ca-pub-[0-9]{16}$`)
var adSenseSlotPattern = regexp.MustCompile(`^[0-9]{1,20}$`)

func configuredAdSenseAdsEnabled() bool {
	return os.Getenv("GOOGLE_ADSENSE_ADS_ENABLED") == "true" &&
		os.Getenv("GOOGLE_ADSENSE_CONSENT_READY") == "true"
}

func configuredAdSenseAdSlot() string {
	slot := strings.TrimSpace(os.Getenv("GOOGLE_ADSENSE_AD_SLOT"))
	if !adSenseSlotPattern.MatchString(slot) {
		return ""
	}
	return slot
}

func (s *server) adSenseServingConfigured() bool {
	return !s.localMode && s.googleAdSenseAdsEnabled &&
		adSenseClientPattern.MatchString(s.googleAdSenseClientID) &&
		adSenseSlotPattern.MatchString(s.googleAdSenseAdSlot)
}

func adSensePageAllowed(path, name string) bool {
	return (path == "/" && name == "index") ||
		(strings.HasPrefix(path, "/abs/") && name == "paper") ||
		(path == "/privacy" && name == "privacy")
}

// Google's supported AdSense CSP uses nonces and strict-dynamic rather than a
// domain allowlist. Only ad-enabled pages use it. Other resource types cannot
// use default-src 'self': creatives and the CMP load resources across domains.
// https://support.google.com/adsense/answer/16283098
func adSenseContentSecurityPolicy(nonce string) string {
	return strings.Join([]string{
		"object-src 'none'", "base-uri 'none'", "frame-ancestors 'none'", "form-action 'self'",
		"script-src 'nonce-" + nonce + "' 'unsafe-inline' 'unsafe-eval' 'strict-dynamic' https: http:",
		"upgrade-insecure-requests",
	}, "; ")
}

func configuredAdSenseClientID() string {
	id := strings.TrimSpace(os.Getenv("GOOGLE_ADSENSE_CLIENT_ID"))
	if !adSenseClientPattern.MatchString(id) {
		return ""
	}
	return id
}

// Ownership verification does not load advertising scripts or set cookies.
// Ad serving is enabled separately after site review and consent configuration.
func (s *server) handleAdsTXT(w http.ResponseWriter, r *http.Request) {
	if rejectNonGetHead(w, r) {
		return
	}
	if !adSenseClientPattern.MatchString(s.googleAdSenseClientID) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if r.Method != http.MethodHead {
		fmt.Fprintf(w, "google.com, %s, DIRECT, f08c47fec0942fa0\n", strings.TrimPrefix(s.googleAdSenseClientID, "ca-"))
	}
}

func (s *server) handlePrivacy(w http.ResponseWriter, r *http.Request) {
	if rejectNonGetHead(w, r) {
		return
	}
	s.renderTemplate(w, r, "privacy", map[string]any{
		"Title":        "Privacy",
		"Description":  "How arXiv.gg uses account information, reading activity, cookies, and analytics.",
		"CanonicalURL": arxiv.SiteBaseURL() + "/privacy",
	})
}
