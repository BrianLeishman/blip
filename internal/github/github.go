// Package github fetches read-only GitHub data using the user's gh login.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BrianLeishman/blip/dashboard"
)

type Config struct {
	UrgentQuery       string   `json:"urgent_query,omitempty"`
	Repositories      []string `json:"repositories,omitempty"`
	Owners            []string `json:"owners,omitempty"`
	IncludeDependabot bool     `json:"include_dependabot,omitempty"`
	AdditionalAuthors []string `json:"additional_authors,omitempty"`
	RequiredApprovals int      `json:"required_approvals"`
}
type Item struct {
	Number                                                  int
	Title, URL, ReviewDecision, MergeStateStatus, UpdatedAt string
	IsDraft                                                 bool
	Author                                                  struct{ Login string }
	Assignees                                               []struct{ Login string }
	ReviewRequests                                          []json.RawMessage
	Labels                                                  []struct{ Name string }
	StatusCheckRollup                                       []Check
}
type Review struct {
	User        struct{ Login, Type string }
	State       string
	SubmittedAt string `json:"submitted_at"`
}
type Result struct {
	Snapshot dashboard.Snapshot
	URLs     map[string]string
}

func gh(ctx context.Context, dst any, args ...string) error {
	cmd := exec.CommandContext(ctx, "gh", args...)
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("gh %s: %w", args[0], err)
	}
	if err = json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

// Approvals uses each person's latest decisive review, not the number of review events.
// Comments don't revoke approvals; dismissed reviews don't count.
func Approvals(reviews []Review) int {
	latest := map[string]Review{}
	for _, r := range reviews {
		if r.User.Login == "" || r.User.Type == "Bot" || r.State == "PENDING" || r.State == "COMMENTED" {
			continue
		}
		old, ok := latest[r.User.Login]
		if !ok || r.SubmittedAt >= old.SubmittedAt {
			latest[r.User.Login] = r
		}
	}
	n := 0
	for _, r := range latest {
		if r.State == "APPROVED" {
			n++
		}
	}
	return n
}
func ValidRepository(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return false
		}
		for _, r := range p {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}
func Validate(c Config) error {
	if len(c.Repositories) == 0 && len(c.Owners) == 0 {
		return fmt.Errorf("configure at least one repository or owner")
	}
	for _, repo := range c.Repositories {
		if !ValidRepository(repo) {
			return fmt.Errorf("invalid repository %q", repo)
		}
	}
	for _, owner := range c.Owners {
		if strings.Contains(owner, "/") || !ValidRepository(owner+"/repo") {
			return fmt.Errorf("invalid owner %q", owner)
		}
	}
	for _, author := range c.AdditionalAuthors {
		if strings.HasPrefix(author, "-") || strings.Contains(author, "/") || !ValidRepository(author+"/repo") {
			return fmt.Errorf("invalid additional author %q", author)
		}
	}
	if c.RequiredApprovals < 1 {
		return fmt.Errorf("required_approvals must be positive")
	}
	return nil
}
func Fetch(ctx context.Context, c Config) (Result, error) {
	result := Result{Snapshot: dashboard.Snapshot{Version: 1, Revision: strconv.FormatInt(time.Now().UnixNano(), 10), Status: "GitHub connected"}, URLs: map[string]string{}}
	if err := Validate(c); err != nil {
		return result, err
	}
	var viewer struct{ Login string }
	if err := gh(ctx, &viewer, "api", "user"); err != nil {
		return result, err
	}
	if viewer.Login == "" {
		return result, fmt.Errorf("GitHub returned an empty viewer login")
	}
	repositories, err := discoverRepositories(ctx, c)
	if err != nil {
		return result, err
	}
	updated := map[string]string{}
	// Fetch up to four repositories concurrently; collect in input order so
	// network completion order never reshuffles the display.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	type repoResult struct {
		result  Result
		updated map[string]string
		err     error
	}
	results := make([]repoResult, len(repositories))
	slots := make(chan struct{}, 4)
	var workers sync.WaitGroup
	for i, repo := range repositories {
		workers.Add(1)
		go func() {
			defer workers.Done()
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-ctx.Done():
				results[i].err = ctx.Err()
				return
			}
			results[i].result, results[i].updated, results[i].err = fetchRepository(ctx, c, repo, viewer.Login)
		}()
	}
	workers.Wait()
	for _, r := range results {
		if r.err != nil {
			return result, r.err
		}
		result.Snapshot.Rows = append(result.Snapshot.Rows, r.result.Snapshot.Rows...)
		for id, link := range r.result.URLs {
			result.URLs[id] = link
		}
		for id, at := range r.updated {
			updated[id] = at
		}
	}

	if c.UrgentQuery != "" {
		rows, urls, err := fetchUrgent(ctx, c.UrgentQuery)
		if err != nil {
			return result, err
		}
		for _, r := range rows {
			if _, exists := result.URLs[r.ID]; !exists {
				result.Snapshot.Rows = append(result.Snapshot.Rows, r)
				result.URLs[r.ID] = urls[r.ID]
			}
		}
	}
	rank := map[string]int{"ready": 0, "mine": 1, "review": 2, "urgent": 3}
	sort.SliceStable(result.Snapshot.Rows, func(i, j int) bool {
		a, b := result.Snapshot.Rows[i], result.Snapshot.Rows[j]
		if a.Section == "review" && b.Section == "review" {
			return updated[a.ID] < updated[b.ID]
		}
		return rank[a.Section] < rank[b.Section]
	})
	total := len(result.Snapshot.Rows)
	if total > dashboard.MaxRows {
		result.Snapshot.Rows = result.Snapshot.Rows[:dashboard.MaxRows]
		result.Snapshot.Status = fmt.Sprintf("Showing %d of %d", dashboard.MaxRows, total)
	}

	return result, nil
}
func SafeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil
}

