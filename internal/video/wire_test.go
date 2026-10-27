package video

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestVideoPacketsAndRecovery(t *testing.T) {
	pixels := make([]byte, TileBytes)
	for i := range pixels {
		pixels[i] = byte(i)
	} // Includes NUL, newline and magic bytes.
	var wire [MaxPacket]byte
	var decoder Decoder
	for _, packet := range []Packet{{Sequence: 1, Kind: Start}, {Sequence: 2, Kind: Tile, Row: Height - TileRows, Pixels: pixels}, {Sequence: 3, Kind: SmoothTile, Row: 57, Pixels: make([]byte, 480)}, {Sequence: 4, Kind: Stop}} {
		n := Encode(wire[:], packet)
		if n == 0 || n > 512 {
			t.Fatal("packet exceeds device RX budget")
		}
		for i, b := range wire[:n] {
			got, done, valid := decoder.Push(b)
			if i < n-1 && done {
				t.Fatal("packet completed early")
			}
			if i == n-1 && (!done || !valid || got.Sequence != packet.Sequence || got.Kind != packet.Kind || got.Row != packet.Row || !bytes.Equal(got.Pixels, packet.Pixels)) {
				t.Fatalf("round trip failed: %+v", got)
			}
		}
	}
	// A corrupt pixel/header is rejected without drawing outside the LCD or losing the next packet.
	for _, changed := range []int{5, 9, 10, 11, HeaderBytes, HeaderBytes + TileBytes - 1} {
		n := Encode(wire[:], Packet{Sequence: 9, Kind: Tile, Pixels: pixels})
		wire[changed] ^= 0x80
		for _, b := range wire[:n] {
			_, done, valid := decoder.Push(b)
			if done && valid {
				t.Fatal("accepted corrupt packet")
			}
		}
		decoder.Reset()
		n = Encode(wire[:], Packet{Sequence: 10, Kind: Start})
		for i, b := range wire[:n] {
			_, done, valid := decoder.Push(b)
			if i == n-1 && (!done || !valid) {
				t.Fatal("did not recover")
			}
		}
	}
	for _, p := range []Packet{{Kind: 255}, {Kind: SmoothTile, Row: 1, Pixels: make([]byte, 480)}, {Kind: Tile, Row: 120, Pixels: pixels}, {Kind: Tile}, {Kind: Start, Pixels: pixels}} {
		if Encode(wire[:], p) != 0 {
			t.Fatalf("accepted invalid packet %+v", p)
		}
	}
	if Encode(wire[:1], Packet{Kind: Start}) != 0 {
		t.Fatal("accepted undersized buffer")
	}
	// Standard CRC-16/CCITT-FALSE check, independent of encoder/decoder agreement.
	if checksum(0xffff, []byte("123456789")) != 0x29b1 {
		t.Fatal("wrong CRC")
	}
}

func TestExpandRGB565ColorsAndBoundaries(t *testing.T) {
	// Include channel extremes and shades that the old RGB332 palette could not preserve.
	colors := []uint16{0x0000, 0xffff, 0xf800, 0x07e0, 0x001f, 0x39e7, 0x0821, 0x7bef, 0x1234, 0xabcd}
	for _, kind := range []byte{Tile, SmoothTile} {
		format := FormatFor(kind)
		pixels := make([]byte, format.TileBytes())
		for i := 0; i < len(pixels)/2; i++ {
			binary.BigEndian.PutUint16(pixels[i*2:], colors[i%len(colors)])
		}
		dst := make([]byte, format.RenderBytes()+2)
		dst[0], dst[len(dst)-1] = 0x19, 0x27
		if !ExpandTile(dst[1:len(dst)-1], pixels, kind) {
			t.Fatal("expand failed")
		}
		for y := range format.Rows {
			for x := range format.Width {
				want := colors[(y*format.Width+x)%len(colors)]
				for dy := range format.Scale {
					for dx := range format.Scale {
						off := 1 + ((y*format.Scale+dy)*320+x*format.Scale+dx)*2
						if got := binary.BigEndian.Uint16(dst[off : off+2]); got != want {
							t.Fatalf("kind %d: lost color bits: got %#x want %#x", kind, got, want)
						}
					}
				}
			}
		}
		if dst[0] != 0x19 || dst[len(dst)-1] != 0x27 {
			t.Fatal("overwrote tile boundaries")
		}
		if ExpandTile(dst, pixels, kind) || ExpandTile(dst[1:len(dst)-1], pixels[:1], kind) {
			t.Fatal("accepted invalid tile size")
		}
	}
}

func FuzzDecoder(f *testing.F) {
	var wire [MaxPacket]byte
	n := Encode(wire[:], Packet{Kind: Tile, Pixels: make([]byte, TileBytes)})
	f.Add(wire[:n])
	f.Add([]byte{0, 'B', 'L', 'V', Version})
	f.Fuzz(func(t *testing.T, input []byte) {
		var d Decoder
		for _, b := range input {
			p, done, valid := d.Push(b)
			if done && valid && !p.Valid() {
				t.Fatal("accepted invalid packet")
			}
		}
	})
}
