package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BrianLeishman/blip/internal/video"
)

type videoOptions struct{ test, detail bool }

type videoSession struct {
	format                             video.Format
	message                            string
	cancel                             context.CancelFunc
	frames                             chan []byte
	errors                             chan error
	frame, next, last                  []byte
	row                                int
	sequence                           uint32
	pending, ready, failed             bool
	kind                               byte
	sent, lastFrame, started, reported time.Time
	count, reportCount                 int
}

func newVideoSession(ctx context.Context, options videoOptions) *videoSession {
	ctx, cancel := context.WithCancel(ctx)
	now := time.Now()
	kind := video.SmoothTile
	if options.detail {
		kind = video.Tile
	}
	v := &videoSession{format: video.FormatFor(kind), cancel: cancel, frames: make(chan []byte, 1), errors: make(chan error, 1), started: now, reported: now}
	go func() {
		var err error
		if options.test {
			err = testFrames(ctx, v.frames, v.format)
		} else {
			err = captureFrames(ctx, v.frames, v.format)
		}
		if ctx.Err() == nil {
			v.errors <- err
		}
	}()
	return v
}

// offerVideo retains only the freshest frame; a slow device never accumulates a backlog.
func offerVideo(ctx context.Context, frames chan []byte, frame []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-frames:
	default:
	}
	select {
	case frames <- frame:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func captureFrames(ctx context.Context, frames chan []byte, format video.Format) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	helper := filepath.Join(filepath.Dir(executable), "BlipCapture.app", "Contents", "MacOS", "blip-capture")
	if _, err := os.Stat(helper); err != nil {
		return errors.New("Capture helper missing. Run make capture-helper on your Mac.")
	}
	args := []string{"--request-permission"}
	if format.Kind == video.Tile {
		args = append(args, "--detail")
	}
	cmd := exec.CommandContext(ctx, helper, args...)
	cmd.Stderr = os.Stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	for {
		frame := make([]byte, format.FrameBytes())
		if _, err = io.ReadFull(pipe, frame); err != nil {
			break
		}
		if err = offerVideo(ctx, frames, frame); err != nil {
			break
		}
	}
	// Kill before Wait on cancellation or an unexpected partial frame.
	if ctx.Err() != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(waitErr, &exit) {
		switch exit.ExitCode() {
		case 2:
			return errors.New("Allow BlipCapture screen recording in System Settings, then try again.")
		case 3:
			return errors.New("Open a Firefox video in Picture-in-Picture, then swipe right again.")
		}
	}
	if waitErr != nil {
		return fmt.Errorf("Firefox capture stopped: %w", waitErr)
	}
	return errors.New("Firefox Picture-in-Picture capture ended.")
}

func testFrames(ctx context.Context, frames chan []byte, format video.Format) error {
	tick := time.NewTicker(time.Second / 12)
	defer tick.Stop()
	for phase := 0; ; phase++ {
		frame := make([]byte, format.FrameBytes())
		for y := range format.Height {
			for x := range format.Width {
				r := uint16(((x + phase*3) % format.Width) * 31 / (format.Width - 1))
				g := uint16(y * 63 / (format.Height - 1))
				b := uint16((x + y + phase) % 32)
				binary.BigEndian.PutUint16(frame[(y*format.Width+x)*2:], r<<11|g<<5|b)
			}
		}
		if err := offerVideo(ctx, frames, frame); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func writeSerial(ctx context.Context, p io.Writer, data []byte) error {
	for len(data) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := p.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func videoMessage(ctx context.Context, p io.Writer, message string) error {
	log.Print(message)
	b, err := json.Marshal(struct {
		Message string `json:"videoMessage"`
	}{message})
	if err != nil {
		return err
	}
	return writeSerial(ctx, p, append(b, '\n'))
}

func (v *videoSession) report() {
	now := time.Now()
	if now.Sub(v.reported) < 5*time.Second {
		return
	}
	log.Printf("Video: %.1f displayed fps (%dx%d RGB565, scaled to 320x240)", float64(v.count-v.reportCount)/now.Sub(v.reported).Seconds(), v.format.Width, v.format.Height)
	v.reportCount, v.reported = v.count, now
}
