// Package video defines bounded, acknowledged USB video packets shared by host and firmware.
package video

import "encoding/binary"

const (
	Width              = 160
	Height             = 120
	FrameBytes         = Width * Height // RGB332: one byte per pixel.
	TileRows           = 3
	TileBytes          = Width * TileRows
	HeaderBytes        = 13
	MaxPacket          = HeaderBytes + TileBytes // Fits inside TinyGo's 512-byte USB RX ring.
	Start         byte = 1
	Tile          byte = 2
	Stop          byte = 3
	SmoothTile    byte = 4
	MaxTileRGB565      = 320 * 6 * 4 * 2
)

// Format keeps the wire image small while always filling the 320x240 panel.
type Format struct {
	Kind                       byte
	Width, Height, Scale, Rows int
}

func FormatFor(kind byte) Format {
	switch kind {
	case Tile:
		return Format{Tile, 160, 120, 2, 3}
	case SmoothTile:
		return Format{SmoothTile, 80, 60, 4, 6}
	}
	return Format{}
}
func (f Format) FrameBytes() int  { return f.Width * f.Height }
func (f Format) TileBytes() int   { return f.Width * f.Rows }
func (f Format) RenderBytes() int { return 320 * f.Rows * f.Scale * 2 }

var magic = [5]byte{0, 'B', 'L', 'V', 1}

type Packet struct {
	Sequence uint32
	Kind     byte
	Row      byte
	Pixels   []byte
}

func checksum(crc uint16, data []byte) uint16 {
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func (p Packet) Valid() bool {
	switch p.Kind {
	case Start, Stop:
		return p.Row == 0 && len(p.Pixels) == 0
	case Tile, SmoothTile:
		f := FormatFor(p.Kind)
		return int(p.Row) <= f.Height-f.Rows && int(p.Row)%f.Rows == 0 && len(p.Pixels) == f.TileBytes()
	}
	return false
}

// Encode returns zero for an invalid packet or an undersized destination.
func Encode(dst []byte, p Packet) int {
	n := HeaderBytes + len(p.Pixels)
	if !p.Valid() || len(dst) < n {
		return 0
	}
	copy(dst, magic[:])
	binary.LittleEndian.PutUint32(dst[5:9], p.Sequence)
	dst[9], dst[10] = p.Kind, p.Row
	copy(dst[HeaderBytes:], p.Pixels)
	// CRC is stored before the payload but covers sequence, kind, row, and pixels.
	crc := checksum(checksum(0xffff, dst[5:11]), p.Pixels)
	binary.LittleEndian.PutUint16(dst[11:13], crc)
	return n
}

// Decoder owns a fixed buffer. Pixels remain valid until the next Push call.
// The caller starts it only at a NUL outside a JSON line and resets after an idle timeout.
type Decoder struct {
	data [MaxPacket]byte
	n    int
}

func (d *Decoder) Active() bool { return d.n > 0 }
func (d *Decoder) Reset()       { d.n = 0 }
func (d *Decoder) Push(b byte) (p Packet, done, valid bool) {
	if d.n < len(magic) && b != magic[d.n] {
		d.Reset()
		return p, true, false
	}
	d.data[d.n] = b
	d.n++
	if d.n < HeaderBytes {
		return p, false, false
	}
	kind := d.data[9]
	n := HeaderBytes
	if kind == Tile || kind == SmoothTile {
		n += FormatFor(kind).TileBytes()
	}
	if d.n < n {
		return p, false, false
	}
	p = Packet{Sequence: binary.LittleEndian.Uint32(d.data[5:9]), Kind: kind, Row: d.data[10], Pixels: d.data[HeaderBytes:n]}
	want := binary.LittleEndian.Uint16(d.data[11:13])
	// Compute over the header and payload without moving the returned pixel slice.
	crc := checksum(checksum(0xffff, d.data[5:11]), p.Pixels)
	d.Reset()
	return p, true, p.Valid() && crc == want
}

// ExpandTile scales RGB332 pixels to the panel and converts to big-endian RGB565.
func ExpandTile(dst []byte, pixels []byte, kind byte) bool {
	f := FormatFor(kind)
	if f.Width == 0 || len(pixels) != f.TileBytes() || len(dst) != f.RenderBytes() {
		return false
	}
	for y := range f.Rows {
		for x := range f.Width {
			p := pixels[y*f.Width+x]
			r, g, b := uint16(p>>5), uint16((p>>2)&7), uint16(p&3)
			c := ((r<<2 | r>>1) << 11) | ((g<<3 | g) << 5) | (b<<3 | b<<1 | b>>1)
			for dy := range f.Scale {
				for dx := range f.Scale {
					o := ((y*f.Scale+dy)*320 + x*f.Scale + dx) * 2
					dst[o], dst[o+1] = byte(c>>8), byte(c)
				}
			}
		}
	}
	return true
}
