package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BrianLeishman/blip/dashboard"
)

// Local acknowledgments and snoozes are independent of GitHub's inbox.
// Filtering happens on the serial loop so an in-flight fetch cannot undo an action.
type alertState struct {
	path       string
	opened     map[string]bool
	snoozePath string
	snoozed    map[string]string
}

func loadAlertState(ctx context.Context, configPath string) (*alertState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := strings.TrimSuffix(configPath, ".json") + ".alerts.local.json"
	s := &alertState{path: path, opened: map[string]bool{}, snoozePath: strings.TrimSuffix(configPath, ".json") + ".snoozes.local.json", snoozed: map[string]string{}}
	if b, err := os.ReadFile(s.snoozePath); err == nil {
		if err = json.Unmarshal(b, &s.snoozed); err != nil {
			return nil, err
		}
		if s.snoozed == nil {
			s.snoozed = map[string]string{}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &s.opened); err != nil {
		return nil, err
	}
	if s.opened == nil {
		s.opened = map[string]bool{}
	}
	return s, nil
}

func (s *alertState) acknowledge(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	next := make(map[string]bool, len(s.opened)+1)
	for k, v := range s.opened {
		next[k] = v
	}
	next[id] = true
	if err := writeLocalState(ctx, s.path, next); err != nil {
		return err
	}
	s.opened = next
	return nil
}

func writeLocalState(ctx context.Context, path string, value any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".blip-alerts-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	return nil
}

func (s *alertState) snooze(ctx context.Context, row dashboard.Row) error {
	next := make(map[string]string, len(s.snoozed)+1)
	for id, activity := range s.snoozed {
		next[id] = activity
	}
	next[row.ID] = row.Activity
	if err := writeLocalState(ctx, s.snoozePath, next); err != nil {
		return err
	}
	s.snoozed = next
	return nil
}

// A snooze ends permanently at the first observed activity change, even if the
// item later returns to its previous status. Disappearance alone isn't a change.
func (s *alertState) refresh(ctx context.Context, snapshot dashboard.Snapshot) error {
	next := make(map[string]string, len(s.snoozed))
	for id, activity := range s.snoozed {
		next[id] = activity
	}
	for _, r := range snapshot.Rows {
		if activity, ok := next[r.ID]; ok && r.Activity != "" && r.Activity != activity {
			delete(next, r.ID)
		}
	}
	if len(next) == len(s.snoozed) {
		return nil
	}
	if err := writeLocalState(ctx, s.snoozePath, next); err != nil {
		return err
	}
	s.snoozed = next
	return nil
}

func (s *alertState) filter(snapshot dashboard.Snapshot) dashboard.Snapshot {
	rows := make([]dashboard.Row, 0, len(snapshot.Rows))
	queued := map[string]bool{}
	for _, r := range snapshot.Rows {
		if r.Section != "comment" {
			queued[r.ID] = true
		}
	}
	for _, r := range snapshot.Rows {
		if activity, ok := s.snoozed[r.ID]; ok && activity == r.Activity {
			continue
		}
		if r.Section == "comment" {
			parent, _, _ := strings.Cut(r.ID, "#")
			if s.opened[r.ID] || queued[parent] {
				continue
			}
		}
		rows = append(rows, r)
	}
	snapshot.Rows = rows
	if len(rows) > dashboard.MaxRows {
		snapshot.Status = fmt.Sprintf("Showing %d of %d", dashboard.MaxRows, len(rows))
		snapshot.Rows = rows[:dashboard.MaxRows]
	}
	return snapshot
}

func validActivity(s string) bool {
	if len(s) != 32 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}
