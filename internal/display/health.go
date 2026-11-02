package display

import "time"

// Health arms recovery only after a real, healthy read from the ILI9341. An
// unconnected MISO line must never cause periodic resets on eight-wire builds.
type Health struct {
	armed        bool
	failures     int
	lastRecovery time.Time
}

func (h *Health) Observe(status Status, now time.Time) bool {
	if status.Healthy() {
		h.armed = true
		h.failures = 0
		return false
	}
	if !h.armed {
		return false
	}
	h.failures++
	if h.failures < 2 || (!h.lastRecovery.IsZero() && now.Sub(h.lastRecovery) < time.Minute) {
		return false
	}
	h.failures = 0
	h.lastRecovery = now
	return true
}
