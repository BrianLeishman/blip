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
	"time"

	"github.com/BrianLeishman/blip/dashboard"
)

type Config struct {
	UrgentQuery       string   `json:"urgent_query,omitempty"`
	Repositories      []string `json:"repositories"`
	RequiredApprovals int      `json:"required_approvals"`
}
type Item struct {
	Number                                                  int
	Title, URL, ReviewDecision, MergeStateStatus, UpdatedAt string
	IsDraft                                                 bool
	Author                                                  struct{ Login string }
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
	if len(c.Repositories) == 0 {
		return fmt.Errorf("configure at least one repository")
	}
	for _, repo := range c.Repositories {
		if !ValidRepository(repo) {
			return fmt.Errorf("invalid repository %q", repo)
		}
	}
	if c.RequiredApprovals < 1 {
		return fmt.Errorf("required_approvals must be positive")
	}
	return nil
}
func Fetch(ctx context.Context, c Config) (Result, error) {
	result := Result{Snapshot: dashboard.Snapshot{Version: 1, Revision: strconv.FormatInt(time.Now().UnixNano(), 10), Status: "Updated " + time.Now().Format("15:04:05")}, URLs: map[string]string{}}
	if err := Validate(c); err != nil {
		return result, err
	}
	updated := map[string]string{}
	for _, repo := range c.Repositories {
		for _, section := range []string{"ready", "review"} {
			var items []Item
			var args []string
			args = []string{"pr", "list", "--repo", repo, "--state", "open", "--limit", "1000", "--json", "number,title,url,isDraft,reviewDecision,mergeStateStatus,updatedAt,author,reviewRequests,labels,statusCheckRollup"}
			if section == "ready" {
				args = append(args, "--author", "@me")
			} else {
				args = append(args, "--search", "is:open -is:draft review-requested:@me sort:updated-asc")
			}

			if err := gh(ctx, &items, args...); err != nil {
				return result, fmt.Errorf("%s %s: %w", repo, section, err)
			}
			for _, item := range items {
				if item.IsDraft {
					continue
				}
				var pages [][]Review
				if err := gh(ctx, &pages, "api", "--paginate", "--slurp", fmt.Sprintf("repos/%s/pulls/%d/reviews?per_page=100", repo, item.Number)); err != nil {
					return result, err
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
				rowSection := section
				badge := fmt.Sprintf("%d/%d", n, c.RequiredApprovals)
				detail := "Review requested from you"
				if section == "review" {
					if n >= c.RequiredApprovals {
						continue
					}
				} else {
					rowSection, badge, detail = OwnStatus(item, n, c.RequiredApprovals)
				}
				kind := "pull"

				// Construct a known GitHub URL instead of accepting arbitrary serial URLs.
				link := "https://github.com/" + repo + "/" + kind + "/" + strconv.Itoa(item.Number)
				id := repo + "/" + kind + "/" + strconv.Itoa(item.Number)
				result.Snapshot.Rows = append(result.Snapshot.Rows, dashboard.Row{ID: id, Section: rowSection, Title: item.Title, Badge: badge, Detail: detail})
				result.URLs[id] = link
				updated[id] = item.UpdatedAt
			}
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
		result.Snapshot.Status = fmt.Sprintf("Showing %d of %d / %s", dashboard.MaxRows, total, time.Now().Format("15:04"))
	}

	return result, nil
}
func SafeURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil
}
