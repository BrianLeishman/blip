package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type prQuery struct {
	section string
	author  string
}

// Owned searches come first so a review request on one of those PRs cannot
// produce a second, misclassified row.
func prQueries(c Config) []prQuery {
	queries := []prQuery{{section: "ready", author: "@me"}}
	if c.IncludeDependabot {
		queries = append(queries, prQuery{section: "ready", author: "dependabot[bot]"})
	}
	seen := map[string]bool{"@me": true}
	for _, author := range c.AdditionalAuthors {
		key := strings.ToLower(author)
		if !seen[key] {
			queries = append(queries, prQuery{section: "ready", author: author})
			seen[key] = true
		}
	}
	return append(queries, prQuery{section: "review"})
}

// GitHub caps one GraphQL request at 500,000 nodes, and additional authors are unbounded.
const searchBatch = 8

const prFragment = `fragment pr on PullRequest{number title isDraft reviewDecision mergeable mergeStateStatus canBeRebased updatedAt headRefOid repository{nameWithOwner rebaseMergeAllowed} author{__typename login} assignees(first:100){nodes{login}} reviewRequests(first:100){nodes{requestedReviewer{__typename ...on User{login} ...on Bot{login} ...on Team{slug}}}} labels(first:100){nodes{name}} reviews(last:100){pageInfo{hasPreviousPage} nodes{state submittedAt author{__typename login}}} commits(last:1){nodes{commit{statusCheckRollup{contexts(first:100){pageInfo{hasNextPage} nodes{__typename ...on CheckRun{name status conclusion startedAt completedAt} ...on StatusContext{context state createdAt}}}}}}}}`

// GraphQL reports a bot as login "dependabot" plus type Bot. Callers still compare
// the historical dependabot[bot] and app/dependabot forms.
type graphQLActor struct {
	Typename string `json:"__typename"`
	Login    string
}

func (a graphQLActor) login() string {
	if a.Typename == "Bot" {
		return a.Login + "[bot]"
	}
	return a.Login
}

type gqlPR struct {
	Number           int
	Title            string
	IsDraft          bool
	ReviewDecision   string
	MergeStateStatus string
	UpdatedAt        string
	HeadRefOid       string
	Mergeable        string
	CanBeRebased     bool
	Repository       struct {
		NameWithOwner      string
		RebaseMergeAllowed bool
	}
	Author         graphQLActor
	Assignees      struct{ Nodes []struct{ Login string } }
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				Typename string `json:"__typename"`
				Login    string
				Slug     string
			}
		}
	}
	Labels  struct{ Nodes []struct{ Name string } }
	Reviews struct {
		PageInfo struct{ HasPreviousPage bool }
		Nodes    []struct {
			State       string
			SubmittedAt string
			Author      graphQLActor
		}
	}
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup struct {
					Contexts struct {
						PageInfo struct{ HasNextPage bool }
						Nodes    []gqlCheck
					}
				}
			}
		}
	}
}

type gqlCheck struct {
	Typename    string `json:"__typename"`
	Name        string
	Status      string
	Conclusion  string
	StartedAt   string
	CompletedAt string
	Context     string
	State       string
	CreatedAt   string
}

type searchPage struct {
	IssueCount int
	PageInfo   struct {
		HasNextPage bool
		EndCursor   string
	}
	Nodes []json.RawMessage
}

type pulledPR struct {
	section     string
	item        Item
	restReviews bool
}

func prSearch(c Config, q prQuery) string {
	parts := []string{"is:pr", "is:open", "draft:false", "archived:false"}
	for _, owner := range c.Owners {
		parts = append(parts, "user:"+owner)
	}
	for _, repo := range c.Repositories {
		parts = append(parts, "repo:"+repo)
	}
	if q.section == "review" {
		return strings.Join(append(parts, "review-requested:@me", "sort:updated-asc"), " ")
	}
	author := q.author
	if author == "dependabot[bot]" {
		author = "app/dependabot"
	}
	return strings.Join(append(parts, "author:"+author, "sort:created-desc"), " ")
}

func searchQuery(n int, viewer, after bool) string {
	var b strings.Builder
	b.WriteString("query(")
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "$q%d:String!", i)
	}
	if after {
		b.WriteString(",$after:String!")
	}
	b.WriteString("){")
	if viewer {
		b.WriteString("viewer{login}")
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "q%d:search(query:$q%d,type:ISSUE,first:100", i, i)
		if after {
			b.WriteString(",after:$after")
		}
		b.WriteString("){issueCount pageInfo{hasNextPage endCursor} nodes{...pr}}")
	}
	b.WriteByte('}')
	b.WriteString(prFragment)
	return b.String()
}

func (client *Client) searchPullRequests(ctx context.Context) (string, []pulledPR, error) {
	queries := prQueries(client.config)
	var viewer string
	var all []pulledPR
	for start := 0; start < len(queries); start += searchBatch {
		end := min(start+searchBatch, len(queries))
		login, got, err := client.searchChunk(ctx, queries[start:end], viewer == "")
		if err != nil {
			return "", nil, err
		}
		if viewer == "" {
			if login == "" {
				return "", nil, fmt.Errorf("GitHub returned an empty viewer login")
			}
			viewer = login
		}
		all = append(all, got...)
	}
	return viewer, all, nil
}

