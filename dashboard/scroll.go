package dashboard

const VisibleRows = 10

// Swipe tracks row-sized finger motion. A touch or small jitter never scrolls.
type Swipe struct {
	active bool
	anchor int
}

func (s *Swipe) Move(y int, pressed bool) int {
	if !pressed {
		s.active = false
		return 0
	}
	if !s.active {
		s.active = true
		s.anchor = y
		return 0
	}
	rows := (s.anchor - y) / 17
	s.anchor -= rows * 17
	return rows
}

// ScrollTop clamps the viewport and keeps knob selection visible.
func ScrollTop(top, selected, count int) int {
	if selected == AfterList {
		return ClampTop(count, count)
	}
	if selected < top {
		top = selected
	}
	if selected >= top+VisibleRows {
		top = selected - VisibleRows + 1
	}
	return ClampTop(top, count)
}
func ClampTop(top, count int) int {
	max := count - VisibleRows
	if max < 0 {
		max = 0
	}
	if top < 0 {
		return 0
	}
	if top > max {
		return max
	}
	return top
}

// Negative selections park outside the list and remain stable across refreshes.
const (
	BeforeList = -1
	AfterList  = -2
)

func MoveSelection(selected, delta, count int) int {
	if count == 0 {
		return BeforeList
	}
	if selected == AfterList {
		selected = count
	}
	selected += delta
	if selected < 0 {
		return BeforeList
	}
	if selected >= count {
		return AfterList
	}
	return selected
}
