// Package dashboard is the small shared wire model used by the Mac and device.
package dashboard

import "strings"

const MaxRows = 60

type Row struct {
	ChecksRunning bool   `json:"checksRunning,omitempty"`
	Detail        string `json:"detail,omitempty"`
	ID            string `json:"id"`
	Section       string `json:"section"`
	Title         string `json:"title"`
	Badge         string `json:"badge,omitempty"`
}
type Snapshot struct {
	Version  int    `json:"version"`
	Revision string `json:"revision"`
	Status   string `json:"status"`
	Rows     []Row  `json:"rows"`
}
type Event struct {
	Kind     string `json:"kind"`
	Revision string `json:"revision,omitempty"`
	ID       string `json:"id,omitempty"`
	Position int32  `json:"position,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Selected preserves the highlighted item when a refreshed list reorders.
func Selected(rows []Row, id string) int {
	for i, r := range rows {
		if r.ID == id {
			return i
		}
	}
	return 0
}

// SameContent ignores refresh revisions while comparing dashboard content and order.
func (s Snapshot) SameContent(next Snapshot) bool {
	if s.Version != next.Version || s.Status != next.Status || len(s.Rows) != len(next.Rows) {
		return false
	}
	for i, row := range s.Rows {
		if row != next.Rows[i] {
			return false
		}
	}
	return true
}

// Identity keeps the item number first so long repository names cannot hide it.
func (r Row) Identity() string {
	parts := strings.Split(r.ID, "/")
	if len(parts) == 5 && parts[2] == "actions" && parts[3] == "runs" {
		return "run " + parts[4] + " " + parts[0] + "/" + parts[1]
	}
	if len(parts) != 4 {
		return ""
	}
	number, _, _ := strings.Cut(parts[3], "#")
	return "#" + number + " " + parts[0] + "/" + parts[1]
}
