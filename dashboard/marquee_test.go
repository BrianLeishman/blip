package dashboard

import (
	"strings"
	"testing"
	"time"
)

func TestMarqueePausesAndRefreshes(t *testing.T) {
	start := time.Unix(1, 0)
	var m Marquee
	title := strings.Repeat("a", TitleColumns) + "bc"
	m.Set("one", title, start)
	if m.Advance(start.Add(time.Second)) {
		t.Fatal("scrolled before initial reading pause")
	}
	if !m.Advance(start.Add(1200*time.Millisecond)) || !strings.HasSuffix(m.Text(), "b") {
		t.Fatal("did not reveal next character")
	}
	// A redraw or unchanged refresh must not restart the reading delay.
	m.Set("one", title, start.Add(1300*time.Millisecond))
	if !m.Advance(start.Add(1400*time.Millisecond)) || !strings.HasSuffix(m.Text(), "bc") {
		t.Fatal("refresh interrupted scrolling")
	}
	if m.Advance(start.Add(2400 * time.Millisecond)) {
		t.Fatal("did not pause at end")
	}
	if !m.Advance(start.Add(2600*time.Millisecond)) || m.Text() != title[:TitleColumns] {
		t.Fatal("did not return to start")
	}
	m.Set("two", title, start.Add(3*time.Second))
	if m.Advance(start.Add(4 * time.Second)) {
		t.Fatal("new selection did not get its reading pause")
	}
	m.Set("", "", start.Add(5*time.Second))
	if m.Advance(start.Add(time.Hour)) || m.Text() != "" {
		t.Fatal("deselected title still animates")
	}
}

func TestShortMarqueeStaysStill(t *testing.T) {
	var m Marquee
	start := time.Unix(1, 0)
	m.Set("short", "A short title", start)
	if m.Advance(start.Add(time.Hour)) || m.Text() != "A short title" {
		t.Fatal("short title changed")
	}
}
