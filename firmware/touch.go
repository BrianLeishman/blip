//go:build tinygo

package main

import (
	"machine"
	"time"
)

// TSC2007 command sequence and calibration follow Adafruit's driver/example:
// https://github.com/adafruit/Adafruit_TSC2007
// https://github.com/adafruit/Adafruit_ILI9341/tree/master/examples/touchpaint_featherwing
func touchADC(command byte) (int, error) {
	if err := machine.I2C1.Tx(0x48, []byte{command}, nil); err != nil {
		return 0, err
	}
	time.Sleep(500 * time.Microsecond)
	var reply [2]byte
	if err := machine.I2C1.Tx(0x48, nil, reply[:]); err != nil {
		return 0, err
	}
	return int(reply[0])<<4 | int(reply[1])>>4, nil
}

// In Rotation270, portrait X becomes landscape Y. We only need
// vertical motion, so absolute horizontal calibration is unnecessary.
func touchY() (y int, pressed bool, err error) {
	defer func() {
		_, powerErr := touchADC(0x00)
		if err == nil {
			err = powerErr
		}
	}()
	z1, err := touchADC(0xe4)
	if err != nil || z1 < 10 {
		return 0, false, err
	}
	z2, err := touchADC(0xf4)
	if err != nil || z2 >= 4095 {
		return 0, false, err
	}
	x1, err := touchADC(0xc4)
	if err != nil {
		return 0, false, err
	}
	x2, err := touchADC(0xc4)
	if err != nil {
		return 0, false, err
	}
	if x1 < 100 || x1 > 4000 || x2-x1 > 100 || x1-x2 > 100 {
		return 0, false, nil
	}
	y = (x1 - 300) * 239 / 3500
	return y, true, nil
}
