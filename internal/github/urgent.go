package github

import (
	"context"
	"fmt"
	"strconv"

	"github.com/BrianLeishman/blip/dashboard"
)

// ISSUE_ADVANCED is required for native field.priority search, independently
// of labels or project membership. Verify returned native values as well.
const urgentQuery = `query($search:String!,$cursor:String){
 search(query:$search,type:ISSUE_ADVANCED,first:100,after:$cursor){
  issueCount pageInfo{hasNextPage endCursor}
  nodes{...on Issue{
   number title state repository{nameWithOwner}
   issueFieldValues(first:100){pageInfo{hasNextPage} nodes{
    ...on IssueFieldSingleSelectValue{name field{...on IssueFieldSingleSelect{name}}}
   }}
  }}
 }
}`

type urgentNode struct {
	Number           int
	Title, State     string
	Repository       struct{ NameWithOwner string }
	IssueFieldValues struct {
		PageInfo struct{ HasNextPage bool }
		Nodes    []struct {
			Name  string
			Field struct{ Name string }
		}
	}
}

func (n urgentNode) urgent() bool {
	if n.State != "OPEN" {
		return false
	}
	for _, v := range n.IssueFieldValues.Nodes {
		if v.Field.Name == "Priority" && v.Name == "Urgent" {
			return true
		}
	}
	return false
}
func fetchUrgent(ctx context.Context, search string) ([]dashboard.Row, map[string]string, error) {
	var rows []dashboard.Row
	urls := map[string]string{}
	cursor := ""
	for {
		var response struct {
			Data struct {
				Search struct {
					IssueCount int
					PageInfo   struct {
						HasNextPage bool
						EndCursor   string
					}
					Nodes []urgentNode
				}
			}
			Errors []struct{ Message string }
		}
		args := []string{"api", "graphql", "-f", "query=" + urgentQuery, "-f", "search=" + search}
		if cursor != "" {
			args = append(args, "-f", "cursor="+cursor)
		}
		if err := gh(ctx, &response, args...); err != nil {
			return nil, nil, err
		}
		if len(response.Errors) > 0 {
			return nil, nil, fmt.Errorf("urgent issue query: %s", response.Errors[0].Message)
		}
		items := response.Data.Search
		if items.IssueCount > 1000 {
			return nil, nil, fmt.Errorf("urgent query exceeds GitHub's 1000-result search limit; narrow the scope")
		}
		for _, n := range items.Nodes {
			if n.IssueFieldValues.PageInfo.HasNextPage {
				return nil, nil, fmt.Errorf("issue %d exceeds 100 native fields; cannot verify priority", n.Number)
			}
			if !n.urgent() || !ValidRepository(n.Repository.NameWithOwner) {
				continue
			}
			id := n.Repository.NameWithOwner + "/issues/" + strconv.Itoa(n.Number)
			rows = append(rows, dashboard.Row{ID: id, Section: "urgent", Title: n.Title, Badge: "urgent"})
			urls[id] = "https://github.com/" + id
		}
		if !items.PageInfo.HasNextPage {
			break
		}
		if items.PageInfo.EndCursor == "" || items.PageInfo.EndCursor == cursor {
			return nil, nil, fmt.Errorf("urgent issue pagination did not advance")
		}
		cursor = items.PageInfo.EndCursor
	}
	return rows, urls, nil
}
