// Package dashboard is the small shared wire model used by the Mac and device.
package dashboard

const MaxRows = 60

type Row struct {
	Detail  string `json:"detail,omitempty"`
	ID      string `json:"id"`
	Section string `json:"section"`
	Title   string `json:"title"`
	Badge   string `json:"badge,omitempty"`
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
