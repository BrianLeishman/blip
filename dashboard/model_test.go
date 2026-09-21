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
