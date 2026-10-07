package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeAlertsGH(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n"+script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestUnreadHumanCommentsAndCache(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 "api repos/Org/widget/pulls/7") echo "$PR" ;;
 *notifications\?*) echo "$THREADS" ;;
 *issues/7/comments*) echo '[[{"id":1,"body":"old","created_at":"2026-10-05T09:00:00Z","user":{"login":"Old"}},{"id":2,"body":"seen","created_at":"2026-10-05T10:30:00Z","user":{"login":"Seen"}},{"id":3,"body":"self","created_at":"2026-10-05T12:30:00Z","user":{"login":"viewer"}},{"id":4,"body":"bot","created_at":"2026-10-05T14:30:00Z","user":{"login":"automation","type":"Bot"}},{"id":5,"body":"human","created_at":"2026-10-05T11:30:00Z","user":{"login":"Teammate"}}]]' ;;
 *pulls/7/comments*) echo '[[{"id":6,"body":"inline","created_at":"2026-10-05T12:00:00Z","user":{"login":"Reviewer"}},{"id":8,"body":"bot","created_at":"2026-10-05T16:00:00Z","user":{"login":"app[bot]"}}]]' ;;
 *pulls/7/reviews*) echo '[[{"id":7,"body":"","submitted_at":"2026-10-05T15:00:00Z","user":{"login":"Approver"}}]]' ;;
 *) exit 1 ;;
