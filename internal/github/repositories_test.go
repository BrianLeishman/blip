package github

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPullSearchStrings(t *testing.T) {
	log := fakeSearchGH(t)
	base := "is:pr is:open draft:false archived:false user:MyOrg user:MyUser repo:myorg/WIDGET repo:Other/explicit repo:Other/archived "
	c := Config{Owners: []string{"MyOrg", "MyUser"}, Repositories: []string{"myorg/WIDGET", "Other/explicit", "Other/archived"}, RequiredApprovals: 2}
	if err := Validate(c); err != nil {
		t.Fatal(err)
	}
	check := func(c Config, want []string) {
		t.Helper()
		t.Setenv("GQL", searchBody("BrianLeishman", emptyGroups(len(prQueries(c)))))
		if _, err := Fetch(t.Context(), c); err != nil {
			t.Fatal(err)
		}
		calls := readCalls(t, log)
		if strings.Contains(calls, "repo view") || strings.Contains(calls, "pr list") {
			t.Fatalf("search still fans out: %s", calls)
		}
		got := searchVars(lastCall(calls))
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
	check(c, []string{base + "author:@me sort:created-desc", base + "review-requested:@me sort:updated-asc"})
	c.IncludeDependabot = true
	check(c, []string{base + "author:@me sort:created-desc", base + "author:app/dependabot sort:created-desc", base + "review-requested:@me sort:updated-asc"})
	if strings.Contains(lastCall(readCalls(t, log)), "dependabot[bot]") {
		t.Fatal("search string used dependabot[bot] instead of app/dependabot")
	}
	c.AdditionalAuthors = []string{"ExtraUser", "extrauser"}
	check(c, []string{base + "author:@me sort:created-desc", base + "author:app/dependabot sort:created-desc", base + "author:ExtraUser sort:created-desc", base + "review-requested:@me sort:updated-asc"})
	if !reflect.DeepEqual(prQueries(c), []prQuery{{"ready", "@me"}, {"ready", "dependabot[bot]"}, {"ready", "ExtraUser"}, {"review", ""}}) {
		t.Fatal("extra author searches must be deduplicated and precede reviews")
	}
	if len(strings.Split(strings.TrimSpace(readCalls(t, log)), "\n")) != 3 {
		t.Fatal("expected one gh call per refresh")
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

func TestReviewQueuePrioritizesApprovalsAcrossRepositories(t *testing.T) {
	log := fakeSearchGH(t)
	approved := func(user string) Review {
		r := Review{State: "APPROVED"}
		r.User.Login = user
		return r
	}
	one := []Review{approved("Reviewer1")}
	q1 := []fakePR{
		{repo: "Org/a", number: 1, title: "Old unreviewed PR", updated: "2026-01-01T00:00:00Z"},
		{repo: "Org/a", number: 2, title: "Recently approved PR", updated: "2026-10-22T00:00:00Z", reviews: one},
		{repo: "Org/a", number: 3, title: "Earlier approved PR", updated: "2026-10-21T00:00:00Z", reviews: one},
		{repo: "Org/b", number: 4, title: "Oldest approved PR", updated: "2026-10-20T00:00:00Z", reviews: one},
		{repo: "Org/b", number: 5, title: "Other unreviewed PR", updated: "2026-01-02T00:00:00Z"},
		{repo: "Org/b", number: 6, title: "Two approvals", updated: "2026-10-23T00:00:00Z", reviews: []Review{approved("Reviewer1"), approved("Reviewer2")}},
		{repo: "Org/b", number: 8, title: "Tied approved PR", updated: "2026-10-21T00:00:00Z", reviews: one},
	}
	t.Setenv("GQL", searchBody("Viewer", [][]fakePR{{{repo: "Org/a", number: 7, title: "Own PR"}}, q1}))
	for _, required := range []int{2, 3} {
		result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/a", "Org/b"}, RequiredApprovals: required})
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Org/a/pull/7"}
		if required == 3 {
			want = append(want, "Org/b/pull/6")
		}
		want = append(want, "Org/b/pull/4", "Org/a/pull/3", "Org/b/pull/8", "Org/a/pull/2", "Org/a/pull/1", "Org/b/pull/5")
		var got []string
		for _, row := range result.Snapshot.Rows {
			got = append(got, row.ID)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("required=%d: got %v, want %v", required, got, want)
		}
	}
	if strings.Contains(readCalls(t, log), "reviews?per_page") || strings.Count(readCalls(t, log), "api graphql") != 2 {
		t.Fatal(readCalls(t, log))
	}
}

func TestFetchMultipleReposKeepsOpenTargetsAndDeduplicatesSharedAuthorReviews(t *testing.T) {
	fakeSearchGH(t)
	both := func(number int, title, authorType, author string, draft bool, requests, assignees []string) []fakePR {
		var out []fakePR
		for _, repo := range []string{"Org/a", "Org/b"} {
			out = append(out, fakePR{repo: repo, number: number, title: title, authorType: authorType, author: author, draft: draft, requests: requests, assignees: assignees})
		}
		return out
	}
	// PR 4 is authored by the raw GraphQL bot shape and assigned elsewhere.
	// Appending [bot] is what makes that shared author hidden.
	groups := [][]fakePR{
		both(1, "Own PR", "", "", false, nil, nil),
		append(both(2, "Bot PR", "Bot", "dependabot", false, nil, nil), both(4, "Assigned bot PR", "Bot", "dependabot", false, nil, []string{"OtherReviewer"})...),
		append(both(5, "Additional author PR", "User", "ExtraUser", false, nil, nil),
			append(both(6, "Awaiting review", "User", "ExtraUser", false, []string{"Reviewer"}, nil),
				append(both(7, "Assigned elsewhere", "User", "ExtraUser", false, nil, []string{"OtherReviewer"}),
					append(both(8, "Assigned to viewer", "User", "ExtraUser", false, nil, []string{"BrianLeishman"}),
						both(9, "Draft", "User", "ExtraUser", true, nil, nil)...)...)...)...),
		append(both(2, "Bot PR", "Bot", "dependabot", false, nil, nil),
			append(both(3, "Review PR", "", "", false, nil, nil),
				append(both(5, "Additional author PR", "User", "ExtraUser", false, nil, nil),
					append(both(6, "Awaiting review", "User", "ExtraUser", false, []string{"Reviewer"}, nil),
						append(both(7, "Assigned elsewhere", "User", "ExtraUser", false, nil, []string{"OtherReviewer"}),
							both(4, "Assigned bot PR", "Bot", "dependabot", false, nil, []string{"OtherReviewer"})...)...)...)...)...),
	}
	t.Setenv("GQL", searchBody("BrianLeishman", groups))
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

func TestReviewQueueHidesUnreadyCIWithoutHidingOwnedPRs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		checks  []Check
		visible bool
	}{
		{"no checks", nil, true},
		{"success", []Check{{Status: "COMPLETED", Conclusion: "SUCCESS"}}, true},
		{"skipped and neutral", []Check{{Status: "COMPLETED", Conclusion: "SKIPPED"}, {Status: "COMPLETED", Conclusion: "NEUTRAL"}}, true},
		{"legacy success", []Check{{State: "SUCCESS"}}, true},
		{"queued", []Check{{Status: "QUEUED"}}, false},
		{"running", []Check{{Status: "IN_PROGRESS"}}, false},
		{"waiting", []Check{{Status: "WAITING"}}, false},
		{"legacy pending", []Check{{State: "PENDING"}}, false},
		{"failure", []Check{{Status: "COMPLETED", Conclusion: "FAILURE"}}, false},
		{"cancelled", []Check{{Status: "COMPLETED", Conclusion: "CANCELLED"}}, false},
		{"timed out", []Check{{Status: "COMPLETED", Conclusion: "TIMED_OUT"}}, false},
		{"action required", []Check{{Status: "COMPLETED", Conclusion: "ACTION_REQUIRED"}}, false},
		{"legacy error", []Check{{State: "ERROR"}}, false},
		{"success and running", []Check{{State: "SUCCESS"}, {Status: "IN_PROGRESS"}}, false},
		{"failure and running", []Check{{Conclusion: "FAILURE", Status: "COMPLETED"}, {Status: "IN_PROGRESS"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeSearchGH(t)
			groups := make([][]fakePR, 4)
			for i := range groups {
				groups[i] = []fakePR{{repo: "Org/example", number: i + 1, title: "Example PR", checks: tc.checks}}
			}
			t.Setenv("GQL", searchBody("Viewer", groups))
			result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, IncludeDependabot: true, AdditionalAuthors: []string{"ExtraUser"}, RequiredApprovals: 2})
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"Org/example/pull/1", "Org/example/pull/2", "Org/example/pull/3"}
			if tc.visible {
				want = append(want, "Org/example/pull/4")
			}
			var got []string
			for _, row := range result.Snapshot.Rows {
				got = append(got, row.ID)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			if (result.URLs["Org/example/pull/4"] != "") != tc.visible {
				t.Fatal("review URL visibility differs from row visibility")
			}
		})
	}
}

func TestConflictsHiddenFromReviewButRetainedForOwnedAuthors(t *testing.T) {
	fakeSearchGH(t)
	pr := func(number int, merge string) fakePR {
		return fakePR{repo: "Org/widget", number: number, merge: merge}
	}
	t.Setenv("GQL", searchBody("Viewer", [][]fakePR{
		{pr(1, "DIRTY")},
		{pr(2, "DIRTY")},
		{pr(3, "DIRTY")},
		{pr(4, "DIRTY"), pr(5, "BLOCKED"), pr(6, "BEHIND"), pr(7, "UNKNOWN")},
	}))
	result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/widget"}, IncludeDependabot: true, AdditionalAuthors: []string{"ExtraUser"}, RequiredApprovals: 2})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Org/widget/pull/1", "Org/widget/pull/2", "Org/widget/pull/3", "Org/widget/pull/5", "Org/widget/pull/6", "Org/widget/pull/7"}
	var got []string
	for i, row := range result.Snapshot.Rows {
		got = append(got, row.ID)
		if i < 3 && (row.Section != "mine" || row.Badge != "CONFLICT") {
			t.Fatalf("owned conflict hidden: %#v", row)
		}
		if i >= 3 && row.Section != "review" {
			t.Fatalf("non-conflicted review misclassified: %#v", row)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if result.URLs["Org/widget/pull/4"] != "" {
		t.Fatal("conflicted review retained")
	}
}

func TestPullSearchLimitsAndOldReviews(t *testing.T) {
	t.Run("result limit", func(t *testing.T) {
		fakeSearchGH(t)
		t.Setenv("GQL", `{"data":{"viewer":{"login":"Viewer"},"q0":{"issueCount":1001,"pageInfo":{"hasNextPage":false},"nodes":[]},"q1":{"issueCount":0,"pageInfo":{"hasNextPage":false},"nodes":[]}}}`)
		_, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err == nil || !strings.Contains(err.Error(), "reached GitHub's 1000-result limit; narrow the owners") {
			t.Fatal(err)
		}
	})
	t.Run("empty viewer", func(t *testing.T) {
		fakeSearchGH(t)
		t.Setenv("GQL", searchBody("", emptyGroups(2)))
		_, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err == nil || err.Error() != "GitHub returned an empty viewer login" {
			t.Fatal(err)
		}
	})
	t.Run("too many checks", func(t *testing.T) {
		fakeSearchGH(t)
		t.Setenv("GQL", searchBody("Viewer", [][]fakePR{{{repo: "Org/example", number: 4, moreChecks: true}}, {}}))
		_, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err == nil || err.Error() != "pull request 4 exceeds 100 checks; cannot verify CI" {
			t.Fatal(err)
		}
	})
	t.Run("invalid repository", func(t *testing.T) {
		fakeSearchGH(t)
		t.Setenv("GQL", searchBody("Viewer", [][]fakePR{{{repo: "not a repo", number: 1}}, {}}))
		_, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err == nil || !strings.Contains(err.Error(), `invalid repository from search: "not a repo"`) {
			t.Fatal(err)
		}
	})
	t.Run("pagination", func(t *testing.T) {
		log := fakeSearchGH(t)
		var body map[string]any
		if err := json.Unmarshal([]byte(searchBody("Viewer", [][]fakePR{{{repo: "Org/example", number: 1, title: "First"}}, {}})), &body); err != nil {
			t.Fatal(err)
		}
		q0 := body["data"].(map[string]any)["q0"].(map[string]any)
		q0["issueCount"] = 2
		q0["pageInfo"] = map[string]any{"hasNextPage": true, "endCursor": "CUR"}
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("GQL", string(raw))
		second := fakePR{repo: "Org/example", number: 2, title: "Second"}.node()
		t.Setenv("GQL_AFTER", `{"data":{"q0":{"issueCount":2,"pageInfo":{"hasNextPage":false,"endCursor":"NEXT"},"nodes":[`+string(second)+`]}}}`)
		result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, row := range result.Snapshot.Rows {
			got = append(got, row.ID)
		}
		if !reflect.DeepEqual(got, []string{"Org/example/pull/1", "Org/example/pull/2"}) {
			t.Fatal(got)
		}
		calls := readCalls(t, log)
		if strings.Count(calls, "api graphql") != 2 || !strings.Contains(calls, "-f after=CUR") {
			t.Fatal(calls)
		}
	})
	t.Run("reviews older than the graphql page", func(t *testing.T) {
		log := filepath.Join(t.TempDir(), "calls")
		t.Setenv("CALLS", log)
		fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 api\ graphql\ *) printf '%s\n' "$GQL" ;;
 *pulls/7/reviews?per_page=100*) ok "$REVIEWS" ;;
 *) exit 1 ;;
esac
`)
		pr := fakePR{repo: "Org/example", number: 7, title: "Reviewed", author: "Viewer", requests: []string{"someone"}, moreReviews: true}
		pr.reviews = []Review{{State: "COMMENTED", SubmittedAt: "2026-10-01T00:00:00Z"}}
		pr.reviews[0].User.Login = "Someone"
		t.Setenv("GQL", searchBody("Viewer", [][]fakePR{{pr}, {}}))
		t.Setenv("REVIEWS", `[{"user":{"login":"Viewer"},"state":"APPROVED","submitted_at":"2020-01-01T00:00:00Z"},{"user":{"login":"Reviewer"},"state":"APPROVED","submitted_at":"2020-01-02T00:00:00Z"},{"user":{"login":"Reviewer"},"state":"COMMENTED","submitted_at":"2026-10-01T00:00:00Z"}]`)
		result, err := Fetch(t.Context(), Config{Repositories: []string{"Org/example"}, RequiredApprovals: 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Snapshot.Rows) != 1 || result.Snapshot.Rows[0].Section != "mine" || !strings.HasPrefix(result.Snapshot.Rows[0].Badge, "1/2") {
			t.Fatalf("rows %#v", result.Snapshot.Rows)
		}
		if strings.Count(readCalls(t, log), "pulls/7/reviews?per_page=100") != 1 {
			t.Fatal(readCalls(t, log))
		}
	})
}

func fakeSearchGH(t *testing.T) string {
	t.Helper()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 *"-f after="*) printf '%s\n' "$GQL_AFTER" ;;
 api\ graphql\ *) printf '%s\n' "$GQL" ;;
 *) exit 1 ;;
esac
`)
	return log
}

