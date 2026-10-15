package main

import (
	"path/filepath"
	"testing"

	"github.com/BrianLeishman/blip/dashboard"
)

func TestCommentAcknowledgmentSurvivesRestartAndNewCommentsReturn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	state, err := loadAlertState(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	first := dashboard.Row{ID: "Org/widget/pull/7#issuecomment-1", Section: "comment"}
	later := dashboard.Row{ID: "Org/widget/pull/7#issuecomment-2", Section: "comment"}
	deploy := dashboard.Row{ID: "Org/widget/actions/runs/3", Section: "deployment"}
	if err := state.acknowledge(t.Context(), first.ID); err != nil {
		t.Fatal(err)
	}
	if err := state.acknowledge(t.Context(), deploy.ID); err != nil {
		t.Fatal(err)
	}
	state, err = loadAlertState(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	got := state.filter(dashboard.Snapshot{Rows: []dashboard.Row{first, later, deploy}})
	if len(got.Rows) != 2 || got.Rows[0] != later || got.Rows[1] != deploy {
		t.Fatalf("rows: %#v", got.Rows)
	}
}

func TestAcknowledgedCommentsDoNotConsumeDisplayLimit(t *testing.T) {
	state := &alertState{opened: map[string]bool{"old": true}}
	rows := make([]dashboard.Row, dashboard.MaxRows)
	for i := range rows {
		rows[i] = dashboard.Row{ID: "old", Section: "comment"}
	}
	rows = append(rows, dashboard.Row{ID: "Org/widget/pull/1", Section: "review"})
	got := state.filter(dashboard.Snapshot{Rows: rows})
	if len(got.Rows) != 1 || got.Rows[0].Section != "review" {
		t.Fatal("hidden comments displaced review work")
	}
}

func TestQueuedItemsDoNotProduceDuplicateCommentEntries(t *testing.T) {
	for _, section := range []string{"ready", "mine", "review", "urgent"} {
		t.Run(section, func(t *testing.T) {
			state := &alertState{opened: map[string]bool{}}
			parent := "Org/widget/pull/7"
			if section == "urgent" {
				parent = "Org/widget/issues/7"
			}
			row := dashboard.Row{ID: parent, Section: section}
			comment := dashboard.Row{ID: parent + "#issuecomment-8", Section: "comment"}
			followup := dashboard.Row{ID: "Org/widget/pull/9#discussion_r10", Section: "comment"}
			got := state.filter(dashboard.Snapshot{Rows: []dashboard.Row{comment, row, followup}})
			if len(got.Rows) != 2 || got.Rows[0] != row || got.Rows[1] != followup {
				t.Fatalf("rows %#v", got.Rows)
			}
			// Suppression isn't an acknowledgment: a follow-up may become relevant
			// when the main item leaves the queue.
			got = state.filter(dashboard.Snapshot{Rows: []dashboard.Row{comment}})
			if len(got.Rows) != 1 {
				t.Fatal("queue suppression permanently dismissed the comment")
			}
		})
	}
}

func TestSnoozeSurvivesRefreshAndRestartUntilSourceChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	state, err := loadAlertState(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	row := dashboard.Row{ID: "Org/widget/pull/7", Activity: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Section: "review", Title: "Example PR"}
	if err = state.snooze(t.Context(), row); err != nil {
		t.Fatal(err)
	}
	state, err = loadAlertState(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := dashboard.Snapshot{Version: 1, Revision: "new-poll", Rows: []dashboard.Row{row}}
	if err = state.refresh(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if len(state.filter(snapshot).Rows) != 0 {
		t.Fatal("poll/restart woke unchanged item")
	}
	snapshot.Rows = nil
	if err = state.refresh(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Rows = []dashboard.Row{row}
	if len(state.filter(snapshot).Rows) != 0 {
		t.Fatal("temporary absence woke item")
	}
	snapshot.Rows[0].Activity = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err = state.refresh(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if len(state.filter(snapshot).Rows) != 1 {
		t.Fatal("change did not wake item")
	}
	state, err = loadAlertState(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.Rows[0] = row
	if len(state.filter(snapshot).Rows) != 1 {
		t.Fatal("ended snooze resurrected when state reverted")
	}
}

func TestSnoozeUsesPressedActivityNotAnInflightNewerSnapshot(t *testing.T) {
	state, err := loadAlertState(t.Context(), filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dashboard.Row{ID: "Org/widget/pull/7", Activity: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Section: "review"}
	newer := old
	newer.Activity = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err = state.snooze(t.Context(), old); err != nil {
		t.Fatal(err)
	}
	snapshot := dashboard.Snapshot{Rows: []dashboard.Row{newer}}
	if err = state.refresh(t.Context(), snapshot); err != nil {
		t.Fatal(err)
	}
	if len(state.filter(snapshot).Rows) != 1 {
		t.Fatal("snoozed newer data the user never held")
	}
}

func TestSnoozedParentDoesNotReappearAsCommentOrConsumeDisplayLimit(t *testing.T) {
	state := &alertState{opened: map[string]bool{}, snoozed: map[string]string{"Org/widget/pull/7": "same"}}
	parent := dashboard.Row{ID: "Org/widget/pull/7", Activity: "same", Section: "review"}
	comment := dashboard.Row{ID: parent.ID + "#issuecomment-1", Activity: "comment", Section: "comment"}
	rows := []dashboard.Row{comment}
	for i := 0; i < dashboard.MaxRows; i++ {
		rows = append(rows, parent)
	}
	rows = append(rows, dashboard.Row{ID: "Org/widget/pull/8", Section: "review"})
	got := state.filter(dashboard.Snapshot{Rows: rows})
	if len(got.Rows) != 1 || got.Rows[0].ID != "Org/widget/pull/8" {
		t.Fatalf("snoozed work resurfaced/displaced other rows: %#v", got.Rows)
	}
}

func TestSnoozeTokenValidation(t *testing.T) {
	for _, s := range []string{"", "abc", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		if validActivity(s) {
			t.Fatalf("accepted invalid token %q", s)
		}
	}
	if !validActivity("0123456789abcdef0123456789abcdef") {
		t.Fatal("rejected source token")
	}
}