esac
`)
	t.Setenv("PR", `{"draft":false,"user":{"login":"Author"}}`)
	thread := notification{ID: "1", Unread: true, Reason: "subscribed", UpdatedAt: "2026-10-05T16:00:00Z", LastReadAt: "2026-10-05T11:00:00Z"}
	thread.Repository.FullName = "Org/widget"
	thread.Subject.Type = "PullRequest"
	thread.Subject.Title = "Example change"
	thread.Subject.URL = "https://api.github.com/repos/Org/widget/pulls/7"
	setThreads := func(ns []notification) {
		b, err := json.Marshal([][]notification{ns})
		if err != nil {
			t.Fatal(err)
		}
		t.Setenv("THREADS", string(b))
	}
	foreign := thread
	foreign.ID = "2"
	foreign.Repository.FullName = "Elsewhere/widget"
	archived := thread
	archived.ID = "3"
	archived.Repository.Archived = true
	read := thread
	read.ID = "4"
	read.Unread = false
	workflow := thread
	workflow.ID = "5"
	workflow.Subject.Type = "CheckSuite"
	setThreads([]notification{thread, foreign, archived, read, workflow})
	client := NewClient(Config{Owners: []string{"Org"}, CommentsSince: "2026-10-05T10:00:00Z"})
	for i := 0; i < 2; i++ {
		rows, err := client.fetchAlerts(t.Context(), "Viewer")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].ID != "Org/widget/pull/7#discussion_r6" {
			t.Fatalf("rows: %#v", rows)
		}
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "issues/7/comments") != 1 {
		t.Fatal("unchanged threads refetched comments")
	}
	thread.UpdatedAt = "2026-10-05T17:00:00Z"
	setThreads([]notification{thread})
	if _, err := client.fetchAlerts(t.Context(), "Viewer"); err != nil {
		t.Fatal(err)
	}
	calls, _ = os.ReadFile(log)
	if strings.Count(string(calls), "issues/7/comments") != 2 {
		t.Fatal("new activity did not refresh comments")
	}
	// Changing draft status must hide a previously cached alert immediately.
	t.Setenv("PR", `{"draft":true,"user":{"login":"Author"}}`)
	rows, err := client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 0 {
		t.Fatalf("draft retained cached comments: %#v %v", rows, err)
	}
	t.Setenv("PR", `{"draft":false,"user":{"login":"Author"}}`)
	// Conflicts on somebody else's PR must not reappear as cached comment alerts.
	t.Setenv("PR", `{"draft":false,"mergeable_state":"dirty","user":{"login":"Author"}}`)
	rows, err = client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 0 {
		t.Fatalf("conflict retained cached comments: %#v %v", rows, err)
	}
	t.Setenv("PR", `{"draft":false,"mergeable_state":"dirty","user":{"login":"Viewer"}}`)
	rows, err = client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 1 {
		t.Fatalf("own conflict comment hidden: %#v %v", rows, err)
	}
	// Advancing the inbox's read time removes all earlier comments.
	thread.LastReadAt = "2026-10-05T18:00:00Z"
	setThreads([]notification{thread})
	rows, err = client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 0 {
		t.Fatalf("read comments retained: %#v %v", rows, err)
	}
}

func TestNotificationSubjectRejectsUntrustedPaths(t *testing.T) {
	for _, raw := range []string{"https://evil.test/repos/Org/widget/pulls/7", "https://api.github.com/repos/Other/widget/pulls/7", "https://api.github.com/repos/Org/widget/pulls/7?x=y", "https://api.github.com/repos/Org/widget/pulls/7/extra", "https://user@api.github.com/repos/Org/widget/pulls/7", "https://api.github.com/repos/Org/widget/pulls/0"} {
		if _, _, ok := subjectPath(raw, "Org/widget", "pulls"); ok {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestDeploymentRecoveryAndExclusionOfPRRuns(t *testing.T) {
	failure := workflowRun{ID: 10, Name: "API deploy", Event: "push", Status: "completed", Conclusion: "failure", HeadBranch: "main", CreatedAt: "2026-10-05T10:00:00Z"}
	for _, tc := range []struct {
		name        string
		newer       workflowRun
		wantFailure bool
	}{
		{"new success clears", workflowRun{ID: 11, Event: "push", Status: "completed", Conclusion: "success", HeadBranch: "main"}, false},
		{"retry still running", workflowRun{ID: 11, Event: "push", Status: "in_progress", HeadBranch: "main"}, true},
		{"cancelled retry clears older alert", workflowRun{ID: 11, Event: "push", Status: "completed", Conclusion: "cancelled", HeadBranch: "main"}, false},
		{"PR event", workflowRun{ID: 11, Event: "pull_request", Status: "completed", Conclusion: "success", HeadBranch: "main"}, true},
		{"PR target", workflowRun{ID: 11, Event: "pull_request_target", Status: "completed", Conclusion: "success", HeadBranch: "main"}, true},
		{"linked PR", workflowRun{ID: 11, Event: "push", Status: "completed", Conclusion: "success", HeadBranch: "main", PullRequests: []struct{ Number int }{{3}}}, true},
		{"other branch", workflowRun{ID: 11, Event: "push", Status: "completed", Conclusion: "success", HeadBranch: "feature"}, true},
		{"manual recovery", workflowRun{ID: 11, Event: "workflow_dispatch", Status: "completed", Conclusion: "success", HeadBranch: "main"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeAlertsGH(t, `echo "$RUNS"`)
			tc.newer.CreatedAt = "2026-10-05T12:00:00Z"
			b, err := json.Marshal(map[string]any{"workflow_runs": []workflowRun{failure, tc.newer}})
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("RUNS", string(b))
			rows, err := fetchDeployment(t.Context(), DeploymentWorkflow{"Org/widget", "deploy.yml", "main"})
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) == 1) != tc.wantFailure {
				t.Fatalf("rows: %#v", rows)
			}
			if len(rows) > 0 && rows[0].ID != "Org/widget/actions/runs/10" {
				t.Fatalf("wrong run: %s", rows[0].ID)
			}
		})
	}
}

func TestDeploymentDoesNotAlertForPRFailuresAlone(t *testing.T) {
	fakeAlertsGH(t, `echo '{"workflow_runs":[{"id":11,"event":"pull_request","status":"completed","conclusion":"failure","head_branch":"main"}]}'`)
	rows, err := fetchDeployment(t.Context(), DeploymentWorkflow{"Org/widget", "deploy.yml", "main"})
	if err != nil || len(rows) != 0 {
		t.Fatalf("PR failure leaked: %#v %v", rows, err)
	}
}

func TestAlertConfigurationValidation(t *testing.T) {
	valid := Config{Owners: []string{"Org"}, RequiredApprovals: 2, CommentsSince: "2026-10-05T00:00:00Z", Deployments: []DeploymentWorkflow{{"Org/widget", "deploy.yml", "release/stable"}}}
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	for _, d := range []DeploymentWorkflow{{"Other/widget", "deploy.yml", "main"}, {"Org/widget", "../deploy.yml", "main"}, {"Org/widget", "deploy.yml", ""}, {"Org/widget", "deploy.yml", "main\n"}} {
		c := valid
		c.Deployments = []DeploymentWorkflow{d}
		if err := Validate(c); err == nil {
			t.Fatalf("accepted %#v", d)
		}
	}
	valid.CommentsSince = "today"
	if err := Validate(valid); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestDeploymentPollingCache(t *testing.T) {
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", log)
	fakeAlertsGH(t, `echo "$*" >> "$CALLS"
case "$*" in
 "repo view "*) echo '{"isArchived":false}' ;;
 *) echo "$RUNS" ;;
