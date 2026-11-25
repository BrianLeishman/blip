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

// GitHub answers many pull requests in one request; 100 keeps each request small.
const pullStateBatch = 100

// pullStates reads the draft, conflict and ownership state of "owner/name/pull/N" ids.
// Every part of an id must already be validated: it is written into the query text.
func pullStates(ctx context.Context, ids []string) (map[string]Item, error) {
	states := map[string]Item{}
	for start := 0; start < len(ids); start += pullStateBatch {
		batch := ids[start:min(start+pullStateBatch, len(ids))]
		var query strings.Builder
		query.WriteString("query{")
		for i, id := range batch {
			parts := strings.Split(id, "/")
			fmt.Fprintf(&query, "p%d:repository(owner:%q,name:%q){rebaseMergeAllowed pullRequest(number:%s){isDraft mergeable mergeStateStatus canBeRebased author{__typename login} assignees(first:100){nodes{login}}}}", i, parts[0], parts[1], parts[3])
		}
		query.WriteString("}")
		var response struct {
			Data map[string]struct {
				RebaseMergeAllowed bool
				PullRequest        *struct {
					IsDraft          bool
					MergeStateStatus string
					Mergeable        string
					CanBeRebased     bool
					Author           graphQLActor
					Assignees        struct{ Nodes []struct{ Login string } }
				}
			}
			Errors []struct{ Message string }
		}
		if err := gh(ctx, &response, "api", "graphql", "-f", "query="+query.String()); err != nil {
			return nil, err
		}
		if len(response.Errors) > 0 {
			return nil, fmt.Errorf("notification pull request query: %s", response.Errors[0].Message)
		}
		for i, id := range batch {
			repo := response.Data[fmt.Sprintf("p%d", i)]
			pr := repo.PullRequest
			if pr == nil {
				return nil, fmt.Errorf("notification pull request %s missing", id)
			}
			item := Item{IsDraft: pr.IsDraft, MergeStateStatus: pr.MergeStateStatus, RebaseBlocked: repo.RebaseMergeAllowed && pr.Mergeable == "MERGEABLE" && !pr.CanBeRebased, Assignees: pr.Assignees.Nodes}
			item.Author.Login = pr.Author.login()
			states[id] = item
		}
	}
	return states, nil
}

