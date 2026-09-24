package github

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverOwnerRepositories(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
 "repo view Other/archived --json isArchived") echo '{"isArchived":true}'; exit 0 ;;
 "api user") echo '{"login":"BrianLeishman"}' ;;
 "repo view "*) echo '{"isArchived":false}'; exit 0 ;;
esac
case "$*" in
 *"--archived=false"*) ;;
 *) exit 1 ;;
esac
case "$*" in
 *"--owner MyOrg --owner MyUser"*) ;;
 *) exit 1 ;;
esac
case "$*" in
 *"--author dependabot[bot]"*) echo '[{"repository":{"nameWithOwner":"MyOrg/bot-only"}}]'; exit 0 ;;
 *--author*) echo '[{"repository":{"nameWithOwner":"MyOrg/widget"}},{"repository":{"nameWithOwner":"MyUser/personal"}}]' ;;
 *--review-requested*) echo '[{"repository":{"nameWithOwner":"MyOrg/widget"}},{"repository":{"nameWithOwner":"MyOrg/review-only"}}]' ;;
 *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	c := Config{Owners: []string{"MyOrg", "MyUser"}, Repositories: []string{"myorg/WIDGET", "Other/explicit", "Other/archived"}, RequiredApprovals: 2}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	got, err := discoverRepositories(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"MyOrg/review-only", "MyOrg/widget", "MyUser/personal", "Other/explicit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	c.IncludeDependabot = true
	got, err = discoverRepositories(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	want = append([]string{"MyOrg/bot-only"}, want...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bot-only repository missing: %v", got)
	}
	if !reflect.DeepEqual(prSections(c), []string{"ready", "dependabot", "review"}) {
		t.Fatal("bot PRs must precede reviews for deduplication")
	}

}

func TestOwnerConfiguration(t *testing.T) {
	for _, owner := range []string{"", "a/b", "a b", ".."} {
		if err := Validate(Config{Owners: []string{owner}, RequiredApprovals: 2}); err == nil {
			t.Fatalf("accepted %q", owner)
		}
	}
	if err := Validate(Config{Owners: []string{"BrianLeishman"}, RequiredApprovals: 2}); err != nil {
		t.Fatal(err)
	}
	if err := Validate(Config{RequiredApprovals: 2}); err == nil {
		t.Fatal("accepted empty scope")
	}
}

func TestFetchMultipleReposKeepsOpenTargetsAndDeduplicatesBotReviews(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
 "api user") echo '{"login":"BrianLeishman"}' ;;
 "repo view "*) echo '{"isArchived":false}' ;;
 "pr list "*"--author @me") echo '[{"number":1,"title":"Own PR"}]' ;;
 "pr list "*"--author dependabot[bot]") echo '[{"number":2,"title":"Bot PR"}]' ;;
 "pr list "*) echo '[{"number":2,"title":"Bot PR"},{"number":3,"title":"Review PR"},{"number":4,"title":"Assigned bot PR","author":{"login":"app/dependabot"},"assignees":[{"login":"OtherReviewer"}]}]' ;;
 "api "*) echo '[[]]' ;;
 *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/b", "Org/a"}, IncludeDependabot: true, RequiredApprovals: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Org/a/pull/1", "Org/a/pull/2", "Org/b/pull/1", "Org/b/pull/2", "Org/a/pull/3", "Org/b/pull/3"}
	var got []string
	for _, row := range result.Snapshot.Rows {
		got = append(got, row.ID)
		if result.URLs[row.ID] != "https://github.com/"+row.ID {
			t.Fatalf("missing open target: %s", row.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