esac
`)
	d := DeploymentWorkflow{"Org/widget", "deploy.yml", "main"}
	client := NewClient(Config{Owners: []string{"Org"}, Deployments: []DeploymentWorkflow{d}})
	t.Setenv("RUNS", `{"workflow_runs":[{"id":10,"event":"push","status":"completed","conclusion":"failure","head_branch":"main"}]}`)
	rows, err := client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 1 {
		t.Fatalf("initial failure: %#v %v", rows, err)
	}
	t.Setenv("RUNS", `{"workflow_runs":[{"id":11,"event":"push","status":"completed","conclusion":"success","head_branch":"main"}]}`)
	rows, err = client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 1 {
		t.Fatalf("cached failure: %#v %v", rows, err)
	}
	cache := client.deployments[d]
	cache.at = time.Now().Add(-3 * time.Minute)
	client.deployments[d] = cache
	rows, err = client.fetchAlerts(t.Context(), "Viewer")
	if err != nil || len(rows) != 0 {
		t.Fatalf("recovery: %#v %v", rows, err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "/actions/workflows/") != 2 {
		t.Fatal("deployment cache did not reduce polling")
	}
}

func TestPRCommentsRequireParticipationOrDirectMention(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		issueComments, reviews, body, author string
		visible                              bool
	}{
		{"initial author comment", "[]", "[]", "How to test this change", "Author", false},
		{"reviewed before cutoff", "[]", `[{"id":2,"body":"","submitted_at":"2026-10-04T10:00:00Z","user":{"login":"Viewer"}}]`, "Follow-up", "Author", true},
		{"commented before cutoff", `[{"id":2,"body":"Question","created_at":"2026-10-04T10:00:00Z","user":{"login":"Viewer"}}]`, "[]", "Follow-up", "Author", true},
		{"own PR", "[]", "[]", "Follow-up", "Viewer", true},
		{"direct mention", "[]", "[]", "Could @viewer take a look?", "Author", true},
		{"similar username", "[]", "[]", "Could @ViewerElse take a look?", "Author", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeAlertsGH(t, `case "$*" in
 *issues/7/comments*) echo "$COMMENTS" ;;
 *pulls/7/comments*) echo '[[]]' ;;
 *pulls/7/reviews*) echo "$REVIEWS" ;;
 *) exit 1 ;;
esac
`)
			// All three comment sources are paginated, including the thread's history.
			var comments []comment
			if err := json.Unmarshal([]byte(tc.issueComments), &comments); err != nil {
				t.Fatal(err)
			}
			latest := comment{ID: 3, Body: tc.body, CreatedAt: "2026-10-05T12:00:00Z"}
			latest.User.Login = "Author"
			comments = append(comments, latest)
			b, err := json.Marshal([][]comment{comments})
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("COMMENTS", string(b))
			t.Setenv("REVIEWS", "["+tc.reviews+"]")
			n := notification{Reason: "mention"} // Old mention reason alone must not bypass participation.
			n.Repository.FullName = "Org/widget"
			n.Subject.Type = "PullRequest"
			n.Subject.URL = "https://api.github.com/repos/Org/widget/pulls/7"
			pr := commentPR{}
			pr.User.Login = tc.author
			rows, err := fetchThreadComments(t.Context(), n, "Viewer", "2026-10-05T00:00:00Z", pr)
			if err != nil {
				t.Fatal(err)
			}
			if (len(rows) > 0) != tc.visible {
				t.Fatalf("rows %#v", rows)
			}
		})
	}
}

func TestCancelledDeploymentsNeverAlertOrResurrectOlderFailures(t *testing.T) {
	for _, tc := range []struct {
		name, runs string
		wantID     string
	}{
		{"cancelled alone", `[{"id":11,"event":"push","status":"completed","conclusion":"cancelled","head_branch":"main","created_at":"2026-10-07T12:00:00Z"}]`, ""},
		{"latest success", `[{"id":10,"event":"push","status":"completed","conclusion":"failure","head_branch":"main","created_at":"2026-10-06T12:00:00Z"},{"id":11,"event":"push","status":"completed","conclusion":"success","head_branch":"main","created_at":"2026-10-07T12:00:00Z"}]`, ""},
		{"later real failure", `[{"id":10,"event":"push","status":"completed","conclusion":"cancelled","head_branch":"main","created_at":"2026-10-06T12:00:00Z"},{"id":11,"event":"push","status":"completed","conclusion":"failure","head_branch":"main","created_at":"2026-10-07T12:00:00Z"}]`, "Org/widget/actions/runs/11"},
		{"PR cancellation must not clear production failure", `[{"id":10,"event":"push","status":"completed","conclusion":"failure","head_branch":"main","created_at":"2026-10-06T12:00:00Z"},{"id":11,"event":"pull_request","status":"completed","conclusion":"cancelled","head_branch":"main","created_at":"2026-10-07T12:00:00Z"}]`, "Org/widget/actions/runs/10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeAlertsGH(t, `case "$*" in
 *branch=*|*status=*|*created=*) exit 1 ;;
 *) echo "$RUNS" ;;
esac
`)
			t.Setenv("RUNS", `{"workflow_runs":`+tc.runs+`}`)
			rows, err := fetchDeployment(t.Context(), DeploymentWorkflow{"Org/widget", "deploy.yml", "main"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantID == "" {
				if len(rows) != 0 {
					t.Fatalf("unexpected alert: %#v", rows)
				}
			} else if len(rows) != 1 || rows[0].ID != tc.wantID {
				t.Fatalf("wrong alert: %#v", rows)
			}
		})
	}
}
