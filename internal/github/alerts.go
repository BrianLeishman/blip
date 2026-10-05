package github

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BrianLeishman/blip/dashboard"
)

// Deployments are an explicit allowlist, not all failures in a repository.
type DeploymentWorkflow struct {
	Repository string `json:"repository"`
	Workflow   string `json:"workflow"`
	Branch     string `json:"branch"`
}

func validWorkflow(s string) bool {
	return !strings.HasPrefix(s, "-") && !strings.Contains(s, "/") && ValidRepository("owner/"+s)
}

func (c Config) includesRepository(repo string) bool {
	owner, _, _ := strings.Cut(repo, "/")
	for _, configured := range c.Owners {
		if strings.EqualFold(owner, configured) {
			return true
		}
	}
	for _, configured := range c.Repositories {
		if strings.EqualFold(repo, configured) {
			return true
		}
	}
	return false
}

type notification struct {
	ID, Reason string
	Unread     bool
	UpdatedAt  string `json:"updated_at"`
	LastReadAt string `json:"last_read_at"`
	Repository struct {
		FullName string `json:"full_name"`
		Archived bool
	}
	Subject struct{ Title, Type, URL string }
}

type comment struct {
	ID          int64
	Body        string
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	SubmittedAt string `json:"submitted_at"`
	User        struct{ Login, Type string }
}

type cachedComments struct {
	signature string
	rows      []dashboard.Row
}

type cachedDeployment struct {
	at   time.Time
	rows []dashboard.Row
}

// Only accept API paths for the expected repository and resource type.
func subjectPath(raw, repo, kind string) (string, string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "api.github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", false
	}
	prefix := "/repos/" + repo + "/" + kind + "/"
	if !strings.HasPrefix(u.Path, prefix) {
		return "", "", false
	}
	number := strings.TrimPrefix(u.Path, prefix)
	if !positiveNumber(number) {
		return "", "", false
	}
	return strings.TrimPrefix(u.Path, "/"), number, true
}

