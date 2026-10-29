package display

import "image/color"

// Band composes header/footer text in RAM. Sending the finished bitmap avoids
// thousands of single-pixel LCD address/command transactions for each label.
// One buffer is reused for the 44-pixel header and 16-pixel footer.
type Band struct {
	pixels [LCDWidth * 44 * 2]byte
	height int16
}

func (b *Band) Clear(height int16, c color.RGBA) {
	if height <= 0 || height > 44 {
		panic("invalid display band height")
	}
	b.height = height
	pixel := rgb565(c)
	for i := 0; i < int(height)*LCDWidth*2; i += 2 {
		b.pixels[i], b.pixels[i+1] = byte(pixel>>8), byte(pixel)
	}
}

func (b *Band) Size() (int16, int16) { return LCDWidth, b.height }
func (b *Band) Display() error       { return nil }
func (b *Band) Pixels() []byte       { return b.pixels[:int(b.height)*LCDWidth*2] }

func (b *Band) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || x >= LCDWidth || y < 0 || y >= b.height {
		return
	}
	i := (int(y)*LCDWidth + int(x)) * 2
	pixel := rgb565(c)
	b.pixels[i], b.pixels[i+1] = byte(pixel>>8), byte(pixel)
}

func (b *Band) Line(x, y, width int16, c color.RGBA) {
	for offset := int16(0); offset < width; offset++ {
		b.SetPixel(x+offset, y, c)
	}
}
