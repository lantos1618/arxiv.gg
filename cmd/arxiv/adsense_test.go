package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdSenseOwnershipVerification(t *testing.T) {
	const client = "ca-pub-9693175657690620"
	s := &server{googleAdSenseClientID: client}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
		w := httptest.NewRecorder()
		s.handleAdsTXT(w, httptest.NewRequest(method, "/ads.txt", nil))
		if method == http.MethodPost {
			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("POST status = %d", w.Code)
			}
			continue
		}
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Fatalf("%s: status=%d headers=%v", method, w.Code, w.Header())
		}
		want := "google.com, pub-9693175657690620, DIRECT, f08c47fec0942fa0\n"
		if method == http.MethodHead {
			want = ""
		}
		if w.Body.String() != want {
			t.Fatalf("%s: body=%q", method, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.renderTemplate(w, httptest.NewRequest(http.MethodGet, "/", nil), "head", map[string]any{"Title": "Test"})
	if !strings.Contains(w.Body.String(), `name="google-adsense-account" content="`+client+`"`) {
		t.Fatal("missing ownership meta tag")
	}
	if strings.Contains(w.Body.String(), "adsbygoogle") {
		t.Fatal("verification must not load ads before consent setup")
	}
}

func TestAdSenseDisabledAndInvalidIDs(t *testing.T) {
	for _, id := range []string{"", "pub-9693175657690620", "ca-pub-123", "ca-pub-9693175657690620\ninjected"} {
		t.Setenv("GOOGLE_ADSENSE_CLIENT_ID", id)
		if configuredAdSenseClientID() != "" {
			t.Fatalf("accepted invalid ID %q", id)
		}
		w := httptest.NewRecorder()
		(&server{googleAdSenseClientID: id}).handleAdsTXT(w, httptest.NewRequest(http.MethodGet, "/ads.txt", nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("invalid ID %q: status=%d", id, w.Code)
		}
	}
}
