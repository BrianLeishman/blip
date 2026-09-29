package github

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Discover only repositories with relevant PRs, rather than polling every repo
// owned by a large organization. Search is repeated on every refresh so new
// repositories are picked up automatically.
func discoverRepositories(ctx context.Context, c Config) ([]string, error) {
	repos := map[string]string{}
	for _, repo := range c.Repositories {
		var metadata struct{ IsArchived bool }
		if err := gh(ctx, &metadata, "repo", "view", repo, "--json", "isArchived"); err != nil {
			return nil, fmt.Errorf("check repository %s: %w", repo, err)
		}
		if !metadata.IsArchived {
			repos[strings.ToLower(repo)] = repo
		}
	}
	if len(c.Owners) > 0 {
		for _, query := range prQueries(c) {
			filter, author := "--author", query.author
			if query.section == "review" {
				filter = "--review-requested"
				author = "@me"
			}
			args := []string{"search", "prs", "--state", "open", "--draft=false", "--archived=false", filter, author, "--limit", "1000", "--json", "repository"}
			for _, owner := range c.Owners {
				args = append(args, "--owner", owner)
			}
			var items []struct {
				Repository struct{ NameWithOwner string }
			}
			if err := gh(ctx, &items, args...); err != nil {
				return nil, fmt.Errorf("discover repositories: %w", err)
			}
			if len(items) >= 1000 {
				return nil, fmt.Errorf("owner PR search reached GitHub's 1000-result limit; narrow the owners")
			}
			for _, item := range items {
				repo := item.Repository.NameWithOwner
				if !ValidRepository(repo) {
					return nil, fmt.Errorf("invalid repository from search: %q", repo)
				}
				repos[strings.ToLower(repo)] = repo
			}
		}
	}
	result := make([]string, 0, len(repos))
	for _, repo := range repos {
		result = append(result, repo)
	}
	sort.Strings(result)
	return result, nil
}

type prQuery struct {
	section string
	author  string
}

// Use the same author searches for discovery and fetching. Fetch owned PRs first
// so review requests for those PRs cannot produce duplicate or misclassified rows.
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
