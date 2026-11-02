package display

import (
	"errors"
	"testing"
	"time"
)

// Control-register model for indexed readback and reset/recovery tests. Pixel
// upload bounds are tested separately by lcdModel.
type controlModel struct {
	dc, cs         testPin
	command, index byte
	registers      map[byte][]byte
	calls, failAt  int
	resets         int
	commands       []byte
}

var controlFailure = errors.New("injected control transfer failure")

func (m *controlModel) Tx(w, r []byte) error {
	m.calls++
	if m.calls == m.failAt {
		return controlFailure
	}
	if m.cs.high || len(w) != len(r) || r == nil {
		return errors.New("invalid control transaction")
	}
	if !m.dc.high {
		if len(w) != 1 {
			return errors.New("invalid command length")
		}
		m.command = w[0]
		m.commands = append(m.commands, w[0])
		switch w[0] {
		case 0x01:
			m.resets++
			memory := byte(0)
			format := byte(0x06)
			if len(m.registers[0x0b]) > 0 {
				memory = m.registers[0x0b][0]
			}
			if len(m.registers[0x0c]) > 0 {
				format = m.registers[0x0c][0]
			}
			m.registers = map[byte][]byte{0xd3: {0, 0, 0x93, 0x41}, 0x0a: {0x08}, 0x0b: {memory}, 0x0c: {format}}
		case 0x11:
			m.registers[0x0a][0] |= 0x90
		case 0x29:
			m.registers[0x0a][0] |= 0x04
		}
		return nil
	}
	switch m.command {
	case 0xd9:
		if len(w) != 1 || w[0] < 0x10 {
			return errors.New("invalid read index")
		}
		m.index = w[0] - 0x10
	case 0xd3, 0x0a, 0x0b, 0x0c:
		if len(w) != 1 || int(m.index) >= len(m.registers[m.command]) {
			return errors.New("invalid indexed read")
		}
		r[0] = m.registers[m.command][m.index]
	case 0x36:
		m.registers[0x0b] = append([]byte(nil), w...)
	case 0x3a:
		// The serial panel reports the DBI format, not the written RGB/DPI bits.
		m.registers[0x0c] = []byte{w[0] & 0x07}
	}
	return nil
}
func (m *controlModel) Transfer(b byte) (byte, error) { return b, nil }

func controlLCD() (*LCD, *controlModel) {
	m := &controlModel{}
	return NewLCD(NewBlockingSPI(m), &m.dc, &m.cs), m
}

func TestReadStatusAndRepeatableReset(t *testing.T) {
	lcd, m := controlLCD()
	for range 50 {
		waits := 0
		if err := lcd.reset(func(d time.Duration) {
			if d < 120*time.Millisecond || !m.cs.high {
				t.Fatal("missing reset/sleep-out delay or LCD left selected")
			}
			waits++
		}); err != nil {
			t.Fatal(err)
		}
		if waits != 3 {
			t.Fatal("missing panel startup wait")
		}
		s, err := lcd.ReadStatus()
		if err != nil || !s.Healthy() || s.ID != 0x9341 {
			t.Fatalf("unexpected readback: %s %v", s, err)
		}
		if !m.cs.high {
			t.Fatal("readback left LCD selected")
		}
	}
	if m.resets != 50 {
		t.Fatalf("reset count changed across repeated initialization: %d", m.resets)
	}
	// The very next frame can set its own window; readback doesn't keep CS low.
	if err := lcd.DrawRGBBitmap8(0, 0, []byte{0, 0}, 1, 1); err != nil {
		t.Fatal(err)
	}
}