func (client *Client) fetchAlerts(ctx context.Context, viewer string) ([]dashboard.Row, error) {
	var threads []notification
	if client.config.CommentsSince != "" {
		endpoint := "notifications?per_page=100&since=" + url.QueryEscape(client.config.CommentsSince)
		var err error
		threads, err = restAll[notification](ctx, client, endpoint)
		if err != nil {
			return nil, fmt.Errorf("comment notifications: %w", err)
		}
	}
	// Both types of alert use bounded concurrency, including comment detail calls.
	tasks := make([]func() ([]dashboard.Row, error), 0)
	var cacheMu sync.Mutex
	active := map[string]bool{}
	// One batched lookup after the loop fills this; the tasks run after that.
	var pulls map[string]Item
	var pullIDs []string
	for _, n := range threads {
		if !n.Unread || n.Repository.Archived || !client.config.includesRepository(n.Repository.FullName) || (n.Subject.Type != "Issue" && n.Subject.Type != "PullRequest") {
			continue
		}
		active[n.ID] = true
		signature := n.UpdatedAt + "/" + n.LastReadAt + "/" + n.Subject.Title + "/" + n.Reason
		pullID := ""
		if n.Subject.Type == "PullRequest" {
			_, number, ok := subjectPath(n.Subject.URL, n.Repository.FullName, "pulls")
			if !ok || !ValidRepository(n.Repository.FullName) {
				return nil, fmt.Errorf("invalid notification subject URL")
			}
			pullID = n.Repository.FullName + "/pull/" + number
			pullIDs = append(pullIDs, pullID)
		}
		tasks = append(tasks, func() ([]dashboard.Row, error) {
			var pr Item
			if pullID != "" {
				// Draft changes need not create a notification. Check current PR state
				// even when its comment contents are already cached.
				pr = pulls[pullID]
				if pr.IsDraft {
					return nil, nil
				}
				if pr.conflicted() && !pr.ownedBy(viewer, client.config) {
					return nil, nil
				}
			}
			cacheMu.Lock()
			cached, ok := client.comments[n.ID]
			cacheMu.Unlock()
			if ok && cached.signature == signature {
				return cached.rows, nil
			}
			rows, err := client.fetchThreadComments(ctx, n, viewer, client.config.CommentsSince, pr)
			if err == nil {
				cacheMu.Lock()
				client.comments[n.ID] = cachedComments{signature, rows}
				cacheMu.Unlock()
			}
			return rows, err
		})
	}
	var err error
	if pulls, err = pullStates(ctx, pullIDs); err != nil {
		return nil, err
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
			rows, err := client.fetchDeployment(ctx, d)
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

func (client *Client) fetchThreadComments(ctx context.Context, n notification, viewer, since string, pr Item) ([]dashboard.Row, error) {
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
		// Look at earlier participation too, not just comments after the cutoff.
		sources[0].path = "issues/" + number + "/comments?per_page=100"
		sources = append(sources, source{"pulls/" + number + "/comments?per_page=100", "discussion_r"}, source{"pulls/" + number + "/reviews?per_page=100", "pullrequestreview-"})
	}
	type datedRow struct {
		at  time.Time
		row dashboard.Row
	}
	var dated []datedRow
	participated := strings.EqualFold(pr.Author.Login, viewer)
	mentioned := false
	for _, s := range sources {
		comments, err := restAll[comment](ctx, client, "repos/"+n.Repository.FullName+"/"+s.path)
		if err != nil {
			return nil, fmt.Errorf("notification comments: %w", err)
		}
		for _, c := range comments {
			stamp := c.CreatedAt
			if c.SubmittedAt != "" {
				stamp = c.SubmittedAt
			}
			at, err := time.Parse(time.RFC3339, stamp)
			if err == nil && strings.EqualFold(c.User.Login, viewer) {
				participated = true
			}
			if err != nil || !at.After(floor) || c.ID <= 0 || strings.TrimSpace(c.Body) == "" || strings.EqualFold(c.User.Login, viewer) || c.User.Type == "Bot" || strings.HasSuffix(strings.ToLower(c.User.Login), "[bot]") {
				continue
			}
			mentioned = mentioned || mentionsUser(c.Body, viewer)
			// One row per unread thread: click the latest actual human comment, not a
			// notification's sometimes-placeholder latest_comment_url.
			badge := "NEW"
			if n.Reason == "mention" {
				badge = "@YOU"
			}
			id := n.Repository.FullName + "/" + browserKind + "/" + number + "#" + s.anchor + strconv.FormatInt(c.ID, 10)
			activity := activityToken(struct {
				Comment comment
				Title   string
			}{c, n.Subject.Title})
			dated = append(dated, datedRow{at, dashboard.Row{ID: id, Section: "comment", Title: c.User.Login + ": " + n.Subject.Title, Badge: badge, Detail: "Comment from " + c.User.Login, Activity: activity}})
		}
	}
	if kind == "pulls" && !participated && !mentioned {
		return nil, nil
	}
	sort.SliceStable(dated, func(i, j int) bool { return dated[i].at.After(dated[j].at) })
	if len(dated) > 0 {
		return []dashboard.Row{dated[0].row}, nil
	}
	return nil, nil
}

func mentionsUser(body, viewer string) bool {
	for _, word := range strings.FieldsFunc(body, func(r rune) bool {
		return !(r == '@' || r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
	}) {
		if strings.EqualFold(strings.TrimRight(word, "."), "@"+viewer) {
			return true
		}
	}
	return false
}

type workflowRun struct {
	RunAttempt                      int    `json:"run_attempt"`
	UpdatedAt                       string `json:"updated_at"`
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
	case "success", "cancelled", "failure", "timed_out", "action_required", "startup_failure":
		return true
	}
	return false
}

func (client *Client) fetchDeployment(ctx context.Context, d DeploymentWorkflow) ([]dashboard.Row, error) {
	// Read the latest unfiltered history, then apply branch/event/outcome rules
	// locally. Filtered searches can return older subsets of a busy workflow.
	base := "repos/" + d.Repository + "/actions/workflows/" + url.PathEscape(d.Workflow) + "/runs?per_page=20"
	for page := 1; page <= 50; page++ {
		var response struct {
			WorkflowRuns []workflowRun `json:"workflow_runs"`
		}
		if err := client.restJSON(ctx, &response, base+"&page="+strconv.Itoa(page)); err != nil {
			return nil, fmt.Errorf("deployment workflow: %w", err)
		}
		sort.SliceStable(response.WorkflowRuns, func(i, j int) bool { return response.WorkflowRuns[i].CreatedAt > response.WorkflowRuns[j].CreatedAt })
		for _, r := range response.WorkflowRuns {
			if !eligibleDeployment(r, d.Branch) {
				continue
			}
			if r.Conclusion == "success" || r.Conclusion == "cancelled" {
				return nil, nil
			}
			if r.ID <= 0 {
				return nil, fmt.Errorf("invalid deployment run ID")
			}
			return []dashboard.Row{{ID: d.Repository + "/actions/runs/" + strconv.FormatInt(r.ID, 10), Section: "deployment", Title: r.Name + " (" + d.Branch + ")", Badge: "FAIL", Detail: "Deployment failed / " + d.Branch, Activity: activityToken(r)}}, nil
		}
		if len(response.WorkflowRuns) < 20 {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("deployment workflow has no eligible result within 1000 runs")
}
