package main

import (
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lantos1618/arxiv.gg"
)

// TestUIBrowserFixtureServer serves actual templates with public fixture data for
// tools/test_graph_ui.cjs and search browser regressions. It never opens a database.
// Opt in with ARXIV_UI_TEST_ADDR=127.0.0.1:18081 and run this test alone; stop the
// process after the browser checks. The listener must be bound to loopback.
func TestUIBrowserFixtureServer(t *testing.T) {
	addr := os.Getenv("ARXIV_UI_TEST_ADDR")
	if addr == "" {
		t.Skip("set ARXIV_UI_TEST_ADDR to run the isolated browser fixture server")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		t.Fatal("ARXIV_UI_TEST_ADDR must use a loopback IP and port")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	paper := arxiv.Paper{ID: "1706.03762", Title: "Attention Is All You Need", Authors: "Ashish Vaswani, Niki Parmar", Categories: "cs.CL cs.LG", Abstract: "Public paper fixture for isolated UI regression checks.", Created: time.Date(2017, 6, 12, 0, 0, 0, 0, time.UTC)}
	s := &server{qwenAsyncWorkerEnabled: true}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "fixture is read-only", http.StatusMethodNotAllowed)
			return
		}
		data := map[string]any{"CurrentUser": nil, "Stats": &arxiv.CacheStats{TotalPapers: 1, QwenEmbeddingsCount: 1}, "Papers": []arxiv.Paper{paper}, "GoogleEnabled": true}
		name := ""
		switch {
		case r.URL.Path == "/ad-presentation":
			serveAdPresentationFixture(w, r)
			return
		case r.URL.Path == "/":
			name = "index"
		case r.URL.Path == "/account":
			name = "account"
			user := &arxiv.User{ID: "fixture", Name: "Alex Reader", Email: "alex@example.test", AuthProvider: "google", Plan: "free"}
			data["CurrentUser"], data["User"] = user, user
			data["PeerViewsSource"] = "community"
			state := r.URL.Query().Get("state")
			data["APIKeys"] = []accountAPIKey{}
			if state != "empty" {
				data["APIKeys"] = []accountAPIKey{
					{ID: "key_fixture_one", Name: "Claude Desktop", Masked: "arxivgg_example...demo", CreatedAt: time.Date(2026, 7, 9, 13, 46, 0, 0, time.UTC)},
					{ID: "key_fixture_two", Name: "Research script", Masked: "arxivgg_script...demo", CreatedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)},
				}
			}
			if state == "new" {
				data["APIKey"] = "arxivgg_fixture_only_not_a_real_key"
				data["APIKeyNotice"] = "API key created. Copy it now; it is only shown on this screen."
			}
			if state == "error" {
				data["APIKeyNotice"] = "We couldn’t update your API key. Your existing key has not been changed. Please try again."
				data["APIKeyWarning"] = true
			}
		case r.URL.Path == "/admin" || r.URL.Path == "/admin/users" || r.URL.Path == "/admin/audit" || r.URL.Path == "/admin/feedback":
			names := map[string]string{"/admin": "admin", "/admin/users": "admin_users", "/admin/audit": "admin_audit", "/admin/feedback": "admin_feedback"}
			titles := map[string]string{"/admin": "Admin", "/admin/users": "Admin Users", "/admin/audit": "Admin Audit", "/admin/feedback": "Admin Feedback"}
			name = names[r.URL.Path]
			data["Title"] = titles[r.URL.Path]
			data["CurrentUserIsAdmin"] = true
			data["Stats"] = &arxiv.AdminStats{GeneratedAt: time.Now(), DBType: arxiv.DBTypePostgres, Cache: arxiv.CacheStats{TotalPapers: 3120000}, Embeddings: arxiv.AdminEmbeddingStats{FullAbstracts: 3120000, QwenAbstracts: 2964000}, Users: arxiv.AdminUserStats{TotalUsers: 1280, Active24h: 146}, RecentUsers: []arxiv.AdminUserRow{{Email: "alex@example.test", Name: "Alex Reader", Plan: "free", Provider: "google"}}}
			data["Plans"] = simplePlanModel()
		case r.URL.Path == "/login":
			name = "login"
			data["Next"] = "/account"
			data["GoogleEnabled"] = r.URL.Query().Get("state") != "unavailable"
			if r.URL.Query().Get("state") == "error" {
				data["Error"] = "Google sign-in could not start. Try again in a moment."
			}
		case r.URL.Path == "/api/v1/":
			name = "api"
			data["BaseURL"], data["MCPURL"] = "https://arxiv.gg/api/v1", "https://arxiv.gg/mcp"
		case r.URL.Path == "/search":
			if r.URL.Query().Get("autostart") == "1" {
				s.handleSearch(w, r)
				return
			}
			name = "search"
			data["Query"] = r.URL.Query().Get("q")
			mode := r.URL.Query().Get("mode")
			if mode == "" {
				mode = "quick"
			}
			data["SearchMode"] = mode
			data["IsSemantic"] = mode == "semantic"
			data["IsDeep"] = mode == "deep"
		case r.URL.Path == "/abs/"+paper.ID:
			name = "paper"
			data["Paper"] = &paper
			data["HasEmbedding"] = true
		case strings.HasPrefix(r.URL.Path, "/author/"):
			name = "author"
			data["Author"] = strings.TrimPrefix(r.URL.Path, "/author/")
			data["PaperCount"] = 1
		default:
			http.NotFound(w, r)
			return
		}
		s.renderTemplate(w, r, name, data)
	})
	t.Logf("UI fixture server ready at http://%s (no database)", listener.Addr())
	if err := http.Serve(listener, handler); err != nil {
		t.Fatal(err)
	}
}
