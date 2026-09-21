package github

import (
	"encoding/json"
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