func lastCall(calls string) string {
	lines := strings.Split(strings.TrimSpace(calls), "\n")
	return lines[len(lines)-1]
}

func searchVars(line string) []string {
	var got []string
	for _, part := range strings.Split(line, " -f ") {
		name, value, ok := strings.Cut(part, "=")
		if !ok || len(name) < 2 || name[0] != 'q' || name[1] < '0' || name[1] > '9' {
			continue
		}
		got = append(got, value)
	}
	return got
}

func emptyGroups(n int) [][]fakePR {
	return make([][]fakePR, n)
}

type fakePR struct {
	repo, title, merge, updated, author, authorType string
	number                                          int
	draft, moreReviews, moreChecks                  bool
	requests, assignees                             []string
	checks                                          []Check
	reviews                                         []Review
}

func (p fakePR) node() json.RawMessage {
	authorType := p.authorType
	if authorType == "" {
		authorType = "User"
	}
	requests := []any{}
	for _, login := range p.requests {
		requests = append(requests, map[string]any{"requestedReviewer": map[string]any{"__typename": "User", "login": login}})
	}
	assignees := []any{}
	for _, login := range p.assignees {
		assignees = append(assignees, map[string]any{"login": login})
	}
	checkNodes := []any{}
	for _, c := range p.checks {
		if c.Status == "" && c.Conclusion == "" && c.Name == "" {
			checkNodes = append(checkNodes, map[string]any{"__typename": "StatusContext", "context": c.Context, "state": c.State, "createdAt": c.CreatedAt})
			continue
		}
		checkNodes = append(checkNodes, map[string]any{"__typename": "CheckRun", "name": c.Name, "status": c.Status, "conclusion": c.Conclusion, "startedAt": c.StartedAt, "completedAt": c.CompletedAt})
	}
	reviewNodes := []any{}
	for _, r := range p.reviews {
		kind := "User"
		if r.User.Type == "Bot" {
			kind = "Bot"
		}
		reviewNodes = append(reviewNodes, map[string]any{"state": r.State, "submittedAt": r.SubmittedAt, "author": map[string]any{"__typename": kind, "login": r.User.Login}})
	}
	node := map[string]any{
		"number": p.number, "title": p.title, "isDraft": p.draft, "mergeStateStatus": p.merge, "updatedAt": p.updated,
		"repository":     map[string]any{"nameWithOwner": p.repo},
		"author":         map[string]any{"__typename": authorType, "login": p.author},
		"assignees":      map[string]any{"nodes": assignees},
		"reviewRequests": map[string]any{"nodes": requests},
		"labels":         map[string]any{"nodes": []any{}},
		"reviews":        map[string]any{"pageInfo": map[string]any{"hasPreviousPage": p.moreReviews}, "nodes": reviewNodes},
		"commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"statusCheckRollup": map[string]any{"contexts": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": p.moreChecks},
			"nodes":    checkNodes,
		}}}}}},
	}
	b, err := json.Marshal(node)
	if err != nil {
		panic(err)
	}
	return b
}

func searchBody(viewer string, groups [][]fakePR) string {
	data := map[string]any{"viewer": map[string]any{"login": viewer}}
	for i, group := range groups {
		nodes := make([]json.RawMessage, 0, len(group))
		for _, pr := range group {
			nodes = append(nodes, pr.node())
		}
		data[fmt.Sprintf("q%d", i)] = map[string]any{
			"issueCount": len(nodes),
			"pageInfo":   map[string]any{"hasNextPage": false, "endCursor": ""},
			"nodes":      nodes,
		}
	}
	b, err := json.Marshal(map[string]any{"data": data})
	if err != nil {
		panic(err)
	}
	return string(b)
}
