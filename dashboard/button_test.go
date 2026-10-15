package dashboard

import (
	"testing"
	"time"
)

func TestShortClickOpensOnlyOnRelease(t *testing.T) {
	var b Button
	start := time.Unix(10, 0)
	row := Row{ID: "Org/widget/pull/7", Activity: "activity"}
	for _, ms := range []int{0, 40, 100} {
		if e := b.Update(start.Add(time.Duration(ms)*time.Millisecond), true, row); e.Kind != "" {
			t.Fatalf("opened before release: %#v", e)
		}
	}
	if e := b.Update(start.Add(110*time.Millisecond), false, row); e.Kind != "" {
		t.Fatal("raw release wasn't debounced")
	}
	e := b.Update(start.Add(150*time.Millisecond), false, row)
	if e.Kind != "open" || e.ID != row.ID {
		t.Fatalf("release: %#v", e)
	}
	if e = b.Update(start.Add(200*time.Millisecond), false, row); e.Kind != "" {
		t.Fatal("repeated click")
	}
}

func TestLongPressSnoozesOriginalActivityExactlyOnce(t *testing.T) {
	var b Button
	start := time.Unix(10, 0)
	row := Row{ID: "Org/widget/pull/7", Activity: "original"}
	b.Update(start, true, row)
	b.Update(start.Add(40*time.Millisecond), true, row)
	// A refresh or selection movement must not change the pressed target.
	other := Row{ID: "Org/widget/pull/8", Activity: "new"}
	e := b.Update(start.Add(1040*time.Millisecond), true, other)
	if e.Kind != "snooze" || e.ID != row.ID || e.Activity != row.Activity {
		t.Fatalf("hold: %#v", e)
	}
	for _, step := range []struct {
		ms      int
		pressed bool
	}{{1800, true}, {1900, false}, {1940, false}} {
		if e = b.Update(start.Add(time.Duration(step.ms)*time.Millisecond), step.pressed, other); e.Kind != "" {
			t.Fatalf("hold repeated/opened: %#v", e)
		}
	}
}

func TestLongReleaseAfterBusyLoopNeverOpens(t *testing.T) {
	var b Button
	start := time.Unix(10, 0)
	row := Row{ID: "Org/widget/pull/7", Activity: "original"}
	b.Update(start, true, row)
	b.Update(start.Add(40*time.Millisecond), true, row)
	b.Update(start.Add(1500*time.Millisecond), false, row)
	if e := b.Update(start.Add(1540*time.Millisecond), false, row); e.Kind != "snooze" {
		t.Fatalf("late release: %#v", e)
	}
}

func TestButtonBounceAndCancelledGestureDoNotOpen(t *testing.T) {
	var b Button
	start := time.Unix(10, 0)
	row := Row{ID: "Org/widget/pull/7", Activity: "original"}
	b.Update(start, true, row)
	b.Update(start.Add(10*time.Millisecond), false, row)
	if e := b.Update(start.Add(50*time.Millisecond), false, row); e.Kind != "" {
		t.Fatal("bounce produced an action")
	}
	b.Update(start.Add(100*time.Millisecond), true, row)
	b.Update(start.Add(140*time.Millisecond), true, row)
	b.Cancel()
	for _, step := range []struct {
		ms      int
		pressed bool
	}{{1400, true}, {1500, false}, {1540, false}} {
		if e := b.Update(start.Add(time.Duration(step.ms)*time.Millisecond), step.pressed, row); e.Kind != "" {
			t.Fatalf("cancelled gesture: %#v", e)
		}
	}
}

func TestLongPressWithoutActivityDoesNotOpen(t *testing.T) {
	for _, row := range []Row{{}, {ID: "Org/widget/pull/7"}} {
		var b Button
		start := time.Unix(10, 0)
		b.Update(start, true, row)
		b.Update(start.Add(40*time.Millisecond), true, row)
		for _, step := range []struct {
			ms      int
			pressed bool
		}{{1040, true}, {1500, false}, {1540, false}} {
			if e := b.Update(start.Add(time.Duration(step.ms)*time.Millisecond), step.pressed, row); e.Kind != "" {
				t.Fatalf("unsupported hold opened: %#v", e)
			}
		}
	}
}
