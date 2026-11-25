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
	UrgentQuery       string               `json:"urgent_query,omitempty"`
	Repositories      []string             `json:"repositories,omitempty"`
	Owners            []string             `json:"owners,omitempty"`
	IncludeDependabot bool                 `json:"include_dependabot,omitempty"`
	AdditionalAuthors []string             `json:"additional_authors,omitempty"`
	CommentsSince     string               `json:"comments_since,omitempty"`
	Deployments       []DeploymentWorkflow `json:"deployments,omitempty"`
	RequiredApprovals int                  `json:"required_approvals"`
}
type Item struct {
	HeadRefOid                                         string
	Number                                             int
	Title, ReviewDecision, MergeStateStatus, UpdatedAt string
	IsDraft                                            bool
	// GitHub calls a PR mergeable when a squash is clean, even if the rebase merge the page offers is not.
	RebaseBlocked     bool
	Repository        string
	Author            struct{ Login string }
	Assignees         []struct{ Login string }
	ReviewRequests    []string
	Labels            []struct{ Name string }
	StatusCheckRollup []Check
	Reviews           []Review
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

type reviewOrder struct {
	approvals int
	updatedAt string
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
	if c.CommentsSince != "" {
		if _, err := time.Parse(time.RFC3339, c.CommentsSince); err != nil {
			return fmt.Errorf("comments_since must be an RFC3339 timestamp: %w", err)
		}
	}
	for _, d := range c.Deployments {
		if !ValidRepository(d.Repository) || !c.includesRepository(d.Repository) || !validWorkflow(d.Workflow) || d.Branch == "" || strings.ContainsAny(d.Branch, "\r\n\x00") {
			return fmt.Errorf("invalid deployment workflow configuration")
		}
	}
	return nil
}

// Client caches comments on unchanged notification threads between refreshes.
// Use one client per serial polling loop; Fetch calls must not overlap.
type Client struct {
	config      Config
	comments    map[string]cachedComments
	deployments map[DeploymentWorkflow]cachedDeployment
	restMu      sync.Mutex
	rest        map[string]cachedREST
}

func NewClient(c Config) *Client {
	return &Client{
		config:      c,
		comments:    map[string]cachedComments{},
		deployments: map[DeploymentWorkflow]cachedDeployment{},
		rest:        map[string]cachedREST{},
	}
}

func Fetch(ctx context.Context, c Config) (Result, error) {
	return NewClient(c).Fetch(ctx)
}

func (client *Client) Fetch(ctx context.Context) (Result, error) {
	c := client.config
	result := Result{Snapshot: dashboard.Snapshot{Version: 1, Revision: strconv.FormatInt(time.Now().UnixNano(), 10), Status: "GitHub connected"}, URLs: map[string]string{}}
	if err := Validate(c); err != nil {
		return result, err
	}
	viewer, pulled, err := client.searchPullRequests(ctx)
	if err != nil {
		return result, err
	}
	// Results arrive grouped by search, not by repository. Stable-sort by repository
	// so each repo's owned rows stay ahead of the next repo.
	sort.SliceStable(pulled, func(i, j int) bool {
		return pulled[i].item.Repository < pulled[j].item.Repository
	})
	order := map[string]reviewOrder{}
	for _, pr := range pulled {
		item := pr.item
		repo := item.Repository
		id := repo + "/pull/" + strconv.Itoa(item.Number)
		if _, exists := result.URLs[id]; exists {
			continue
		}
		if item.IsDraft || !item.visibleTo(viewer, c) {
			continue
		}
		if pr.section == "review" {
			if item.conflicted() {
				continue
			}
			switch Checks(item.StatusCheckRollup) {
			case "!", "~":
				continue
			}
		}
		if pr.restReviews {
			item.Reviews, err = restAll[Review](ctx, client, fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=100", repo, item.Number))
			if err != nil {
				return result, err
			}
		}
		var reviews []Review
		for _, r := range item.Reviews {
			if r.User.Login != item.Author.Login {
				reviews = append(reviews, r)
			}
		}
		item.Reviews = reviews
		n := Approvals(reviews)
		rowSection := pr.section
		badge := fmt.Sprintf("%d/%d", n, c.RequiredApprovals)
		detail := "Review requested from you"
		if pr.section == "review" {
			if n >= c.RequiredApprovals {
				continue
			}
		} else {
			rowSection, badge, detail = OwnStatus(item, n, c.RequiredApprovals)
		}
		// Construct a known GitHub URL instead of accepting arbitrary serial URLs.
		link := "https://github.com/" + repo + "/pull/" + strconv.Itoa(item.Number)
		row := dashboard.Row{ID: id, Section: rowSection, Title: item.Title, Badge: badge, Detail: detail, ChecksRunning: ChecksRunning(item.StatusCheckRollup)}
		row.Activity = activityToken(struct {
			Item Item
			Row  dashboard.Row
		}{item, row})
		result.Snapshot.Rows = append(result.Snapshot.Rows, row)
		result.URLs[id] = link
		order[id] = reviewOrder{approvals: n, updatedAt: item.UpdatedAt}
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
	alerts, err := client.fetchAlerts(ctx, viewer)
	if err != nil {
		return result, err
	}
	result.Snapshot.Rows = append(result.Snapshot.Rows, alerts...)
	for _, row := range alerts {
		result.URLs[row.ID] = "https://github.com/" + row.ID
	}
	rank := map[string]int{"deployment": 0, "comment": 1, "ready": 2, "mine": 3, "review": 4, "urgent": 5}
	sort.SliceStable(result.Snapshot.Rows, func(i, j int) bool {
		a, b := result.Snapshot.Rows[i], result.Snapshot.Rows[j]
		if a.Section == "review" && b.Section == "review" {
			x, y := order[a.ID], order[b.ID]
			if x.approvals != y.approvals {
				return x.approvals > y.approvals
			}
			return x.updatedAt < y.updatedAt
		}
		return rank[a.Section] < rank[b.Section]
	})
	return result, nil
}
func SafeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil
}

func (item Item) conflicted() bool {
	return item.MergeStateStatus == "DIRTY" || item.RebaseBlocked
}

func (item Item) ownedBy(viewer string, c Config) bool {
	if !item.visibleTo(viewer, c) {
		return false
	}
	if strings.EqualFold(item.Author.Login, viewer) {
		return true
	}
	if c.IncludeDependabot && (strings.EqualFold(item.Author.Login, "app/dependabot") || strings.EqualFold(item.Author.Login, "dependabot[bot]")) {
		return true
	}
	for _, author := range c.AdditionalAuthors {
		if strings.EqualFold(item.Author.Login, author) {
			return true
		}
	}
	return false
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
