package dashboard

import "time"

const TitleColumns = 37

// Marquee scrolls already-normalized ASCII, pausing at both ends of a title.
type Marquee struct {
	id, title string
	offset    int
	next      time.Time
}

func (m *Marquee) Set(id, title string, now time.Time) {
	if m.id == id && m.title == title {
		return
	}
	m.id, m.title, m.offset = id, title, 0
	m.next = now.Add(1200 * time.Millisecond)
}

func (m *Marquee) Text() string {
	end := m.offset + TitleColumns
	if end > len(m.title) {
		end = len(m.title)
	}
	return m.title[m.offset:end]
}

func (m *Marquee) Advance(now time.Time) bool {
	last := len(m.title) - TitleColumns
	if last <= 0 || now.Before(m.next) {
		return false
	}
	if m.offset == last {
		m.offset = 0
	} else {
		m.offset++
	}
	m.next = now.Add(200 * time.Millisecond)
	if m.offset == 0 || m.offset == last {
		m.next = now.Add(1200 * time.Millisecond)
	}
	return true
}
