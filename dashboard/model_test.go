package dashboard

import "testing"

func TestSelectionSurvivesReordering(t *testing.T) {
	rows := []Row{{ID: "b"}, {ID: "a"}}
	if Selected(rows, "a") != 1 {
		t.Fatal("lost selection")
	}
	if Selected(rows, "gone") != 0 {
		t.Fatal("bad fallback")
	}
}

func TestSnapshotContentChanges(t *testing.T) {
	base := Snapshot{Version: 1, Revision: "first", Status: "GitHub connected", Rows: []Row{
		{ID: "a", Section: "mine", Title: "First PR", Badge: "0/2", Detail: "CI pending"},
		{ID: "b", Section: "review", Title: "Second PR"},
	}}
	for _, tc := range []struct {
		name   string
		change func(*Snapshot)
		same   bool
	}{
		{"refresh only", func(s *Snapshot) { s.Revision = "second" }, true},
		{"approval", func(s *Snapshot) { s.Rows[0].Badge = "1/2" }, false},
		{"CI", func(s *Snapshot) { s.Rows[0].Detail = "CI passed" }, false},
		{"CI starts", func(s *Snapshot) { s.Rows[0].ChecksRunning = true }, false},
		{"title", func(s *Snapshot) { s.Rows[0].Title = "Renamed" }, false},
		{"section", func(s *Snapshot) { s.Rows[0].Section = "ready" }, false},
		{"identity", func(s *Snapshot) { s.Rows[0].ID = "c" }, false},
		{"removed", func(s *Snapshot) { s.Rows = s.Rows[:1] }, false},
		{"order", func(s *Snapshot) { s.Rows[0], s.Rows[1] = s.Rows[1], s.Rows[0] }, false},
		{"status", func(s *Snapshot) { s.Status = "Showing 60 of 70" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := base
			next.Rows = append([]Row(nil), base.Rows...)
			tc.change(&next)
			if got := base.SameContent(next); got != tc.same {
				t.Fatalf("SameContent = %v; want %v", got, tc.same)
			}
		})
	}
}

func TestIdentity(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{"Org/repo/pull/123", "#123 Org/repo"},
		{"Org/repo/issues/456", "#456 Org/repo"},
		{"Org/repo/pull/123#discussion_r99", "#123 Org/repo"},
		{"Org/repo/actions/runs/12345", "run 12345 Org/repo"},
		{"", ""},
	} {
		if got := (Row{ID: tc.id}).Identity(); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}
