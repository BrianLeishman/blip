# Experimental video mode

Blip can mirror a Firefox Picture-in-Picture window over its existing USB cable.
The Mac captures and scales the picture; the Feather paints small RGB tiles. Audio
continues playing on the Mac. This does not move or pause the original video.

## Setup on macOS

Requires macOS 14 or later and the Swift compiler supplied with Apple's Command
Line Tools. Build the companion, capture helper, and firmware:

```sh
make build/blip capture-helper firmware
make flash
```

The capture helper is built locally as `build/BlipCapture.app`. It lives next to
`build/blip`; keep them together. On the first attempt, allow screen recording for
BlipCapture (or the launching application macOS identifies) in **System Settings
→ Privacy & Security → Screen & System Audio Recording**. Retry after granting
permission. A locally signed helper can need removing and re-adding to this list
after it is rebuilt, because its code signature changes. No system audio or
microphone capture is requested.

1. Open a Firefox video in **Picture-in-Picture** and leave that window open.
2. Swipe **right** across Blip to start mirroring.
3. Swipe **left**, or click the knob, to return to the dashboard.

Vertical swipes still scroll the dashboard. Horizontal gestures lock to their own
axis so switching modes doesn't also scroll. The dashboard keeps its selection;
the companion continues fetching GitHub while video is displayed and sends the
latest dashboard when you return.

Only Firefox windows with a Picture-in-Picture title are eligible. The helper
never falls back to capturing an entire display or a normal browser window. If
multiple PiP windows exist, it selects the one with the lowest window ID. Window
title matching currently recognizes the English PiP title. Protected video may
not be capturable. Media-key/Now Playing integration is not part of this prototype.

## Picture quality and measurement

The default favors motion: **80×60 pixels, 256 colors**, scaled 4× to fill the
320×240 panel. The optional `-video-detail` companion flag uses **160×120 pixels**,
scaled 2×, with a lower frame rate. Both modes preserve aspect ratio with black
bars. The capture helper samples at up to 12 fps; actual display speed depends on
USB, firmware, and screen throughput. The companion logs completed-frame rates. On the development Feather RP2040,
the moving USB test pattern measured about **6 fps** in smooth mode and **3 fps**
in detailed mode. These measurements exclude Firefox capture.

To benchmark the device without capture permission, stop the regular companion
first so only one process owns the serial connection, then run:

```sh
./build/blip -video-test 20s
./build/blip -video-test 20s -video-detail
```

The moving test pattern exits at the requested duration and restores the device's
stored dashboard. Restart the regular companion afterward. These are physical
device measurements, not capture or CPU-only benchmarks.

## Transport and failure behavior

The normal dashboard remains newline-delimited JSON. A NUL outside a JSON line
introduces a video packet: `00 42 4c 56 01`, little-endian 32-bit sequence, one-byte
kind, one-byte source row, little-endian CRC-16/CCITT-FALSE, then pixel bytes. Kinds
are start (1), detailed tile (2), stop (3), and smooth tile (4). CRC covers sequence,
kind, row, and pixels. Tile payloads contain three detailed rows or six smooth rows of RGB332 pixels. Start
and stop have no payload and a zero row.

Each packet fits inside TinyGo's 512-byte USB receive buffer (493 bytes maximum).
The host waits for a matching `video-ack` sequence before sending the next packet.
The device checks CRC, payload size, and row bounds before drawing, then expands
the tile to big-endian RGB565. No full-frame buffer is allocated on the Feather.
The host retains the latest queued frame rather than building up playback delay.
Paused video repeats the last picture periodically to maintain the connection.

Capture errors appear on Blip, then the device returns to the dashboard. Missing
packet acknowledgments stop the stream, and the device also returns to the
dashboard after ten seconds without video data. Returning to the dashboard stops
the helper process. No captured frames are saved to disk or sent over the network.
