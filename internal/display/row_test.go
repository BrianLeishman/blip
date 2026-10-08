package display

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
)

var testPalette = Palette{Background: color.RGBA{0, 0, 0, 255}, Highlight: color.RGBA{0, 0, 255, 255}, Text: color.RGBA{255, 255, 255, 255}, Selection: color.RGBA{255, 255, 255, 255}}

func at(pixels []byte, x, y int) uint16 {
	i := (y*RowWidth + x) * 2
	return uint16(pixels[i])<<8 | uint16(pixels[i+1])
}

func TestRowClipsTextToIndependentFields(t *testing.T) {
	var r Renderer
	frame := Row{Selected: true, Marker: "M", MarkerColor: color.RGBA{0, 255, 0, 255}, Title: strings.Repeat("W", 150), Badge: strings.Repeat("FAIL", 20), BadgeColor: color.RGBA{255, 0, 0, 255}, Spinner: "/", SpinnerColor: color.RGBA{255, 255, 0, 255}}
	pixels := r.Render(frame, testPalette)
	if len(pixels) != RowWidth*RowHeight*2 {
		t.Fatal("incorrect row payload size")
	}
	whiteTitle := 0
	redBadge := 0
	for y := 0; y < RowHeight; y++ {
		for x := 0; x < RowWidth; x++ {
			switch at(pixels, x, y) {
			case 0xffff:
				if x >= 3 && (x < TitleLeft || x >= TitleRight) {
					t.Fatalf("title/selection escaped into x=%d y=%d", x, y)
				}
				if x >= TitleLeft && x < TitleRight {
					whiteTitle++
				}
			case 0xf800:
				if x < BadgeLeft || x >= BadgeRight {
					t.Fatalf("badge escaped into x=%d y=%d", x, y)
				}
				redBadge++
			case 0xffe0:
				if x < SpinnerLeft || x >= SpinnerRight {
					t.Fatalf("spinner escaped into x=%d y=%d", x, y)
				}
			}
		}
	}
	if whiteTitle == 0 || whiteTitle >= (TitleRight-TitleLeft)*RowHeight/3 {
		t.Fatalf("title isn't a sparse glyph mask: %d white pixels", whiteTitle)
	}
	if redBadge == 0 {
		t.Fatal("badge wasn't rendered")
	}
}

func TestRowReplacementClearsOldGlyphsAndHighlight(t *testing.T) {
	var r Renderer
	r.Render(Row{Selected: true, Marker: "M", Title: "A selected long title", Badge: "FAIL", BadgeColor: color.RGBA{255, 0, 0, 255}, Spinner: "/", SpinnerColor: color.RGBA{255, 255, 0, 255}}, testPalette)
	pixels := r.Render(Row{}, testPalette)
	for i, b := range pixels {
		if b != 0 {
			t.Fatalf("old row pixels survived at byte %d", i)
		}
	}
}

func TestMarqueeAndSpinnerFramesPreserveBadgeAndAdjacentRow(t *testing.T) {
	var r Renderer
	frame := Row{Selected: true, Title: strings.Repeat("A", 37), Badge: "FAIL", BadgeColor: color.RGBA{255, 0, 0, 255}, Spinner: "|", SpinnerColor: color.RGBA{255, 255, 0, 255}}
	first := append([]byte(nil), r.Render(frame, testPalette)...)
	screen := make([]byte, 2*len(first))
	copy(screen, first)
	copy(screen[len(first):], r.Render(Row{Title: "Neighbor row", Badge: "0/2", BadgeColor: color.RGBA{0, 255, 0, 255}}, testPalette))
	neighbor := append([]byte(nil), screen[len(first):]...)
	for tick := 0; tick < 40; tick++ {
		frame.Title = strings.Repeat(string(rune('A'+tick%26)), 37)
		frame.Spinner = string("|/-\\"[tick%4])
		pixels := r.Render(frame, testPalette)
		copy(screen, pixels)
		if !bytes.Equal(screen[len(first):], neighbor) {
			t.Fatal("animated row changed its neighbor")
		}
		for y := 0; y < RowHeight; y++ {
			for x := BadgeLeft; x < BadgeRight; x++ {
				if at(pixels, x, y) != at(first, x, y) {
					t.Fatalf("title/spinner altered badge at %d,%d", x, y)
				}
			}
		}
	}
}
