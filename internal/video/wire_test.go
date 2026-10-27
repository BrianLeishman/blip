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
	for _, packet := range []Packet{{Sequence: 1, Kind: Start}, {Sequence: 2, Kind: Tile, Row: Height - TileRows, Pixels: pixels}, {Sequence: 3, Kind: SmoothTile, Row: 54, Pixels: pixels}, {Sequence: 4, Kind: Stop}} {
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
	for _, changed := range []int{5, 9, 10, 11, HeaderBytes, MaxPacket - 1} {
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
	for _, p := range []Packet{{Kind: 255}, {Kind: Tile, Row: 1, Pixels: pixels}, {Kind: Tile, Row: 120, Pixels: pixels}, {Kind: Tile}, {Kind: Start, Pixels: pixels}} {
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

func TestExpandAllRGB332ColorsAndBoundaries(t *testing.T) {
	for _, kind := range []byte{Tile, SmoothTile} {
		format := FormatFor(kind)
		pixels := make([]byte, format.TileBytes())
		for i := range pixels {
			pixels[i] = byte(i)
		}
		dst := make([]byte, format.RenderBytes()+2)
		dst[0], dst[len(dst)-1] = 0x19, 0x27
		if !ExpandTile(dst[1:len(dst)-1], pixels, kind) {
			t.Fatal("expand failed")
		}
		red := [...]uint16{0, 4, 9, 13, 18, 22, 27, 31}
		green := [...]uint16{0, 9, 18, 27, 36, 45, 54, 63}
		blue := [...]uint16{0, 10, 21, 31}
		for y := range format.Rows {
			for x := range format.Width {
				p := pixels[y*format.Width+x]
				want := red[p>>5]<<11 | green[(p>>2)&7]<<5 | blue[p&3]
				for dy := range format.Scale {
					for dx := range format.Scale {
						off := 1 + ((y*format.Scale+dy)*320+x*format.Scale+dx)*2
						if got := binary.BigEndian.Uint16(dst[off : off+2]); got != want {
							t.Fatalf("kind %d color %#x: got %#x want %#x", kind, p, got, want)
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
	f.Add([]byte{0, 'B', 'L', 'V', 1})
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
