//go:build tinygo

package main

import (
	"encoding/binary"
	"encoding/json"
	"image/color"
	"machine"
	"strconv"
	"strings"
	"time"

	"github.com/BrianLeishman/blip/dashboard"
	"tinygo.org/x/drivers/ili9341"
	"tinygo.org/x/drivers/seesaw"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"
)

var screen *ili9341.Device
var bg = color.RGBA{12, 17, 23, 255}
var white = color.RGBA{230, 237, 243, 255}
var muted = color.RGBA{135, 151, 168, 255}
var green = color.RGBA{80, 220, 145, 255}
var amber = color.RGBA{255, 193, 90, 255}
var red = color.RGBA{255, 110, 120, 255}
var snapshot = dashboard.Snapshot{Version: 1, Status: "Waiting for Mac companion"}
var selected int
var scrollTop int
var marquee dashboard.Marquee
var lastUpdate time.Time
var testPosition int32
var clicks int
var encoderError string
var spinnerFrame int
var highlight = color.RGBA{28, 85, 150, 255}

func emit(e dashboard.Event) {
	b, err := json.Marshal(e)
	if err == nil {
		machine.Serial.Write(append(b, '\n'))
	}
}
func label(x, y int16, s string, c color.RGBA) {
	tinyfont.WriteLine(screen, &proggy.TinySZ8pt7b, x, y, s, c)
}
func ascii(s string, max int) string {
	// Abbreviate display text before truncation; IDs and links stay untouched.
	s = strings.ReplaceAll(s, "StirlingMarketingGroup", "SMG")
	s = strings.ReplaceAll(s, "BrianLeishman", "BL")
	b := make([]byte, 0, max)
	for _, r := range s {
		if len(b) >= max {
			break
		}
		if r >= 32 && r < 127 {
			b = append(b, byte(r))
		} else {
			b = append(b, '?')
		}
	}
	if len([]rune(s)) > max && max >= 3 {
		copy(b[max-3:], "...")
	}
	return string(b)
}
func draw() {
	if selected >= 0 && selected < len(snapshot.Rows) {
		r := snapshot.Rows[selected]
		marquee.Set(r.ID, ascii(r.Title, len(r.Title)), time.Now())
	} else {
		marquee.Set("", "", time.Now())
	}
	screen.FillScreen(bg)
	label(10, 17, "blip", green)
	label(48, 17, "Brian's Little Information Panel", muted)
	ready, mine, review, urgent := 0, 0, 0, 0
	for _, r := range snapshot.Rows {
		switch r.Section {
		case "ready":
			ready++
		case "mine":
			mine++
		case "review":
			review++
		case "urgent":
			urgent++
		}
	}
	label(10, 36, "MERGE "+strconv.Itoa(ready), green)
	label(90, 36, "MINE "+strconv.Itoa(mine), muted)
	label(166, 36, "REVIEW "+strconv.Itoa(review), amber)
	if urgent > 0 {
		label(261, 36, "! "+strconv.Itoa(urgent), red)
	}

	screen.FillRectangle(8, 43, 304, 1, muted)
	if len(snapshot.Rows) == 0 {
		if snapshot.Revision == "" {
			label(12, 77, "Screen OK / TinyGo", green)
			label(12, 100, "Turn and click the knob", white)
			label(12, 124, "Position: "+strconv.Itoa(int(testPosition)), white)
			label(12, 145, "Clicks:   "+strconv.Itoa(clicks), white)
		} else {
			label(12, 90, "All clear. Nothing needs attention.", green)
		}
	} else {
		// One continuous list, ten visible rows; selection scrolls the viewport.
		start := scrollTop
		for i := start; i < len(snapshot.Rows) && i < start+dashboard.VisibleRows; i++ {
			r := snapshot.Rows[i]
			y := int16(61 + (i-start)*17)
			c := green
			marker := "M"
			if r.Section == "mine" {
				c = muted
				marker = "O"
			}
			if r.Section == "review" {
				c = amber
				marker = "R"
			}
			if r.Section == "urgent" {
				c = red
				marker = "!"
			}
			if i == selected {
				screen.FillRectangle(6, y-11, 308, 16, highlight)
				screen.FillRectangle(6, y-11, 3, 16, white)
			}
			label(10, y, marker, c)
			title := ascii(r.Title, dashboard.TitleColumns)
			if i == selected {
				title = marquee.Text()
			}
			label(24, y, title, white)
			if strings.HasSuffix(r.Badge, "FAIL") || r.Badge == "CONFLICT" {
				c = red
			}
			label(263, y, ascii(r.Badge, 8), c)
		}
	}
	screen.FillRectangle(8, 224, 304, 1, muted)
	status := snapshot.Status
	identity := ""
	if selected >= 0 && selected < len(snapshot.Rows) {
		r := snapshot.Rows[selected]
		identity = r.Identity()
		status = r.Detail
		// Details from the companion may repeat the leading #number.
		number, _, _ := strings.Cut(identity, " ")
		status = strings.TrimPrefix(status, number+" ")
		status = strings.NewReplacer(
			"Review requested from you", "Your review",
			"changes requested", "changes needed",
			"merge conflict", "CONFLICT",
			"no requests", "no req",
			" requested", " req",
			" / ", " ",
		).Replace(status)
	}
	if encoderError != "" {
		status = "Encoder: " + encoderError
	} else if !lastUpdate.IsZero() && time.Since(lastUpdate) > 90*time.Second {
		status = "STALE DATA"
	}
	footer := ascii(status, 50)
	if identity != "" {
		// Reserve space for status; shorten the repo before hiding CI/conflicts.
		status = ascii(status, 34)
		footer = ascii(identity, 50)
		if status != "" {
			footer = ascii(identity, 49-len(status)) + " " + status
		}
	}
	label(10, 237, footer, muted)
	drawSpinners()
}