func fetchRepository(ctx context.Context, c Config, repo, viewer string) (Result, map[string]string, error) {
	result := Result{URLs: map[string]string{}}
	updated := map[string]string{}
	for _, query := range prQueries(c) {
		var items []Item
		var args []string
		args = []string{"pr", "list", "--repo", repo, "--state", "open", "--limit", "1000", "--json", "number,title,url,isDraft,reviewDecision,mergeStateStatus,updatedAt,author,assignees,reviewRequests,labels,statusCheckRollup"}
		if query.author != "" {
			args = append(args, "--author", query.author)
		} else {
			args = append(args, "--search", "is:open -is:draft review-requested:@me sort:updated-asc")
		}

		if err := gh(ctx, &items, args...); err != nil {
			return result, updated, fmt.Errorf("%s %s author=%q: %w", repo, query.section, query.author, err)
		}
		for _, item := range items {
			id := repo + "/pull/" + strconv.Itoa(item.Number)
			if _, exists := result.URLs[id]; exists {
				continue
			}
			if item.IsDraft || !item.visibleTo(viewer, c) {
				continue
			}
			if query.section == "review" {
				switch Checks(item.StatusCheckRollup) {
				case "!", "~":
					continue
				}
			}
			var pages [][]Review
			if err := gh(ctx, &pages, "api", "--paginate", "--slurp", fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=100", repo, item.Number)); err != nil {
				return result, updated, err
			}
			var reviews []Review
			for _, page := range pages {
				for _, r := range page {
					if r.User.Login != item.Author.Login {
						reviews = append(reviews, r)
					}
				}
			}
			n := Approvals(reviews)
			rowSection := query.section
			badge := fmt.Sprintf("%d/%d", n, c.RequiredApprovals)
			detail := "Review requested from you"
			if query.section == "review" {
				if n >= c.RequiredApprovals {
					continue
				}
				if item.MergeStateStatus == "DIRTY" {
					badge = "CONFLICT"
					detail = fmt.Sprintf("#%d %d/%d / merge conflict / CI:%s", item.Number, n, c.RequiredApprovals, Checks(item.StatusCheckRollup))
				}
			} else {
				rowSection, badge, detail = OwnStatus(item, n, c.RequiredApprovals)
			}
			kind := "pull"

			// Construct a known GitHub URL instead of accepting arbitrary serial URLs.
			link := "https://github.com/" + repo + "/" + kind + "/" + strconv.Itoa(item.Number)
			result.Snapshot.Rows = append(result.Snapshot.Rows, dashboard.Row{ID: id, Section: rowSection, Title: item.Title, Badge: badge, Detail: detail, ChecksRunning: ChecksRunning(item.StatusCheckRollup)})
			result.URLs[id] = link
			updated[id] = item.UpdatedAt
		}
	}
	return result, updated, nil
}

// Dependabot and additional-author work belongs here only while unassigned or assigned
// to the viewer. Apply this to review results too, so hidden PRs cannot reappear.
func (item Item) visibleTo(viewer string, c Config) bool {
	sharedAuthor := strings.EqualFold(item.Author.Login, "app/dependabot") || strings.EqualFold(item.Author.Login, "dependabot[bot]")
	for _, author := range c.AdditionalAuthors {
		sharedAuthor = sharedAuthor || strings.EqualFold(item.Author.Login, author)
	}
	if strings.EqualFold(item.Author.Login, viewer) || !sharedAuthor || len(item.Assignees) == 0 {
		return true
	}
	for _, assignee := range item.Assignees {
		if strings.EqualFold(assignee.Login, viewer) {
			return true
		}
	}
	return false
}
