package display

import "time"

// These panel settings match the TinyGo ILI9341 driver used by the original
// startup path. Keep the sequence immutable: the upstream Configure appends its
// sleep-out/display-on tail to a package-global slice on every initialization.
// https://github.com/tinygo-org/drivers/blob/v0.36.0/ili9341/ili9341.go
var panelSetup = [...]struct {
	command byte
	data    []byte
}{
	{0xef, []byte{0x03, 0x80, 0x02}},
	{0xcf, []byte{0x00, 0xc1, 0x30}},
	{0xed, []byte{0x64, 0x03, 0x12, 0x81}},
	{0xe8, []byte{0x85, 0x00, 0x78}},
	{0xcb, []byte{0x39, 0x2c, 0x00, 0x34, 0x02}},
	{0xf7, []byte{0x20}},
	{0xea, []byte{0x00, 0x00}},
	{0xc0, []byte{0x23}},
	{0xc1, []byte{0x10}},
	{0xc5, []byte{0x3e, 0x28}},
	{0xc7, []byte{0x86}},
	{0x36, []byte{0x48}},
	{0x37, []byte{0x00, 0x00}},
	{0x3a, []byte{0x55}},
	{0xb1, []byte{0x00, 0x18}},
	{0xb6, []byte{0x08, 0x82, 0x27}},
	{0xf2, []byte{0x00}},
	{0x26, []byte{0x01}},
	{0xe0, []byte{0x0f, 0x31, 0x2b, 0x0c, 0x0e, 0x08, 0x4e, 0xf1, 0x37, 0x07, 0x10, 0x03, 0x0e, 0x09, 0x00}},
	{0xe1, []byte{0x00, 0x0e, 0x14, 0x03, 0x11, 0x07, 0x31, 0xc1, 0x48, 0x08, 0x0f, 0x0c, 0x31, 0x36, 0x0f}},
}

func (d *LCD) sendCommand(command byte, data []byte) error {
	d.cs.Low()
	defer d.cs.High()
	return d.command(command, data)
}

// Reset reinitializes only the LCD. GPIOs must already be configured as outputs.
// No hardware reset wire is needed. The caller redraws its retained UI afterward.
func (d *LCD) Reset() error {
	return d.reset(time.Sleep)
}

func (d *LCD) reset(wait func(time.Duration)) error {
	if err := d.sendCommand(0x01, nil); err != nil {
		return err
	}
	wait(150 * time.Millisecond)
	for _, setting := range panelSetup {
		if err := d.sendCommand(setting.command, setting.data); err != nil {
			return err
		}
	}
	if err := d.sendCommand(0x11, nil); err != nil {
		return err
	}
	wait(150 * time.Millisecond)
	if err := d.sendCommand(0x29, nil); err != nil {
		return err
	}
	wait(150 * time.Millisecond)
	// Rotation270: mirror X/Y, exchange axes, BGR, bottom-to-top refresh.
	return d.sendCommand(0x36, []byte{0xf8})
}
