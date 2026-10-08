// Package display composes bounded LCD updates in memory before SPI transfer.
package display

import (
	"image/color"

	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"
)

const (
	RowWidth     = 308
	RowHeight    = 16
	TitleLeft    = 18
	TitleRight   = 240
	SpinnerLeft  = 244
	SpinnerRight = 254
	BadgeLeft    = 257
	BadgeRight   = 305
)

type Palette struct {
	Background, Highlight, Text, Selection color.RGBA
}

// Row includes all visible state, so identical frames can skip the LCD transfer.
type Row struct {
	Marker, Title, Spinner, Badge         string
	MarkerColor, SpinnerColor, BadgeColor color.RGBA
	Selected                              bool
}

// Renderer reuses one RGB565 big-endian row buffer. The caller must finish sending
// the returned bytes before composing the next row.
type Renderer struct {
	pixels      [RowWidth * RowHeight * 2]byte
	left, right int16
}

func (r *Renderer) Size() (int16, int16) { return RowWidth, RowHeight }
func (r *Renderer) Display() error       { return nil }

func (r *Renderer) SetPixel(x, y int16, c color.RGBA) {
	if x < r.left || x >= r.right || y < 0 || y >= RowHeight {
		return
	}
	at := (int(y)*RowWidth + int(x)) * 2
	pixel := rgb565(c)
	r.pixels[at] = byte(pixel >> 8)
	r.pixels[at+1] = byte(pixel)
}

func rgb565(c color.RGBA) uint16 {
	return uint16(c.R>>3)<<11 | uint16(c.G>>2)<<5 | uint16(c.B>>3)
}

func (r *Renderer) text(x, left, right int16, s string, c color.RGBA) {
	r.left, r.right = left, right
	tinyfont.WriteLine(r, &proggy.TinySZ8pt7b, x, 11, s, c)
}

func (r *Renderer) Render(row Row, p Palette) []byte {
	background := p.Background
	if row.Selected {
		background = p.Highlight
	}
	pixel := rgb565(background)
	for i := 0; i < len(r.pixels); i += 2 {
		r.pixels[i], r.pixels[i+1] = byte(pixel>>8), byte(pixel)
	}
	if row.Selected {
		r.left, r.right = 0, 3
		for y := int16(0); y < RowHeight; y++ {
			for x := int16(0); x < 3; x++ {
				r.SetPixel(x, y, p.Selection)
			}
		}
	}
	// Independent clips prevent title/status glyphs escaping into another field.
	r.text(4, 4, 16, row.Marker, row.MarkerColor)
	r.text(TitleLeft, TitleLeft, TitleRight, row.Title, p.Text)
	r.text(SpinnerLeft+1, SpinnerLeft, SpinnerRight, row.Spinner, row.SpinnerColor)
	r.text(BadgeLeft, BadgeLeft, BadgeRight, row.Badge, row.BadgeColor)
	return r.pixels[:]
}
