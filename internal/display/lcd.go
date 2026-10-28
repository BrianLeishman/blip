package display

import (
	"errors"
	"image/color"

	"tinygo.org/x/drivers"
)

const LCDWidth, LCDHeight = 320, 240

// Pin is the output-pin subset used to select the LCD and distinguish commands.
type Pin interface {
	High()
	Low()
}

// LCD draws into the already-initialized landscape ILI9341. Each write sets both
// address ranges and RAMWR explicitly under one chip-select assertion. In
// particular, repeated marquee updates never depend on a cached hardware window.
// Initialization and rotation are handled by the upstream driver once at startup.
type LCD struct {
	bus         drivers.SPI
	dc, cs      Pin
	commandByte [1]byte
	address     [4]byte
	fill        [64]byte
	pixel       [2]byte
	pending     error
}

func NewLCD(bus drivers.SPI, dc, cs Pin) *LCD {
	cs.High()
	dc.High()
	return &LCD{bus: bus, dc: dc, cs: cs}
}
func (d *LCD) Size() (int16, int16) { return LCDWidth, LCDHeight }

// Display reports errors from the font interface, whose SetPixel cannot return one.
func (d *LCD) Display() error { err := d.pending; d.pending = nil; return err }
func (d *LCD) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || y < 0 || x >= LCDWidth || y >= LCDHeight {
		return
	}
	pixel := rgb565(c)
	d.pixel[0], d.pixel[1] = byte(pixel>>8), byte(pixel)
	if err := d.DrawRGBBitmap8(x, y, d.pixel[:], 1, 1); err != nil {
		d.pending = err
	}
}
func (d *LCD) FillScreen(c color.RGBA) { _ = d.FillRectangle(0, 0, LCDWidth, LCDHeight, c) }

func validRectangle(x, y, w, h int16) bool {
	return x >= 0 && y >= 0 && w > 0 && h > 0 && int(x)+int(w) <= LCDWidth && int(y)+int(h) <= LCDHeight
}

func (d *LCD) command(cmd byte, data []byte) error {
	d.commandByte[0] = cmd
	d.dc.Low()
	if err := d.bus.Tx(d.commandByte[:], nil); err != nil {
		return err
	}
	d.dc.High()
	if len(data) > 0 {
		return d.bus.Tx(data, nil)
	}
	return nil
}

func (d *LCD) window(x, y, w, h int16) error {
	end := x + w - 1
	d.address = [4]byte{byte(x >> 8), byte(x), byte(end >> 8), byte(end)}
	if err := d.command(0x2a, d.address[:]); err != nil {
		return err
	}
	end = y + h - 1
	d.address = [4]byte{byte(y >> 8), byte(y), byte(end >> 8), byte(end)}
	if err := d.command(0x2b, d.address[:]); err != nil {
		return err
	}
	return d.command(0x2c, nil)
}

func (d *LCD) DrawRGBBitmap8(x, y int16, pixels []byte, w, h int16) error {
	if !validRectangle(x, y, w, h) {
		return errors.New("LCD rectangle outside screen")
	}
	if len(pixels) != int(w)*int(h)*2 {
		return errors.New("LCD bitmap must exactly fill its RGB565 rectangle")
	}
	d.cs.Low()
	defer d.cs.High()
	if err := d.window(x, y, w, h); err != nil {
		return err
	}
	return d.bus.Tx(pixels, nil)
}

func (d *LCD) FillRectangle(x, y, w, h int16, c color.RGBA) (err error) {
	defer func() {
		if err != nil {
			d.pending = err
		}
	}()
	if !validRectangle(x, y, w, h) {
		return errors.New("LCD rectangle outside screen")
	}
	pixel := rgb565(c)
	for i := 0; i < len(d.fill); i += 2 {
		d.fill[i], d.fill[i+1] = byte(pixel>>8), byte(pixel)
	}
	d.cs.Low()
	defer d.cs.High()
	if err = d.window(x, y, w, h); err != nil {
		return err
	}
	for remaining := int(w) * int(h) * 2; remaining > 0; {
		n := min(remaining, len(d.fill))
		if err = d.bus.Tx(d.fill[:n], nil); err != nil {
			return err
		}
		remaining -= n
	}
	return nil
}