func TestControlTransferFailuresReleaseChipSelect(t *testing.T) {
	for failAt := 1; failAt <= 20; failAt++ {
		lcd, m := controlLCD()
		if err := lcd.reset(func(time.Duration) {}); err != nil {
			t.Fatal(err)
		}
		m.failAt = m.calls + failAt
		_, err := lcd.ReadStatus()
		if !errors.Is(err, controlFailure) || !m.cs.high {
			t.Fatalf("failed read %d didn't release CS: %v", failAt, err)
		}
	}
	for failAt := 1; failAt <= 45; failAt++ {
		lcd, m := controlLCD()
		m.failAt = failAt
		if err := lcd.reset(func(time.Duration) {}); !errors.Is(err, controlFailure) || !m.cs.high {
			t.Fatalf("failed reset %d didn't stop/release CS: %v", failAt, err)
		}
	}
}

func TestHealthRequiresReadbackAndConfirmsLossBeforeRecovery(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	good := Status{ID: 0x9341, Power: 0x94, MemoryAccess: 0xf8, Format: 0x05}
	reset := Status{ID: 0x9341, Power: 0x08, MemoryAccess: 0xf8, Format: 0x05}
	var h Health
	for i := range 100 {
		for _, missing := range []Status{{}, {ID: 0xffff, Power: 0xff, MemoryAccess: 0xff, Format: 0xff}, reset} {
			if h.Observe(missing, start.Add(time.Duration(i)*time.Minute)) {
				t.Fatal("unwired/unverified LCD armed automatic recovery")
			}
		}
	}
	if h.Observe(good, start) || h.Observe(reset, start.Add(5*time.Second)) {
		t.Fatal("healthy or single bad reading triggered recovery")
	}
	if !h.Observe(reset, start.Add(10*time.Second)) {
		t.Fatal("confirmed controller reset didn't trigger recovery")
	}
	for i := 3; i < 14; i++ {
		if h.Observe(reset, start.Add(time.Duration(i)*5*time.Second)) {
			t.Fatal("recovery loop ignored one-minute cooldown")
		}
	}
	if !h.Observe(reset, start.Add(70*time.Second)) {
		t.Fatal("failed LCD wasn't retried after cooldown")
	}
	h.Observe(good, start.Add(75*time.Second))
	h.Observe(reset, start.Add(140*time.Second))
	h.Observe(good, start.Add(145*time.Second))
	if h.Observe(reset, start.Add(150*time.Second)) {
		t.Fatal("isolated bad read was carried across a healthy reading")
	}
}

func TestHealthyUsesSerialFormatAndRequiredPowerBits(t *testing.T) {
	for _, power := range []byte{0x94, 0x9c} {
		for _, format := range []byte{0x05, 0x55} {
			s := Status{ID: 0x9341, Power: power, MemoryAccess: 0xf8, Format: format}
			if !s.Healthy() {
				t.Fatalf("healthy serial panel rejected: %s", s)
			}
		}
	}
	for _, power := range []byte{0x14, 0x90, 0x84, 0xd4, 0xb4, 0x08, 0xff} {
		if (Status{ID: 0x9341, Power: power, MemoryAccess: 0xf8, Format: 0x05}).Healthy() {
			t.Fatalf("incorrect power/mode accepted: %02x", power)
		}
	}
	for _, format := range []byte{0x00, 0x06, 0x66, 0xff} {
		if (Status{ID: 0x9341, Power: 0x94, MemoryAccess: 0xf8, Format: format}).Healthy() {
			t.Fatalf("non-RGB565 serial format accepted: %02x", format)
		}
	}
}

func TestHealthDetectsFaultStatesMeasuredOnFeatherWing(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	good := Status{ID: 0x9341, Power: 0x94, MemoryAccess: 0xf8, Format: 0x05}
	for _, power := range []byte{0x90, 0x04, 0x08} {
		var h Health
		h.Observe(good, start)
		bad := good
		bad.Power = power // Display off, sleep in, and software reset respectively.
		if h.Observe(bad, start.Add(5*time.Second)) || !h.Observe(bad, start.Add(10*time.Second)) {
			t.Fatalf("measured fault state %s wasn't confirmed in two reads", bad)
		}
	}
}
