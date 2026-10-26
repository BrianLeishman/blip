package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRESTNotModifiedReusesBody(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 *If-None-Match:*)
  printf 'HTTP/2.0 304 Not Modified\r\n\r\n'
  exit 1 ;;
 *)
  printf 'HTTP/2.0 200 OK\r\nEtag: W/"abc"\r\n\r\n'
  printf '%s\n' '{"login":"ada"}' ;;
esac
`)
	client := NewClient(Config{})
	var first, second struct{ Login string }
	if err := client.restJSON(t.Context(), &first, "user"); err != nil {
		t.Fatal(err)
	}
	if err := client.restJSON(t.Context(), &second, "user"); err != nil {
		t.Fatal(err)
	}
	if first.Login != "ada" || second.Login != first.Login {
		t.Fatalf("bodies: %#v %#v", first, second)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || strings.Contains(lines[0], "If-None-Match") || !strings.Contains(lines[1], "-H") || !strings.Contains(lines[1], `If-None-Match: W/"abc"`) {
		t.Fatalf("args:\n%s", b)
	}
}

func TestRESTAllFollowsConditionalNext(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 *If-None-Match:*)
  printf 'HTTP/2.0 304 Not Modified\r\nLink: <https://evil.test/nope>; rel="next"\r\n\r\n'
  exit 1 ;;
 *page=2*)
  printf 'HTTP/2.0 200 OK\r\nEtag: "p2"\r\n\r\n'
  printf '%s\n' '[{"n":2}]' ;;
 *)
  printf 'HTTP/2.0 200 OK\r\nEtag: "p1"\r\n'
  printf 'Link: <https://api.github.com/items?per_page=1&page=2>; rel="next", <https://api.github.com/items?per_page=1&page=2>; rel="last"\r\n\r\n'
  printf '%s\n' '[{"n":1}]' ;;
esac
`)
	client := NewClient(Config{})
	for run := 0; run < 2; run++ {
		items, err := restAll[struct{ N int }](t.Context(), client, "items?per_page=1")
		if err != nil {
			t.Fatal(err)
		}
		if len(items) != 2 || items[0].N != 1 || items[1].N != 2 {
			t.Fatalf("run %d: %#v", run, items)
		}
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var page2 []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if strings.Contains(line, "page=2") {
			page2 = append(page2, line)
		}
	}
	if len(page2) != 2 || strings.Contains(page2[0], "If-None-Match") || !strings.Contains(page2[1], `If-None-Match: "p2"`) {
		t.Fatalf("page 2 args:\n%s", b)
	}
}

func TestRESTRejectsForbiddenAndUncached304(t *testing.T) {
	fakeAlertsGH(t, `printf 'HTTP/2.0 403 Forbidden\r\n\r\n'
exit 1
`)
	var dst struct{}
	err := NewClient(Config{}).restJSON(t.Context(), &dst, "user")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatal(err)
	}
	fakeAlertsGH(t, `printf 'HTTP/2.0 304 Not Modified\r\n\r\n'
exit 1
`)
	if err = NewClient(Config{}).restJSON(t.Context(), &dst, "user"); err == nil {
		t.Fatal("304 without a cache entry succeeded")
	}
}

func TestRESTRejectsNextLinkOnOtherHost(t *testing.T) {
	fakeAlertsGH(t, `printf 'HTTP/2.0 200 OK\r\nEtag: "x"\r\nLink: <https://evil.test/items?page=2>; rel="next"\r\n\r\n'
printf '%s\n' '[{"n":1}]'
`)
	_, err := restAll[struct{ N int }](t.Context(), NewClient(Config{}), "items")
	if err == nil || !strings.Contains(err.Error(), "api.github.com") {
		t.Fatal(err)
	}
}

func TestRESTEvictsLeastRecentlyUsed(t *testing.T) {
	previous := restCacheEntries
	restCacheEntries = 2
	t.Cleanup(func() { restCacheEntries = previous })
	fakeAlertsGH(t, `case "$*" in
 *If-None-Match:*)
  printf 'HTTP/2.0 304 Not Modified\r\n\r\n'
  exit 1 ;;
 *)
  printf 'HTTP/2.0 200 OK\r\nEtag: "e"\r\n\r\n'
  printf '%s\n' '{"ok":true}' ;;
esac
`)
	client := NewClient(Config{})
	for _, path := range []string{"a", "b"} {
		if _, _, err := client.restGet(t.Context(), path); err != nil {
			t.Fatal(err)
		}
	}
	client.restMu.Lock()
	older := client.rest["a"]
	older.used = time.Unix(100, 0)
	client.rest["a"] = older
	newer := client.rest["b"]
	newer.used = time.Unix(200, 0)
	client.rest["b"] = newer
	client.restMu.Unlock()
	if _, _, err := client.restGet(t.Context(), "a"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.restGet(t.Context(), "c"); err != nil {
		t.Fatal(err)
	}
	client.restMu.Lock()
	defer client.restMu.Unlock()
	if _, ok := client.rest["a"]; !ok {
		t.Fatal("304 hit was evicted")
	}
	if _, ok := client.rest["b"]; ok {
		t.Fatal("least recently used entry was kept")
	}
	if _, ok := client.rest["c"]; !ok {
		t.Fatal("new entry missing")
	}
}

func TestETagSurvivesDeploymentTTL(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 "repo view "*) echo '{"isArchived":false}' ;;
 *If-None-Match:*)
  printf 'HTTP/2.0 304 Not Modified\r\n\r\n'
  exit 1 ;;
 *actions/workflows/*)
  printf 'HTTP/2.0 200 OK\r\nEtag: "dep"\r\n\r\n'
  printf '%s\n' "$RUNS" ;;
 *) exit 1 ;;
esac
`)
	d := DeploymentWorkflow{"Org/widget", "deploy.yml", "main"}
	client := NewClient(Config{Owners: []string{"Org"}, Deployments: []DeploymentWorkflow{d}})
	t.Setenv("RUNS", `{"workflow_runs":[{"id":10,"event":"push","status":"completed","conclusion":"failure","head_branch":"main","created_at":"2026-10-05T10:00:00Z"}]}`)
	if _, err := client.fetchAlerts(t.Context(), "Viewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.fetchAlerts(t.Context(), "Viewer"); err != nil {
		t.Fatal(err)
	}
	calls := readCalls(t, log)
	if strings.Count(calls, "/actions/workflows/") != 1 {
		t.Fatalf("deployment TTL still called REST:\n%s", calls)
	}
	cache := client.deployments[d]
	cache.at = time.Now().Add(-3 * time.Minute)
	client.deployments[d] = cache
	if _, err := client.fetchAlerts(t.Context(), "Viewer"); err != nil {
		t.Fatal(err)
	}
	calls = readCalls(t, log)
	var workflows []string
	for _, line := range strings.Split(strings.TrimSpace(calls), "\n") {
		if strings.Contains(line, "/actions/workflows/") {
			workflows = append(workflows, line)
		}
	}
	if len(workflows) != 2 || strings.Contains(workflows[0], "If-None-Match") || !strings.Contains(workflows[1], `If-None-Match: "dep"`) {
		t.Fatalf("workflow args:\n%s", calls)
	}
}

func readCalls(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
