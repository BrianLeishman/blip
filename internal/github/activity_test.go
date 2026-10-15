package github

import (
	"encoding/json"
	"testing"
)

func TestActivityIsStableAcrossCollectionOrdering(t *testing.T) {
	first := map[string]any{"head": "commit-a", "checks": []Check{{Name: "lint", Status: "COMPLETED", Conclusion: "SUCCESS"}, {Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"}}, "reviewers": []json.RawMessage{json.RawMessage(`{"login":"A","id":1}`), json.RawMessage(`{"login":"B","id":2}`)}}
	second := map[string]any{"head": "commit-a", "reviewers": []json.RawMessage{json.RawMessage(`{"id":2,"login":"B"}`), json.RawMessage(`{"id":1,"login":"A"}`)}, "checks": []Check{{Name: "test", Status: "COMPLETED", Conclusion: "SUCCESS"}, {Name: "lint", Status: "COMPLETED", Conclusion: "SUCCESS"}}}
	if activityToken(first) != activityToken(second) {
		t.Fatal("ordering alone changed activity")
	}
}

func TestSourceChangesWakeEvenWithoutVisibleBadgeChange(t *testing.T) {
	for _, tc := range []struct {
		name          string
		before, after any
	}{
		{"new commit", Item{HeadRefOid: "a"}, Item{HeadRefOid: "b"}},
		{"new comment", Item{UpdatedAt: "2026-10-15T10:00:00Z"}, Item{UpdatedAt: "2026-10-15T10:01:00Z"}},
		{"new review", []Review{{State: "APPROVED", SubmittedAt: "one"}}, []Review{{State: "APPROVED", SubmittedAt: "two"}}},
		{"CI rerun with same result", Check{Conclusion: "SUCCESS", CompletedAt: "one"}, Check{Conclusion: "SUCCESS", CompletedAt: "two"}},
		{"deployment retry with same result", workflowRun{ID: 1, RunAttempt: 1, Conclusion: "failure"}, workflowRun{ID: 1, RunAttempt: 2, Conclusion: "failure"}},
		{"edited comment", comment{ID: 1, Body: "before"}, comment{ID: 1, Body: "after"}},
		{"issue update", urgentNode{UpdatedAt: "one"}, urgentNode{UpdatedAt: "two"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if activityToken(tc.before) == activityToken(tc.after) {
				t.Fatal("source change did not affect token")
			}
		})
	}
}
