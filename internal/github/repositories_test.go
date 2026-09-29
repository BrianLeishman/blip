package github

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
 *"--author ExtraUser"*) echo '[{"repository":{"nameWithOwner":"MyOrg/extra-only"}}]'; exit 0 ;;
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
	c.AdditionalAuthors = []string{"ExtraUser", "extrauser"}
	got, err = discoverRepositories(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"MyOrg/bot-only", "MyOrg/extra-only", "MyOrg/review-only", "MyOrg/widget", "MyUser/personal", "Other/explicit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("additional-author-only repository missing: %v", got)
	}
	if !reflect.DeepEqual(prQueries(c), []prQuery{{"ready", "@me"}, {"ready", "dependabot[bot]"}, {"ready", "ExtraUser"}, {"review", ""}}) {
		t.Fatal("extra author searches must be deduplicated and precede reviews")
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

func TestFetchMultipleReposKeepsOpenTargetsAndDeduplicatesSharedAuthorReviews(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$*" in
 "api user") echo '{"login":"BrianLeishman"}' ;;
 "repo view "*) echo '{"isArchived":false}' ;;
 "pr list "*"--author @me") echo '[{"number":1,"title":"Own PR"}]' ;;
 "pr list "*"--author dependabot[bot]") echo '[{"number":2,"title":"Bot PR"}]' ;;
 "pr list "*"--author ExtraUser") echo '[{"number":5,"title":"Additional author PR","author":{"login":"ExtraUser"}},{"number":6,"title":"Awaiting review","author":{"login":"ExtraUser"},"reviewRequests":[{"login":"Reviewer"}]},{"number":7,"title":"Assigned elsewhere","author":{"login":"ExtraUser"},"assignees":[{"login":"OtherReviewer"}]},{"number":8,"title":"Assigned to viewer","author":{"login":"ExtraUser"},"assignees":[{"login":"BrianLeishman"}]},{"number":9,"title":"Draft","author":{"login":"ExtraUser"},"isDraft":true}]' ;;
 "pr list "*) echo '[{"number":2,"title":"Bot PR"},{"number":3,"title":"Review PR"},{"number":5,"title":"Additional author PR","author":{"login":"ExtraUser"}},{"number":6,"title":"Awaiting review","author":{"login":"ExtraUser"},"reviewRequests":[{"login":"Reviewer"}]},{"number":7,"title":"Assigned elsewhere","author":{"login":"ExtraUser"},"assignees":[{"login":"OtherReviewer"}]},{"number":4,"title":"Assigned bot PR","author":{"login":"app/dependabot"},"assignees":[{"login":"OtherReviewer"}]}]' ;;
 "api "*) echo '[[]]' ;;
 *) exit 1 ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/b", "Org/a"}, IncludeDependabot: true, AdditionalAuthors: []string{"ExtraUser"}, RequiredApprovals: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Org/a/pull/1", "Org/a/pull/2", "Org/a/pull/5", "Org/a/pull/8", "Org/b/pull/1", "Org/b/pull/2", "Org/b/pull/5", "Org/b/pull/8", "Org/a/pull/6", "Org/b/pull/6", "Org/a/pull/3", "Org/b/pull/3"}
	var got []string
	for _, row := range result.Snapshot.Rows {
		got = append(got, row.ID)
		wantSection := "ready"
		if strings.HasSuffix(row.ID, "/6") {
			wantSection = "mine"
		} else if strings.HasSuffix(row.ID, "/3") {
			wantSection = "review"
		}
		if row.Section != wantSection {
			t.Fatalf("%s: section %q, want %q", row.ID, row.Section, wantSection)
		}
		if result.URLs[row.ID] != "https://github.com/"+row.ID {
			t.Fatalf("missing open target: %s", row.ID)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAdditionalAuthorConfiguration(t *testing.T) {
	for _, author := range []string{"", "@me", "a/b", "a b", "..", "--search", "x\nuser"} {
		c := Config{Owners: []string{"ExampleOrg"}, AdditionalAuthors: []string{author}, RequiredApprovals: 2}
		if err := Validate(c); err == nil {
			t.Fatalf("accepted author %q", author)
		}
	}
	c := Config{Owners: []string{"ExampleOrg"}, AdditionalAuthors: []string{"ExtraUser"}, RequiredApprovals: 2}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prQueries(c), []prQuery{{"ready", "@me"}, {"ready", "ExtraUser"}, {"review", ""}}) {
		t.Fatal("extra authors must work with Dependabot disabled")
	}
}
