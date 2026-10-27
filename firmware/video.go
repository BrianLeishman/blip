//go:build tinygo

package main

import (
	"image/color"
	"time"

	"github.com/BrianLeishman/blip/dashboard"
	"github.com/BrianLeishman/blip/internal/video"
)

const videoProtocolVersion = video.Version

var videoMode bool
var videoDecoder video.Decoder
var videoTile [video.MaxTileRGB565]byte
var lastVideoPacket time.Time
var lastVideoData time.Time

func showVideoMessage(message string) {
	screen.FillScreen(color.RGBA{0, 0, 0, 255})
	label(12, 30, "blip video", green)
	for i := 0; len(message) > 0 && i < 5; i++ {
		n := len(message)
		if n > 45 {
			n = 45
		}
		label(12, int16(65+i*18), message[:n], white)
		message = message[n:]
	}
	label(12, 222, "Swipe left or click knob to return", muted)
}

func enterVideo() {
	videoMode = true
	lastVideoData = time.Now()
	showVideoMessage("Looking for Firefox Picture-in-Picture...")
}
func leaveVideo() {
	videoMode = false
	emit(dashboard.Event{Kind: "video-stop"})
	draw()
}

func receiveVideo(b byte) {
	lastVideoPacket = time.Now()
	p, done, valid := videoDecoder.Push(b)
	if !done {
		return
	}
	e := dashboard.Event{Kind: "video-ack", Sequence: p.Sequence}
	if !valid {
		e.Error = "Invalid video packet"
	} else {
		switch p.Kind {
		case video.Start:
			if !videoMode {
				enterVideo()
			}
			lastVideoData = time.Now()
		case video.Stop:
			videoMode = false
			draw()
		case video.Tile, video.SmoothTile:
			if videoMode {
				format := video.FormatFor(p.Kind)
				pixels := videoTile[:format.RenderBytes()]
				video.ExpandTile(pixels, p.Pixels, p.Kind)
				if err := screen.DrawRGBBitmap8(0, int16(p.Row)*int16(format.Scale), pixels, 320, int16(format.Rows*format.Scale)); err != nil {
					e.Error = err.Error()
				}
				lastVideoData = time.Now()
			}
		}
	}
	emit(e)
}
