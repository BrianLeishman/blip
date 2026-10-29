package display

import (
	"image/color"
	"strings"
	"testing"

	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"
)

func TestBandReuseClipsAndClearsFooter(t *testing.T) {
	var band Band
	background := color.RGBA{12, 17, 23, 255}
	ink := color.RGBA{255, 255, 255, 255}
	band.Clear(44, ink)
	band.Clear(16, background)
	tinyfont.WriteLine(&band, &proggy.TinySZ8pt7b, -12, 13, strings.Repeat("W", 100), ink)
	if len(band.Pixels()) != LCDWidth*16*2 {
		t.Fatal("footer uploaded header's leftover pixels")
	}
	// Attempts to draw outside the active band cannot alter its bitmap.
	before := append([]byte(nil), band.Pixels()...)
	for _, p := range [][2]int16{{-1, 0}, {320, 0}, {0, -1}, {0, 16}, {319, 43}} {
		band.SetPixel(p[0], p[1], ink)
	}
	for i, b := range band.Pixels() {
		if b != before[i] {
			t.Fatal("out-of-bounds glyph changed the band")
		}
	}
	band.Clear(16, background)
	for i := 0; i < len(band.Pixels()); i += 2 {
		if got := uint16(band.Pixels()[i])<<8 | uint16(band.Pixels()[i+1]); got != rgb565(background) {
			t.Fatal("old footer text survived reuse")
		}
	}
}

func TestBufferedChromeAndSelectionPreserveOtherPixels(t *testing.T) {
	lcd, model := testLCD()
	for i := range model.pixels {
		model.pixels[i] = 0x1234
	}
	var band Band
	var rows Renderer
	for frame := range 20 {
		// Header and footer reuse one buffer, just as the firmware does.
		for _, region := range [][2]int16{{0, 44}, {224, 16}} {
			band.Clear(region[1], testPalette.Background)
			tinyfont.WriteLine(&band, &proggy.TinySZ8pt7b, 10, 13, "Buffered title / footer", testPalette.Text)
			band.Line(8, region[1]-1, 304, testPalette.Text)
			if err := lcd.DrawRGBBitmap8(0, region[0], band.Pixels(), LCDWidth, region[1]); err != nil {
				t.Fatal(err)
			}
		}
		for slot := range 2 {
			row := Row{Title: "A row that changes selection", Selected: slot == frame%2}
			if err := lcd.DrawRGBBitmap8(6, int16(50+slot*17), rows.Render(row, testPalette), RowWidth, RowHeight); err != nil {
				t.Fatal(err)
			}
		}
		for y := range LCDHeight {
			for x := range LCDWidth {
				chrome := y < 44 || y >= 224
				row := x >= 6 && x < 6+RowWidth && ((y >= 50 && y < 66) || (y >= 67 && y < 83))
				if !chrome && !row && model.pixels[y*LCDWidth+x] != 0x1234 {
					t.Fatalf("chrome/selection write escaped into %d,%d", x, y)
				}
			}
		}
	}
}