// Animate only the small reserved CI column, preserving the rest of the screen.
func drawSpinners() {
	start := scrollTop
	for i := start; i < len(snapshot.Rows) && i < start+dashboard.VisibleRows; i++ {
		if !snapshot.Rows[i].ChecksRunning {
			continue
		}
		y := int16(61 + (i-start)*17)
		background := bg
		if i == selected {
			background = highlight
		}
		screen.FillRectangle(250, y-11, 10, 14, background)
		glyph, ink := string("|/-\\"[spinnerFrame%4]), amber
		if time.Since(lastUpdate) > 90*time.Second {
			glyph, ink = "-", muted
		}
		label(251, y, glyph, ink)
	}
}

func initEncoder() (*seesaw.Device, error) {
	machine.I2C1.Configure(machine.I2CConfig{Frequency: 100000, SDA: machine.SDA_PIN, SCL: machine.SCL_PIN})
	e := seesaw.New(machine.I2C1)
	e.Address = 0x36
	e.ReadDelay = time.Millisecond
	if err := e.SoftReset(); err != nil {
		return e, err
	}
	// Button is seesaw GPIO 24: input with internal pull-up.
	mask := []byte{1, 0, 0, 0}
	for _, fn := range []seesaw.FunctionAddress{seesaw.FunctionGpioDirclrBulk, seesaw.FunctionGpioPullenset, seesaw.FunctionGpioBulkSet} {
		if err := e.Write(seesaw.ModuleGpioBase, fn, mask); err != nil {
			return e, err
		}
	}
	return e, nil
}
func main() {
	machine.Serial.Configure(machine.UARTConfig{})
	machine.SPI0.Configure(machine.SPIConfig{Frequency: 24000000, SCK: machine.SPI0_SCK_PIN, SDO: machine.SPI0_SDO_PIN, SDI: machine.SPI0_SDI_PIN})
	// FeatherWing SD CS is D5; keep the unused SD card deselected.
	machine.D5.Configure(machine.PinConfig{Mode: machine.PinOutput})
	machine.D5.High()
	screen = ili9341.NewSPI(machine.SPI0, machine.D10, machine.D9, machine.NoPin)
	screen.Configure(ili9341.Config{Rotation: ili9341.Rotation270})
	draw()
	encoder, err := initEncoder()
	if err != nil {
		encoderError = "not found at 0x36"
	}
	draw()
	var line [16384]byte
	n := 0
	discard := false
	var prevPosition int32
	if err == nil {
		prevPosition, _ = encoder.GetEncoderPosition(0, false)
	}
	lastTouch := time.Now()
	var swipe dashboard.Swipe
	touchFailed := false
	lastPoll := time.Now()
	lastHello := time.Time{}
	offlineShown := false
	lastSpinner := time.Now()
	lastRetry := time.Now()
	var rawPressed, stablePressed bool
	changed := time.Now()
	for {
		for machine.Serial.Buffered() > 0 {
			b, e := machine.Serial.ReadByte()
			if e != nil {
				break
			}
			if b == '\n' {
				if !discard && n > 0 {
					var next dashboard.Snapshot
					if json.Unmarshal(line[:n], &next) == nil && next.Version == 1 && len(next.Rows) <= dashboard.MaxRows {
						needsDraw := snapshot.Revision == "" || !snapshot.SameContent(next) || offlineShown || (!lastUpdate.IsZero() && time.Since(lastUpdate) > 90*time.Second)
						id := ""
						if selected >= 0 && selected < len(snapshot.Rows) {
							id = snapshot.Rows[selected].ID
						}
						snapshot = next
						if selected >= 0 {
							selected = dashboard.Selected(snapshot.Rows, id)
							scrollTop = dashboard.ScrollTop(scrollTop, selected, len(snapshot.Rows))
						} else {
							scrollTop = dashboard.ClampTop(scrollTop, len(snapshot.Rows))
						}
						lastUpdate = time.Now()
						offlineShown = false
						if needsDraw {
							draw()
						}
						emit(dashboard.Event{Kind: "ack", Revision: snapshot.Revision})
					}
				}
				n = 0
				discard = false
			} else if n < len(line) {
				line[n] = b
				n++
			} else {
				discard = true
			}
		}
		now := time.Now()
		if now.Sub(lastHello) > 5*time.Second {
			emit(dashboard.Event{Kind: "hello", Revision: snapshot.Revision, Error: encoderError})
			lastHello = now
		}
		if encoderError != "" && now.Sub(lastRetry) > 5*time.Second {
			encoder, err = initEncoder()
			lastRetry = now
			if err == nil {
				encoderError = ""
				prevPosition, _ = encoder.GetEncoderPosition(0, false)
				draw()
			}
		}
		if now.Sub(lastTouch) >= 40*time.Millisecond {
			lastTouch = now
			y, pressed, touchErr := touchY()
			if touchErr != nil {
				if !touchFailed {
					emit(dashboard.Event{Kind: "touch-error", Error: touchErr.Error()})
				}
				touchFailed = true
				swipe.Move(0, false)
			} else {
				touchFailed = false
				if delta := swipe.Move(y, pressed); delta != 0 && len(snapshot.Rows) > 0 {
					next := dashboard.ClampTop(scrollTop+delta, len(snapshot.Rows))
					if next != scrollTop {
						if selected >= 0 {
							selected += next - scrollTop
						}
						scrollTop = next
						emit(dashboard.Event{Kind: "touch-scroll", Position: int32(scrollTop)})
						draw()
					}
				}
			}
		}

		if encoderError == "" && now.Sub(lastPoll) >= 10*time.Millisecond {
			lastPoll = now
			pos, e := encoder.GetEncoderPosition(0, false)
			var pins [4]byte
			pe := encoder.Read(seesaw.ModuleGpioBase, seesaw.FunctionGpioBulk, pins[:])
			if e != nil || pe != nil {
				encoderError = "connection lost"
				draw()
				continue
			}
			if pos != prevPosition {
				// This encoder counts down clockwise; clockwise advances the list.
				delta := prevPosition - pos
				prevPosition = pos
				testPosition = pos
				if len(snapshot.Rows) > 0 {
					selected = dashboard.MoveSelection(selected, int(delta), len(snapshot.Rows))
				}
				scrollTop = dashboard.ScrollTop(scrollTop, selected, len(snapshot.Rows))
				emit(dashboard.Event{Kind: "turn", Position: pos})
				draw()
			}
			pressed := binary.BigEndian.Uint32(pins[:])&(1<<24) == 0
			if pressed != rawPressed {
				rawPressed = pressed
				changed = now
			}
			if pressed != stablePressed && now.Sub(changed) > 30*time.Millisecond {
				stablePressed = pressed
				if pressed {
					clicks++
					if selected >= 0 && selected < len(snapshot.Rows) {
						emit(dashboard.Event{Kind: "open", Revision: snapshot.Revision, ID: snapshot.Rows[selected].ID})
					} else {
						emit(dashboard.Event{Kind: "click"})
					}
					draw()
				}
			}
		}
		if !offlineShown && !lastUpdate.IsZero() && now.Sub(lastUpdate) > 90*time.Second {
			draw()
			offlineShown = true
		}
		if now.Sub(lastSpinner) >= 250*time.Millisecond && !lastUpdate.IsZero() && now.Sub(lastUpdate) <= 90*time.Second {
			spinnerFrame = (spinnerFrame + 1) % 4
			drawSpinners()
			lastSpinner = now
		}
		if selected >= scrollTop && selected < len(snapshot.Rows) && selected < scrollTop+dashboard.VisibleRows && marquee.Advance(now) {
			y := int16(61 + (selected-scrollTop)*17)
			// Redraw only the title; preserve highlight bar, badge and CI spinner.
			screen.FillRectangle(24, y-11, dashboard.TitleColumns*6, 16, highlight)
			label(24, y, marquee.Text(), white)
		}
		time.Sleep(time.Millisecond)
	}
}
