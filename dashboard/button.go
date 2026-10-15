package dashboard

import "time"

const LongPressDuration = time.Second
const buttonDebounce = 30 * time.Millisecond

// Button binds both gestures to the item under the knob when the press is accepted.
// Moving the knob cancels the gesture; a hold never also produces an open.
type Button struct {
	raw, stable, consumed bool
	changed, started      time.Time
	target                Row
}

func (b *Button) Cancel() { b.consumed = true }

func (b *Button) Update(now time.Time, pressed bool, target Row) Event {
	if pressed != b.raw {
		b.raw = pressed
		b.changed = now
	}
	if pressed != b.stable && now.Sub(b.changed) >= buttonDebounce {
		b.stable = pressed
		if pressed {
			b.started = now
			b.target = target
			b.consumed = false
		} else if !b.consumed {
			b.consumed = true
			if now.Sub(b.started) >= LongPressDuration {
				if b.target.ID != "" && b.target.Activity != "" {
					return Event{Kind: "snooze", ID: b.target.ID, Activity: b.target.Activity}
				}
				return Event{}
			}
			if b.target.ID != "" {
				return Event{Kind: "open", ID: b.target.ID}
			}
			return Event{Kind: "click"}
		}
	}
	if b.stable && b.raw && !b.consumed && now.Sub(b.started) >= LongPressDuration {
		b.consumed = true
		if b.target.ID != "" && b.target.Activity != "" {
			return Event{Kind: "snooze", ID: b.target.ID, Activity: b.target.Activity}
		}
	}
	return Event{}
}
