package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BrianLeishman/blip/dashboard"
)

func TestSnapshotFitsDeviceBufferWithLongCommentTitles(t *testing.T) {
	s := dashboard.Snapshot{Version: 1, Revision: "test", Rows: make([]dashboard.Row, dashboard.MaxRows)}
	for i := range s.Rows {
		s.Rows[i] = dashboard.Row{ID: "ExampleOrg/widget/pull/123456#issuecomment-1234567890", Section: "comment", Title: strings.Repeat("界", 150), Detail: "Comment from ExampleReviewer", Badge: "NEW"}
	}
	bounded, wire, err := snapshotWire(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) > 16384 || len(bounded.Rows) == 0 || len(bounded.Rows) >= len(s.Rows) {
		t.Fatalf("rows %d bytes %d", len(bounded.Rows), len(wire))
	}
	var decoded dashboard.Snapshot
	if err = json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if len([]rune(decoded.Rows[0].Title)) != 100 {
		t.Fatal("title limit not applied")
	}
	if len([]rune(s.Rows[0].Title)) != 150 {
		t.Fatal("source snapshot was mutated")
	}
}
