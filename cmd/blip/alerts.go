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

// This state acknowledges only comment IDs, independently of GitHub's inbox.
// Filtering happens on the serial loop so an in-flight fetch cannot undo a click.
type alertState struct {
	path   string
	opened map[string]bool
}

func loadAlertState(ctx context.Context, configPath string) (*alertState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := strings.TrimSuffix(configPath, ".json") + ".alerts.local.json"
	s := &alertState{path: path, opened: map[string]bool{}}
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
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".blip-alerts-*")
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
	if err = os.Rename(f.Name(), s.path); err != nil {
		return err
	}
	s.opened = next
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
