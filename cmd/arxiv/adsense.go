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
