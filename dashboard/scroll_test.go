package dashboard

import "testing"

func TestSwipe(t *testing.T) {
	var s Swipe
	for _, tc := range []struct {
		y       int
		pressed bool
		want    int
	}{
		{150, true, 0}, {148, true, 0}, {130, true, 1}, {96, true, 2},
		{150, true, -3}, {0, false, 0}, {40, true, 0}, {75, true, -2},
	} {
		if got := s.Move(tc.y, tc.pressed); got != tc.want {
			t.Fatalf("%+v got %d", tc, got)
		}
	}
}
func TestViewport(t *testing.T) {
	for _, tc := range []struct{ top, selected, count, want int }{
		{0, 9, 20, 0}, {0, 10, 20, 1}, {8, 5, 20, 5}, {15, 19, 20, 10}, {2, 3, 5, 0}, {0, 0, 0, 0},
	} {
		if got := ScrollTop(tc.top, tc.selected, tc.count); got != tc.want {
			t.Fatalf("%+v got %d", tc, got)
		}
	}
}

func TestSelectionCanParkOutsideList(t *testing.T) {
	for _, tc := range []struct{ selected, delta, count, want int }{
		{0, -1, 6, BeforeList}, {BeforeList, -10, 6, BeforeList},
		{BeforeList, 1, 6, 0}, {5, 1, 6, AfterList},
		{AfterList, 10, 6, AfterList}, {AfterList, -1, 6, 5},
		{AfterList, -1, 3, 2}, {0, -1, 0, BeforeList},
	} {
		if got := MoveSelection(tc.selected, tc.delta, tc.count); got != tc.want {
			t.Fatalf("%+v got %d", tc, got)
		}
	}
	if ScrollTop(5, BeforeList, 20) != 0 || ScrollTop(5, AfterList, 20) != 10 {
		t.Fatal("parking should leave the nearest end of the list visible")
	}
}
