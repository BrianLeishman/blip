package display

import "fmt"

// Status is read from the LCD itself, independently of the dashboard's row cache.
type Status struct {
	ID                          uint16
	Power, MemoryAccess, Format byte
}

func (s Status) String() string {
	return fmt.Sprintf("id=%04x power=%02x madctl=%02x format=%02x", s.ID, s.Power, s.MemoryAccess, s.Format)
}

// Healthy checks the serial interface's RGB565 format (DBI bits 2:0), rotation,
// and an awake, enabled display with no idle/partial mode or booster fault.
// The RGB/DPI format bits are unrelated to SPI. Normal-mode bit 3 can read zero
// on the working FeatherWing, whose verified readback is power=94 / format=05.
func (s Status) Healthy() bool {
	return s.ID == 0x9341 && s.Power&0xf4 == 0x94 && s.MemoryAccess == 0xf8 && s.Format&0x07 == 0x05
}

// readRegister follows Adafruit_ILI9341::readcommand8's indexed SPI read. The
// caller must use a read-safe bus speed (firmware uses 1 MHz) and connect MISO.
// https://github.com/adafruit/Adafruit_ILI9341/blob/master/Adafruit_ILI9341.cpp
func (d *LCD) readRegister(register, index byte) (byte, error) {
	d.cs.Low()
	d.address[0] = 0x10 + index
	err := d.command(0xd9, d.address[:1])
	d.cs.High()
	if err != nil {
		return 0, err
	}
	d.cs.Low()
	defer d.cs.High()
	if err := d.command(register, nil); err != nil {
		return 0, err
	}
	var tx, rx [1]byte
	if err := d.bus.Tx(tx[:], rx[:]); err != nil {
		return 0, err
	}
	return rx[0], nil
}

func (d *LCD) ReadStatus() (Status, error) {
	var values [5]byte
	for i, r := range [][2]byte{{0xd3, 2}, {0xd3, 3}, {0x0a, 0}, {0x0b, 0}, {0x0c, 0}} {
		value, err := d.readRegister(r[0], r[1])
		if err != nil {
			return Status{}, err
		}
		values[i] = value
	}
	return Status{ID: uint16(values[0])<<8 | uint16(values[1]), Power: values[2], MemoryAccess: values[3], Format: values[4]}, nil
}
