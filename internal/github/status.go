package github

import (
	"fmt"
	"strings"
)

type Check struct{ Status, Conclusion, State string }

func Checks(checks []Check) string {
	if len(checks) == 0 {
		return "?"
	}
	pending := false
	for _, c := range checks {
		switch c.Conclusion {
		case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
			return "!"
		}
		if c.State == "FAILURE" || c.State == "ERROR" {
			return "!"
		}
		if c.State == "PENDING" || c.Status != "" && c.Status != "COMPLETED" {
			pending = true
		}
	}
	if pending {
		return "~"
	}
	return "+"
}

// OwnStatus describes the user's personal merge queue, not GitHub permission to merge.
func OwnStatus(item Item, approvals, required int) (section, badge, detail string) {
	section = "mine"
	if item.ReviewDecision != "CHANGES_REQUESTED" && (approvals >= required || item.ReviewDecision == "APPROVED" || len(item.ReviewRequests) == 0) {
		section = "ready"
	}
	lgtm := "-"
	for _, l := range item.Labels {
		if strings.EqualFold(l.Name, "LGTM") {
			lgtm = "+"
		}
	}
	ci := Checks(item.StatusCheckRollup)
	badge = fmt.Sprintf("%d/%d L%s %s", approvals, required, lgtm, ci)
	waiting := fmt.Sprintf("%d requested", len(item.ReviewRequests))
	if len(item.ReviewRequests) == 0 {
		waiting = "no requests"
	}
	if item.ReviewDecision == "CHANGES_REQUESTED" {
		waiting = "changes requested"
	}
	if item.MergeStateStatus == "DIRTY" {
		waiting = "merge conflict"
	}
	ciText := map[string]string{"!": "FAIL", "~": "WAIT", "+": "OK", "?": "?"}[ci]
	detail = fmt.Sprintf("#%d LGTM%s CI:%s / %s", item.Number, lgtm, ciText, waiting)
	return
}
