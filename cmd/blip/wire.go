package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/BrianLeishman/blip/dashboard"
)

// Bound actual JSON bytes, including longer comment anchors and Unicode titles.
func snapshotWire(s dashboard.Snapshot) (dashboard.Snapshot, []byte, error) {
	s.Rows = append([]dashboard.Row(nil), s.Rows...)
	for i := range s.Rows {
		title := []rune(s.Rows[i].Title)
		if len(title) > 100 {
			s.Rows[i].Title = string(title[:100])
		}
	}
	for {
		b, err := json.Marshal(s)
		if err != nil {
			return s, nil, err
		}
		if len(b) < 16384 {
			return s, append(b, '\n'), nil
		}
		if len(s.Rows) == 0 {
			return s, nil, errors.New("snapshot metadata exceeds device buffer")
		}
		s.Rows = s.Rows[:len(s.Rows)-1]
		s.Status = fmt.Sprintf("Showing %d rows (USB limit)", len(s.Rows))
	}
}
