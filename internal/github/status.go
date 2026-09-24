package github

import (
	"fmt"
	"strings"
)

type Check struct{ Status, Conclusion, State string }

// ChecksRunning is independent of failure: other jobs may still be running.
func ChecksRunning(checks []Check) bool {
	for _, c := range checks {
		if c.State == "PENDING" || c.Status != "" && c.Status != "COMPLETED" {
			return true
		}
	}
	return false
}

func Checks(checks []Check) string {
	if len(checks) == 0 {
		return "?"
	}
	for _, c := range checks {
		switch c.Conclusion {
		case "FAILURE", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "STARTUP_FAILURE":
			return "!"
		}
		if c.State == "FAILURE" || c.State == "ERROR" {
			return "!"
		}
	}
	if ChecksRunning(checks) {
		return "~"
	}
	return "+"
}

// OwnStatus describes the user's personal merge queue, not GitHub permission to merge.
func OwnStatus(item Item, approvals, required int) (section, badge, detail string) {
	section = "mine"
	ci := Checks(item.StatusCheckRollup)
	if ci != "!" && item.MergeStateStatus != "DIRTY" && item.ReviewDecision != "CHANGES_REQUESTED" && (approvals >= required || item.ReviewDecision == "APPROVED" || len(item.ReviewRequests) == 0) {
		section = "ready"
	}
	lgtm := "-"
	for _, l := range item.Labels {
		if strings.EqualFold(l.Name, "LGTM") {
			lgtm = "+"
		}
	}
	badge = fmt.Sprintf("%d/%d L%s %s", approvals, required, lgtm, ci)
	if ci == "!" {
		badge = fmt.Sprintf("%d/%d FAIL", approvals, required)
	}
	waiting := fmt.Sprintf("%d requested", len(item.ReviewRequests))
	if len(item.ReviewRequests) == 0 {
		waiting = "no requests"
	}
	if item.ReviewDecision == "CHANGES_REQUESTED" {
		waiting = "changes requested"
	}
	if item.MergeStateStatus == "DIRTY" {
		badge = "CONFLICT"
		waiting = "merge conflict"
	}
	ciText := map[string]string{"!": "FAIL", "~": "WAIT", "+": "OK", "?": "?"}[ci]
	detail = fmt.Sprintf("#%d LGTM%s CI:%s / %s", item.Number, lgtm, ciText, waiting)
	if item.MergeStateStatus == "DIRTY" {
		detail = fmt.Sprintf("#%d %d/%d LGTM%s CI:%s / merge conflict", item.Number, approvals, required, lgtm, ciText)
	}
	return
}
