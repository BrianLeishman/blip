package github

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApprovals(t *testing.T) {
	review := func(user, state, at string) Review {
		r := Review{State: state, SubmittedAt: at}
		r.User.Login = user
		return r
	}
	tests := []struct {
		name    string
		reviews []Review
		want    int
	}{
		{"duplicate approvals", []Review{review("a", "APPROVED", "1"), review("a", "APPROVED", "2")}, 1},
		{"comment retains approval", []Review{review("a", "APPROVED", "1"), review("a", "COMMENTED", "2")}, 1},
		{"changes revoke approval", []Review{review("a", "APPROVED", "1"), review("a", "CHANGES_REQUESTED", "2")}, 0},
		{"dismissed", []Review{review("a", "DISMISSED", "1")}, 0},
		{"two people unordered", []Review{review("a", "APPROVED", "3"), review("b", "APPROVED", "2"), review("a", "CHANGES_REQUESTED", "1")}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Approvals(tt.reviews); got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}
func TestSafeURL(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "https://github.com.evil.test/x", "https://u@github.com/x", "http://github.com/x"} {
		if SafeURL(u) {
			t.Fatal(u)
		}
	}
	if !SafeURL("https://github.com/a/b/pull/1") {
		t.Fatal("valid URL rejected")
	}
}

func TestUrgentFieldSources(t *testing.T) {
	n := urgentNode{}
	n.State = "OPEN"
	n.IssueFieldValues.Nodes = append(n.IssueFieldValues.Nodes, struct {
		Name  string
		Field struct{ Name string }
	}{Name: "Urgent", Field: struct{ Name string }{Name: "Priority"}})
	if !n.urgent() {
		t.Fatal("native issue priority missing")
	}
	n.Repository.IsArchived = true
	if n.urgent() {
		t.Fatal("archived repository issue included")
	}
	n.Repository.IsArchived = false
	n.State = "CLOSED"
	if n.urgent() {
		t.Fatal("closed issue included")
	}
	n.State = "OPEN"
	n.IssueFieldValues.Nodes[0].Name = "High"
	if n.urgent() {
		t.Fatal("nonurgent included")
	}
	n.IssueFieldValues.Nodes[0].Name = "Urgent"
	n.IssueFieldValues.Nodes[0].Field.Name = "Other"
	if n.urgent() {
		t.Fatal("wrong field included")
	}
}

func TestOwnStatus(t *testing.T) {
	item := Item{ReviewDecision: "REVIEW_REQUIRED"}
	s, _, _ := OwnStatus(item, 0, 2)
	if s != "ready" {
		t.Fatal("no requests should await my merge")
	}
	item.ReviewRequests = []json.RawMessage{json.RawMessage(`{}`)}
	s, _, _ = OwnStatus(item, 0, 2)
	if s != "mine" {
		t.Fatal("pending review should stay in mine")
	}
	s, _, _ = OwnStatus(item, 2, 2)
	if s != "ready" {
		t.Fatal("two approvals should be ready")
	}
	item.ReviewDecision = "CHANGES_REQUESTED"
	item.ReviewRequests = nil
	s, _, _ = OwnStatus(item, 0, 2)
	if s != "mine" {
		t.Fatal("changes requested is not ready")
	}
}
func TestChecks(t *testing.T) {
	if Checks([]Check{{Status: "QUEUED"}, {Conclusion: "FAILURE", Status: "COMPLETED"}}) != "!" {
		t.Fatal("failure must take precedence")
	}
	if Checks([]Check{{Status: "QUEUED"}}) != "~" {
		t.Fatal("pending")
	}
	if Checks([]Check{{Status: "COMPLETED", Conclusion: "SKIPPED"}, {State: "SUCCESS"}}) != "+" {
		t.Fatal("success")
	}
}

func TestChecksRunning(t *testing.T) {
	for _, tc := range []struct {
		name   string
		checks []Check
		want   bool
	}{
		{"no checks", nil, false},
		{"queued", []Check{{Status: "QUEUED"}}, true},
		{"running", []Check{{Status: "IN_PROGRESS"}}, true},
		{"legacy pending", []Check{{State: "PENDING"}}, true},
		{"failed with another running", []Check{{Status: "COMPLETED", Conclusion: "FAILURE"}, {Status: "IN_PROGRESS"}}, true},
		{"finished", []Check{{Status: "COMPLETED", Conclusion: "FAILURE"}, {Status: "COMPLETED", Conclusion: "SUCCESS"}, {State: "SUCCESS"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ChecksRunning(tc.checks); got != tc.want {
				t.Fatalf("ChecksRunning = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestFailedChecksBlockEveryMergeReadyPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		item      Item
		approvals int
	}{
		{"no review requests", Item{ReviewDecision: "REVIEW_REQUIRED"}, 0},
		{"enough approvals", Item{ReviewRequests: []json.RawMessage{json.RawMessage(`{}`)}}, 2},
		{"GitHub approved", Item{ReviewDecision: "APPROVED", ReviewRequests: []json.RawMessage{json.RawMessage(`{}`)}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, check := range []Check{
				{Status: "COMPLETED", Conclusion: "FAILURE"},
				{State: "ERROR"},
			} {
				tc.item.StatusCheckRollup = []Check{{Status: "IN_PROGRESS"}, check}
				section, badge, detail := OwnStatus(tc.item, tc.approvals, 2)
				if section != "mine" || !strings.Contains(badge, "FAIL") || !strings.Contains(detail, "CI:FAIL") {
					t.Fatalf("failure not surfaced: %q %q %q", section, badge, detail)
				}
			}
			tc.item.StatusCheckRollup = []Check{{Status: "COMPLETED", Conclusion: "SUCCESS"}}
			section, badge, _ := OwnStatus(tc.item, tc.approvals, 2)
			if section != "ready" || strings.Contains(badge, "FAIL") {
				t.Fatalf("successful rerun did not restore ready: %q %q", section, badge)
			}
		})
	}
}

func TestUrgentExcludedStatus(t *testing.T) {
	for _, status := range []string{"Blocked", "Backlog: Blocked", "UI/UX: Blocked 🚩", "Dev: Blocked 🚩", "BI: Blocked 🚩", "blocked", "Dev: QA 🎨", "dev:QA", "Dev: QA"} {
		t.Run(status, func(t *testing.T) {
			var n urgentNode
			n.State = "OPEN"
			if err := json.Unmarshal([]byte(`{"issueFieldValues":{"nodes":[{"name":"Urgent","field":{"name":"Priority"}}]},"projectItems":{"nodes":[{"fieldValueByName":{"name":"Dev: In Progress"}},{"fieldValueByName":{"name":"`+status+`"}}]}}`), &n); err != nil {
				t.Fatal(err)
			}
			if n.urgent() {
				t.Fatal("blocked issue included")
			}
			n.ProjectItems.Nodes[1].FieldValueByName.Name = "Dev: Ready to go 🟢"
			if !n.urgent() {
				t.Fatal("unblocked issue not restored")
			}
			n.IssueFieldValues.Nodes = append(n.IssueFieldValues.Nodes, n.IssueFieldValues.Nodes[0])
			n.IssueFieldValues.Nodes[1].Field.Name = "Status"
			n.IssueFieldValues.Nodes[1].Name = status
			if n.urgent() {
				t.Fatal("native blocked status ignored")
			}
		})
	}
	for _, status := range []string{"", "Dev: In Progress 🚧", "Unblocked", "BI: QA", "Dev: QA Ready"} {
		if excludedUrgentStatus(status) {
			t.Fatalf("incorrectly blocked: %s", status)
		}
	}
}

func TestMergeConflictBlocksReady(t *testing.T) {
	for _, approvals := range []int{0, 2} {
		for _, ci := range []string{"SUCCESS", "FAILURE"} {
			item := Item{Number: 10908, MergeStateStatus: "DIRTY", StatusCheckRollup: []Check{{Status: "COMPLETED", Conclusion: ci}}}
			section, badge, detail := OwnStatus(item, approvals, 2)
			if section != "mine" || badge != "CONFLICT" || !strings.Contains(detail, "merge conflict") {
				t.Fatalf("conflict not surfaced: %q %q %q", section, badge, detail)
			}
			if ci == "FAILURE" && !strings.Contains(detail, "CI:FAIL") {
				t.Fatal("conflict hid failing CI")
			}
		}
	}
	item := Item{MergeStateStatus: "CLEAN", ReviewDecision: "APPROVED"}
	section, badge, _ := OwnStatus(item, 2, 2)
	if section != "ready" || badge == "CONFLICT" {
		t.Fatal("resolved conflict still blocks merge")
	}
}

func TestDependabotAssignments(t *testing.T) {
	for _, author := range []string{"app/dependabot", "dependabot[bot]", "human"} {
		for _, tc := range []struct {
			assignees string
			want      bool
		}{
			{`[]`, true},
			{`[{"login":"OtherReviewer"}]`, false},
			{`[{"login":"brianleishman"}]`, true},
			{`[{"login":"OtherReviewer"},{"login":"BrianLeishman"}]`, true},
		} {
			var item Item
			if err := json.Unmarshal([]byte(`{"author":{"login":"`+author+`"},"assignees":`+tc.assignees+`}`), &item); err != nil {
				t.Fatal(err)
			}
			want := tc.want || author == "human"
			if item.visibleTo("BrianLeishman") != want {
				t.Fatalf("%s assigned %s: want visible %t", author, tc.assignees, want)
			}
		}
	}
}
