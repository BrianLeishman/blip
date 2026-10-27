package dashboard

// Touch locks each gesture to one axis, so horizontal mode swipes don't scroll the list.
type Touch struct {
	active     bool
	x, y, axis int
	vertical   Swipe
}

// Move returns a horizontal direction (+1 right, -1 left) and vertical row delta.
func (t *Touch) Move(x, y int, pressed bool) (int, int) {
	if !pressed {
		*t = Touch{}
		return 0, 0
	}
	if !t.active {
		t.active, t.x, t.y = true, x, y
		t.vertical.Move(y, true)
		return 0, 0
	}
	dx, dy := x-t.x, y-t.y
	ax, ay := abs(dx), abs(dy)
	if t.axis == 0 {
		if ax >= 24 && ax > ay*2 {
			t.axis = 1
		}
		if ay >= 12 && ay > ax {
			t.axis = 2
		}
	}
	if t.axis == 1 && ax >= 60 {
		t.axis = 3 // One mode switch per finger gesture.
		if dx > 0 {
			return 1, 0
		}
		return -1, 0
	}
	if t.axis == 2 {
		return 0, t.vertical.Move(y, true)
	}
	return 0, 0
}
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
