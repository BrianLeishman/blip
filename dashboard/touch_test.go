package dashboard

import "testing"

func TestTouchAxesAndOneModeSwitchPerGesture(t *testing.T) {
	var touch Touch
	touch.Move(100, 100, true)
	if x, y := touch.Move(125, 103, true); x != 0 || y != 0 {
		t.Fatal("switched before threshold")
	}
	if x, y := touch.Move(165, 105, true); x != 1 || y != 0 {
		t.Fatal("right swipe did not switch")
	}
	if x, y := touch.Move(20, 110, true); x != 0 || y != 0 {
		t.Fatal("switched twice in one gesture")
	}
	touch.Move(0, 0, false)
	touch.Move(200, 100, true)
	if x, y := touch.Move(130, 95, true); x != -1 || y != 0 {
		t.Fatal("left swipe did not switch")
	}
	touch.Move(0, 0, false)
	touch.Move(100, 100, true)
	if x, y := touch.Move(103, 65, true); x != 0 || y != 2 {
		t.Fatalf("vertical scrolling changed: %d %d", x, y)
	}
	if x, _ := touch.Move(200, 40, true); x != 0 {
		t.Fatal("vertical gesture changed modes")
	}
	touch.Move(0, 0, false)
	touch.Move(100, 100, true)
	if x, y := touch.Move(105, 105, true); x != 0 || y != 0 {
		t.Fatal("touch jitter moved anything")
	}
}