func (client *Client) searchChunk(ctx context.Context, queries []prQuery, withViewer bool) (string, []pulledPR, error) {
	searches := make([]string, len(queries))
	for i, q := range queries {
		searches[i] = prSearch(client.config, q)
	}
	pages, viewer, err := client.querySearches(ctx, searches, "", withViewer)
	if err != nil {
		return "", nil, err
	}
	var all []pulledPR
	for i, page := range pages {
		got, err := decodeSearchPage(queries[i], page)
		if err != nil {
			return "", nil, err
		}
		cursor := page.PageInfo.EndCursor
		for page.PageInfo.HasNextPage {
			next, _, err := client.querySearches(ctx, []string{searches[i]}, cursor, false)
			if err != nil {
				return "", nil, err
			}
			page = next[0]
			if page.PageInfo.EndCursor == "" || page.PageInfo.EndCursor == cursor {
				return "", nil, fmt.Errorf("pull request pagination did not advance")
			}
			cursor = page.PageInfo.EndCursor
			more, err := decodeSearchPage(queries[i], page)
			if err != nil {
				return "", nil, err
			}
			got = append(got, more...)
		}
		all = append(all, got...)
	}
	return viewer, all, nil
}

func (client *Client) querySearches(ctx context.Context, searches []string, after string, withViewer bool) ([]searchPage, string, error) {
	args := []string{"api", "graphql", "-f", "query=" + searchQuery(len(searches), withViewer, after != "")}
	for i, search := range searches {
		args = append(args, "-f", fmt.Sprintf("q%d=%s", i, search))
	}
	if after != "" {
		args = append(args, "-f", "after="+after)
	}
	var response struct {
		Data   map[string]json.RawMessage
		Errors []struct{ Message string }
	}
	if err := gh(ctx, &response, args...); err != nil {
		return nil, "", err
	}
	if len(response.Errors) > 0 {
		return nil, "", fmt.Errorf("pull request query: %s", response.Errors[0].Message)
	}
	var viewer string
	if withViewer {
		var v struct{ Login string }
		if raw, ok := response.Data["viewer"]; ok {
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, "", fmt.Errorf("decode GitHub response: %w", err)
			}
		}
		viewer = v.Login
	}
	pages := make([]searchPage, len(searches))
	for i := range searches {
		raw, ok := response.Data[fmt.Sprintf("q%d", i)]
		if !ok {
			return nil, "", fmt.Errorf("pull request query: missing q%d", i)
		}
		if err := json.Unmarshal(raw, &pages[i]); err != nil {
			return nil, "", fmt.Errorf("decode GitHub response: %w", err)
		}
	}
	return pages, viewer, nil
}

func decodeSearchPage(q prQuery, page searchPage) ([]pulledPR, error) {
	if page.IssueCount > 1000 {
		return nil, fmt.Errorf("owner PR search reached GitHub's 1000-result limit; narrow the owners")
	}
	var got []pulledPR
	for _, raw := range page.Nodes {
		if body := strings.TrimSpace(string(raw)); body == "" || body == "null" {
			continue
		}
		var node gqlPR
		if err := json.Unmarshal(raw, &node); err != nil {
			return nil, fmt.Errorf("decode GitHub response: %w", err)
		}
		item, restReviews, err := node.item()
		if err != nil {
			return nil, err
		}
		got = append(got, pulledPR{section: q.section, item: item, restReviews: restReviews})
	}
	return got, nil
}

func (n gqlPR) item() (Item, bool, error) {
	repo := n.Repository.NameWithOwner
	if !ValidRepository(repo) {
		return Item{}, false, fmt.Errorf("invalid repository from search: %q", repo)
	}
	item := Item{
		HeadRefOid:       n.HeadRefOid,
		Number:           n.Number,
		Title:            n.Title,
		ReviewDecision:   n.ReviewDecision,
		MergeStateStatus: n.MergeStateStatus,
		UpdatedAt:        n.UpdatedAt,
		IsDraft:          n.IsDraft,
		Repository:       repo,
		// canBeRebased is also false while GitHub recomputes mergeability after a push
		// to the base branch (mergeable UNKNOWN). That is not a conflict.
		RebaseBlocked: n.Repository.RebaseMergeAllowed && n.Mergeable == "MERGEABLE" && !n.CanBeRebased,
	}
	if len(n.Commits.Nodes) > 0 {
		contexts := n.Commits.Nodes[0].Commit.StatusCheckRollup.Contexts
		if contexts.PageInfo.HasNextPage {
			return Item{}, false, fmt.Errorf("pull request %d exceeds 100 checks; cannot verify CI", n.Number)
		}
		for _, c := range contexts.Nodes {
			// A StatusContext fills Context/State/CreatedAt, a CheckRun the rest.
			item.StatusCheckRollup = append(item.StatusCheckRollup, Check{Name: c.Name, Status: c.Status, Conclusion: c.Conclusion, StartedAt: c.StartedAt, CompletedAt: c.CompletedAt, Context: c.Context, State: c.State, CreatedAt: c.CreatedAt})
		}
	}
	item.Author.Login = n.Author.login()
	for _, r := range n.ReviewRequests.Nodes {
		who := r.RequestedReviewer.Login
		if r.RequestedReviewer.Typename == "Team" {
			who = r.RequestedReviewer.Slug
		}
		item.ReviewRequests = append(item.ReviewRequests, who)
	}
	item.Assignees = n.Assignees.Nodes
	item.Labels = n.Labels.Nodes
	if n.Reviews.PageInfo.HasPreviousPage {
		return item, true, nil
	}
	for _, r := range n.Reviews.Nodes {
		rev := Review{State: r.State, SubmittedAt: r.SubmittedAt}
		rev.User.Login = r.Author.login()
		rev.User.Type = r.Author.Typename
		item.Reviews = append(item.Reviews, rev)
	}
	return item, false, nil
}
