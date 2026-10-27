package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/BrianLeishman/blip/internal/video"
)

func TestVideoFrameQueueDropsBacklogAndCancels(t *testing.T) {
	frames := make(chan []byte, 1)
	for i := range 10 {
		if err := offerVideo(t.Context(), frames, []byte{byte(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := <-frames; !bytes.Equal(got, []byte{9}) {
		t.Fatalf("stale frame: %v", got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := offerVideo(ctx, frames, []byte{10}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(frames) != 0 {
		t.Fatal("enqueued after cancellation")
	}
}

type partialVideoWriter struct{ bytes.Buffer }

func (w *partialVideoWriter) Write(p []byte) (int, error) {
	if len(p) > 7 {
		p = p[:7]
	}
	return w.Buffer.Write(p)
}

type zeroVideoWriter struct{}

func (zeroVideoWriter) Write([]byte) (int, error) { return 0, nil }
func TestVideoWritesHandlePartialWritesAndCancellation(t *testing.T) {
	var out partialVideoWriter
	data := bytes.Repeat([]byte{0, 10, 255}, video.TileBytes)
	if err := writeSerial(t.Context(), &out, data); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatal("truncated video packet")
	}
	if err := writeSerial(t.Context(), zeroVideoWriter{}, data); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out.Reset()
	if err := writeSerial(ctx, &out, data); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("wrote after cancellation")
	}
}