func positiveNumber(s string) bool {
	if s == "" || s[0] < '1' || s[0] > '9' {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (client *Client) fetchAlerts(ctx context.Context, viewer string) ([]dashboard.Row, error) {
	var threads []notification
	if client.config.CommentsSince != "" {
		var pages [][]notification
		endpoint := "notifications?per_page=100&since=" + url.QueryEscape(client.config.CommentsSince)
		if err := gh(ctx, &pages, "api", "--paginate", "--slurp", endpoint); err != nil {
			return nil, fmt.Errorf("comment notifications: %w", err)
		}
		for _, page := range pages {
			threads = append(threads, page...)
		}
	}
	// Both types of alert use bounded concurrency, including comment detail calls.
	tasks := make([]func() ([]dashboard.Row, error), 0)
	var cacheMu sync.Mutex
	active := map[string]bool{}
	for _, n := range threads {
		if !n.Unread || n.Repository.Archived || !client.config.includesRepository(n.Repository.FullName) || (n.Subject.Type != "Issue" && n.Subject.Type != "PullRequest") {
			continue
		}
		active[n.ID] = true
		signature := n.UpdatedAt + "/" + n.LastReadAt + "/" + n.Subject.Title + "/" + n.Reason
		tasks = append(tasks, func() ([]dashboard.Row, error) {
			cacheMu.Lock()
			cached, ok := client.comments[n.ID]
			cacheMu.Unlock()
			if ok && cached.signature == signature {
				return cached.rows, nil
			}
			rows, err := fetchThreadComments(ctx, n, viewer, client.config.CommentsSince)
			if err == nil {
				cacheMu.Lock()
				client.comments[n.ID] = cachedComments{signature, rows}
				cacheMu.Unlock()
			}
			return rows, err
		})
	}
	for id := range client.comments {
		if !active[id] {
			delete(client.comments, id)
		}
	}
	// Check each configured repository once, even if it has no open PRs.
	repos := map[string]bool{}
	for _, d := range client.config.Deployments {
		if _, ok := repos[d.Repository]; !ok {
			var repo struct{ IsArchived bool }
			if err := gh(ctx, &repo, "repo", "view", d.Repository, "--json", "isArchived"); err != nil {
				return nil, err
			}
			repos[d.Repository] = repo.IsArchived
		}
		if repos[d.Repository] {
			continue
		}
		tasks = append(tasks, func() ([]dashboard.Row, error) {
			cacheMu.Lock()
			cached, ok := client.deployments[d]
			cacheMu.Unlock()
			if ok && time.Since(cached.at) < 2*time.Minute {
				return cached.rows, nil
			}
			rows, err := fetchDeployment(ctx, d)
			if err == nil {
				cacheMu.Lock()
				client.deployments[d] = cachedDeployment{time.Now(), rows}
				cacheMu.Unlock()
			}
			return rows, err
		})
	}
	results := make([][]dashboard.Row, len(tasks))
	errs := make([]error, len(tasks))
	slots := make(chan struct{}, 4)
	var workers sync.WaitGroup
	for i, task := range tasks {
		workers.Add(1)
		go func() {
			defer workers.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				errs[i] = ctx.Err()
				return
			}
			results[i], errs[i] = task()
		}()
	}
	workers.Wait()
	rows := []dashboard.Row{}
	seen := map[string]bool{}
	for i, r := range results {
		if errs[i] != nil {
			return nil, errs[i]
		}
		for _, row := range r {
			if !seen[row.ID] {
				rows = append(rows, row)
				seen[row.ID] = true
			}
		}
	}
	return rows, nil
}

func fetchThreadComments(ctx context.Context, n notification, viewer, since string) ([]dashboard.Row, error) {
	kind := "issues"
	browserKind := "issues"
	if n.Subject.Type == "PullRequest" {
		kind = "pulls"
		browserKind = "pull"
	}
	_, number, ok := subjectPath(n.Subject.URL, n.Repository.FullName, kind)
	if !ok {
		return nil, fmt.Errorf("invalid notification subject URL")
	}
	floor, _ := time.Parse(time.RFC3339, since)
	if lastRead, err := time.Parse(time.RFC3339, n.LastReadAt); err == nil && lastRead.After(floor) {
		floor = lastRead
	}
	type source struct{ path, anchor string }
	sources := []source{{"issues/" + number + "/comments?per_page=100&since=" + url.QueryEscape(floor.Format(time.RFC3339)), "issuecomment-"}}
	if kind == "pulls" {
		sources = append(sources, source{"pulls/" + number + "/comments?per_page=100", "discussion_r"}, source{"pulls/" + number + "/reviews?per_page=100", "pullrequestreview-"})
	}
	type datedRow struct {
		at  time.Time
		row dashboard.Row
	}
	var dated []datedRow
	for _, s := range sources {
		var pages [][]comment
		if err := gh(ctx, &pages, "api", "--paginate", "--slurp", "repos/"+n.Repository.FullName+"/"+s.path); err != nil {
			return nil, fmt.Errorf("notification comments: %w", err)
		}
		for _, page := range pages {
			for _, c := range page {
				stamp := c.CreatedAt
				if c.SubmittedAt != "" {
					stamp = c.SubmittedAt
				}
				at, err := time.Parse(time.RFC3339, stamp)
				if err != nil || !at.After(floor) || c.ID <= 0 || strings.TrimSpace(c.Body) == "" || strings.EqualFold(c.User.Login, viewer) || c.User.Type == "Bot" || strings.HasSuffix(strings.ToLower(c.User.Login), "[bot]") {
					continue
				}
				// One row per unread thread: click the latest actual human comment, not a
				// notification's sometimes-placeholder latest_comment_url.
				badge := "NEW"
				if n.Reason == "mention" {
					badge = "@YOU"
				}
				id := n.Repository.FullName + "/" + browserKind + "/" + number + "#" + s.anchor + strconv.FormatInt(c.ID, 10)
				dated = append(dated, datedRow{at, dashboard.Row{ID: id, Section: "comment", Title: c.User.Login + ": " + n.Subject.Title, Badge: badge, Detail: "Comment from " + c.User.Login}})
			}
		}
	}
	sort.SliceStable(dated, func(i, j int) bool { return dated[i].at.After(dated[j].at) })
	if len(dated) > 0 {
		return []dashboard.Row{dated[0].row}, nil
	}
	return nil, nil
}

type workflowRun struct {
	ID                              int64
	Name, Event, Status, Conclusion string
	HeadBranch                      string                 `json:"head_branch"`
	CreatedAt                       string                 `json:"created_at"`
	PullRequests                    []struct{ Number int } `json:"pull_requests"`
}

func eligibleDeployment(run workflowRun, branch string) bool {
	if run.HeadBranch != branch || run.Status != "completed" || len(run.PullRequests) > 0 {
		return false
	}
	switch run.Event {
	case "push", "workflow_dispatch", "repository_dispatch", "release", "schedule":
	default:
		return false
	}
	switch run.Conclusion {
	case "success", "failure", "timed_out", "action_required", "startup_failure":
		return true
	}
	return false
}

func fetchDeployment(ctx context.Context, d DeploymentWorkflow) ([]dashboard.Row, error) {
	// A created range also avoids GitHub serving stale branch-filtered results.
	// No failure-only filter: a later successful run must clear an older failure.
	base := "repos/" + d.Repository + "/actions/workflows/" + url.PathEscape(d.Workflow) + "/runs?per_page=20&status=completed&branch=" + url.QueryEscape(d.Branch) + "&created=" + url.QueryEscape(">=2008-01-01")
	for page := 1; page <= 50; page++ {
		var response struct {
			WorkflowRuns []workflowRun `json:"workflow_runs"`
		}
		if err := gh(ctx, &response, "api", base+"&page="+strconv.Itoa(page)); err != nil {
			return nil, fmt.Errorf("deployment workflow: %w", err)
		}
		sort.SliceStable(response.WorkflowRuns, func(i, j int) bool { return response.WorkflowRuns[i].CreatedAt > response.WorkflowRuns[j].CreatedAt })
		for _, r := range response.WorkflowRuns {
			if !eligibleDeployment(r, d.Branch) {
				continue
			}
			if r.Conclusion == "success" {
				return nil, nil
			}
			if r.ID <= 0 {
				return nil, fmt.Errorf("invalid deployment run ID")
			}
			return []dashboard.Row{{ID: d.Repository + "/actions/runs/" + strconv.FormatInt(r.ID, 10), Section: "deployment", Title: r.Name + " (" + d.Branch + ")", Badge: "FAIL", Detail: "Deployment failed / " + d.Branch}}, nil
		}
		if len(response.WorkflowRuns) < 20 {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("deployment workflow has no eligible result within 1000 runs")
}
