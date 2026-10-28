package display

import (
	"bytes"
	"errors"
	"image/color"
	"testing"
)

type testPin struct{ high bool }

func (p *testPin) High() { p.high = true }
func (p *testPin) Low()  { p.high = false }

// Model the ILI9341's command/data distinction and address counters. The transport
// must not write outside the most recently specified rectangle or change DC/CS
// while a write-only DMA operation could still be in flight.
type lcdModel struct {
	dc, cs         testPin
	command        byte
	x0, x1, y0, y1 int
	cursor         int
	highByte       byte
	pixels         [LCDWidth * LCDHeight]uint16
	commands       []byte
	written, calls int
	failAt         int
	failure        error
}

func (m *lcdModel) Tx(w, r []byte) error {
	m.calls++
	if m.failAt > 0 && m.calls == m.failAt {
		return m.failure
	}
	if m.cs.high {
		return errors.New("SPI write while LCD deselected")
	}
	if r == nil || len(r) != len(w) {
		return errors.New("LCD used asynchronous send-only DMA path")
	}
	if len(w) > 64 {
		return errors.New("unbounded SPI chunk")
	}
	if !m.dc.high {
		if len(w) != 1 {
			return errors.New("command must be one byte")
		}
		m.command = w[0]
		m.commands = append(m.commands, w[0])
		if w[0] == 0x2c {
			m.cursor = 0
			m.written = 0
		}
		return nil
	}
	switch m.command {
	case 0x2a, 0x2b:
		if len(w) != 4 {
			return errors.New("incomplete address range")
		}
		a, b := int(w[0])<<8|int(w[1]), int(w[2])<<8|int(w[3])
		if m.command == 0x2a {
			m.x0, m.x1 = a, b
		} else {
			m.y0, m.y1 = a, b
		}
	case 0x2c:
		width, height := m.x1-m.x0+1, m.y1-m.y0+1
		if width <= 0 || height <= 0 || m.x0 < 0 || m.x1 >= LCDWidth || m.y0 < 0 || m.y1 >= LCDHeight {
			return errors.New("invalid panel address window")
		}
		for _, b := range w {
			if m.cursor >= width*height*2 {
				return errors.New("pixel data overran target rectangle")
			}
			if m.cursor%2 == 0 {
				m.highByte = b
			} else {
				p := m.cursor / 2
				x, y := m.x0+p%width, m.y0+p/width
				m.pixels[y*LCDWidth+x] = uint16(m.highByte)<<8 | uint16(b)
				m.written++
			}
			m.cursor++
		}
	default:
		return errors.New("data without a supported command")
	}
	return nil
}
func (m *lcdModel) Transfer(b byte) (byte, error) { return b, nil }
func testLCD() (*LCD, *lcdModel) {
	m := &lcdModel{}
	return NewLCD(NewBlockingSPI(m), &m.dc, &m.cs), m
}

func TestRepeatedRowWritesResetLostWindowAndPreserveScreen(t *testing.T) {
	lcd, m := testLCD()
	for i := range m.pixels {
		m.pixels[i] = 0x1234
	}
	var renderer Renderer
	for tick := range 80 {
		// Emulate address state being lost between identical animated row writes.
		m.x0, m.x1, m.y0, m.y1 = 0, LCDWidth-1, 0, LCDHeight-1
		m.commands = nil
		frame := Row{Title: "Long selected title scrolling past the viewport", Selected: true, Badge: "FAIL", BadgeColor: color.RGBA{255, 0, 0, 255}, Spinner: string("|/-\\"[tick%4])}
		pixels := renderer.Render(frame, testPalette)
		if err := lcd.DrawRGBBitmap8(6, 67, pixels, RowWidth, RowHeight); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(m.commands, []byte{0x2a, 0x2b, 0x2c}) {
			t.Fatalf("relied on a cached window: %x", m.commands)
		}
		if !m.cs.high || m.written != RowWidth*RowHeight {
			t.Fatal("incomplete LCD transaction")
		}
		for y := range LCDHeight {
			for x := range LCDWidth {
				want := uint16(0x1234)
				if x >= 6 && x < 6+RowWidth && y >= 67 && y < 67+RowHeight {
					want = at(pixels, x-6, y-67)
				}
				if got := m.pixels[y*LCDWidth+x]; got != want {
					t.Fatalf("corrupted screen at %d,%d: %04x != %04x", x, y, got, want)
				}
			}
		}
	}
}

