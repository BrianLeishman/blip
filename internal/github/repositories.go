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
		for _, section := range prSections(c) {
			filter, author := "--author", "@me"
			if section == "review" {
				filter = "--review-requested"
			} else if section == "dependabot" {
				author = "dependabot[bot]"
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

// Process owned PRs before review requests so overlapping results appear once.
func prSections(c Config) []string {
	if c.IncludeDependabot {
		return []string{"ready", "dependabot", "review"}
	}
	return []string{"ready", "review"}
}