func TestSolidFillsSendExactPixelCounts(t *testing.T) {
	for _, width := range []int16{1, 2, 31, 32, 33, 63, 64, 65, 304, 308, 320} {
		lcd, m := testLCD()
		if err := lcd.FillRectangle(0, 10, width, 1, color.RGBA{255, 0, 0, 255}); err != nil {
			t.Fatal(err)
		}
		if m.written != int(width) {
			t.Fatalf("width %d wrote %d pixels", width, m.written)
		}
		for x := range int(width) {
			if m.pixels[10*LCDWidth+x] != 0xf800 {
				t.Fatal("wrong fill byte order")
			}
		}
	}
	lcd, m := testLCD()
	lcd.FillScreen(color.RGBA{0, 255, 0, 255})
	if err := lcd.Display(); err != nil {
		t.Fatal(err)
	}
	if m.written != LCDWidth*LCDHeight {
		t.Fatal("incomplete screen fill")
	}
	lcd.SetPixel(319, 239, color.RGBA{0, 0, 255, 255})
	if err := lcd.Display(); err != nil {
		t.Fatal(err)
	}
	if m.pixels[len(m.pixels)-1] != 0x001f {
		t.Fatal("last pixel not written")
	}
	before := m.calls
	lcd.SetPixel(-1, 0, color.RGBA{})
	if m.calls != before {
		t.Fatal("clipped glyph touched SPI")
	}
}

func TestLCDRejectsMalformedBitmapAndRecoversAfterTransferError(t *testing.T) {
	lcd, m := testLCD()
	for _, r := range [][4]int16{{-1, 0, 1, 1}, {0, -1, 1, 1}, {319, 0, 2, 1}, {0, 239, 1, 2}, {0, 0, 0, 1}, {32760, 0, 16, 1}} {
		if err := lcd.DrawRGBBitmap8(r[0], r[1], []byte{0, 0}, r[2], r[3]); err == nil {
			t.Fatal("invalid rectangle accepted")
		}
	}
	if err := lcd.DrawRGBBitmap8(0, 0, []byte{0, 0, 0}, 1, 1); err == nil {
		t.Fatal("extra pixel bytes accepted")
	}
	if m.calls != 0 {
		t.Fatal("invalid bitmap reached SPI")
	}
	for _, fail := range []int{1, 2, 3, 4, 5, 6} {
		lcd, m = testLCD()
		m.failure = errors.New("injected transfer failure")
		m.failAt = fail
		if err := lcd.DrawRGBBitmap8(6, 67, make([]byte, RowWidth*RowHeight*2), RowWidth, RowHeight); !errors.Is(err, m.failure) {
			t.Fatal(err)
		}
		if !m.cs.high || m.calls != fail {
			t.Fatal("failed write continued or left LCD selected")
		}
		m.failAt = 0
		lcd.SetPixel(1, 1, color.RGBA{255, 255, 255, 255})
		if err := lcd.Display(); err != nil {
			t.Fatal(err)
		}
		if m.pixels[LCDWidth+1] != 0xffff {
			t.Fatal("next transaction did not recover")
		}
	}
	lcd, m = testLCD()
	m.failure = errors.New("font write failed")
	m.failAt = 1
	lcd.SetPixel(1, 1, color.RGBA{})
	if !errors.Is(lcd.Display(), m.failure) {
		t.Fatal("font SPI error discarded")
	}
	if lcd.Display() != nil {
		t.Fatal("font error was not cleared")
	}
}
