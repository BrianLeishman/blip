# V2 enclosure and component packing study

**Current full-case prototype:** [assembly and printing instructions](ASSEMBLY.md).
The notes below retain the earlier packing-study comparisons. The full case now
uses the successfully tested eight-wire relocation and an upside-down Feather.

`packing.scad` models the existing boards and provisional connector clearances to
compare layouts before designing another enclosure. **This is not a printable
case or a guarantee of physical fit.** V1 remains in `../tapered.scad`.

Open `packing.scad` in OpenSCAD. Set `layout` to `stacked`, `remote-flat`, or
`remote-edge`; `qt_source` chooses `feather` (default) or `screen`; `view` supports `assembly`, `exploded`, `collision`, and `envelope`.
Green is the screen PCB, blue the Feather, purple the encoder PCB, and amber the
reserved connector/cable space. Red indicates intersections included in the
collision check. Axes: x right, y down the display, z behind its face; the knob
projects into negative z. This is a component layout, not the tilted desktop stand.

## Published geometry

| Part | PCB outline from vendor CAD | Other published dimensions |
|---|---|---|
| [TFT FeatherWing V2 #3315](https://www.adafruit.com/product/3315) | 64.770 × 52.578 mm landscape | Product overall height 9.5 mm; LCD footprint outline 60.15 × 42.6 mm |
| [Feather RP2040 #4884](https://www.adafruit.com/product/4884) | 50.800 × 22.860 mm | Product overall height 7.5 mm, not assembled header-stack height |
| [Encoder breakout #4991](https://www.adafruit.com/product/4991) | 25.400 × 25.400 mm | Product height 4.6 mm without the separately soldered encoder |
| [Encoder + knob #377](https://www.adafruit.com/product/377) | Encoder body footprint 13.2 × 13.2 mm | Supplied knob base diameter 16 mm, height 14.5 mm |

All three PCB mounting-hole drills are 2.5 mm in the CAD. Mounting holes and
connector centers are preserved in `vendor-dimensions.json`; board outlines and
holes are exported to `dimensions.scad`. Dimensions are in millimeters.

Sources: Adafruit's [TFT CAD](https://github.com/adafruit/Adafruit-2.4-TFT-FeatherWing-PCB),
[Feather CAD](https://github.com/adafruit/Adafruit-Feather-RP2040-PCB), and
[encoder breakout CAD](https://github.com/adafruit/Adafruit-I2C-QT-Rotary-Encoder-PCB),
plus the linked [knob drawing](https://cdn-shop.adafruit.com/datasheets/EPD-200732.pdf)
and [PEC11 drawing](https://cdn-shop.adafruit.com/datasheets/pec11.pdf).
These are simplified envelope models, not detailed manufacturer STEP models.

## Assumptions that still need checking

- PCB thickness 1.6 mm; LCD depth 3.4 mm; rear socket height 4.5 mm. These split
  the screen's published 9.5 mm total but do not establish the real Z positions.
- Feather components occupy a conservative full-board 5.9 mm box above its PCB.
  Soldered male headers, pin tails, adapter sockets and ribbon termination heights
  are not individually modeled. Thus the stacked Z placement is provisional too.
- Encoder assembly depth is 12 mm from the modeled body front to the PCB rear.
  The knob's external dimensions are known; bushing/shaft engagement and face
  thickness must be resolved when making the case.
- QT plugs plus wire bends reserve 8 mm outside a board edge. The screen plug
  deliberately has its own space above the PCB edge. Actual cable paths are not
  modeled; the two plugs must still be joined without pinching the cable.
- USB male plug reserves 15.24 mm beyond the Feather, based on the V1 clearance
  report. Its box does not reproduce the specific right-angle plug shape.
- The panel USB socket is a **16 × 12 × 10 mm placeholder**, not a selected product.
  It excludes mounting flanges and screws; select a real data-capable panel cable
  and replace this envelope before creating its cutout.
- The remote Feather has 3 mm nominal wire space behind the encoder envelope.
  A real header/ribbon adapter may need appreciably more room.

## Layouts and verification

Run `make enclosure-v2-check` to export envelopes and intersections into
`build/v2/` and write `results.json`. Only OpenSCAD and Python's standard library
are required; generated dimensions are already included in the repository.

- **Stacked:** conventional Feather behind the FeatherWing, USB extending past
  the lower board edge.
- **Remote flat:** Feather stays parallel to the LCD but is relocated and rotated
  so USB points toward the encoder bay. This tests rerouting, not a base-mounted
  Feather parallel to the desk.
- **Remote edge:** Feather turns 90 degrees behind the screen. This trades depth
  for placement freedom and is not automatically slimmer.
- **Remote close:** same flat position with `board_gap=-2`, allowing the encoder
  PCB to overlap the display PCB's XY outline by 2 mm at a different depth. This
  tests whether the hidden encoder body, rather than its PCB outline, limits spacing.

Initial results with the encoder cable on the **screen** (superseded by Feather routing, 2026-09-24):

| Candidate | Modeled envelope, W × H × D (mm) | Modeled intersection |
|---|---|---|
| Stacked | 92.17 × 75.82 × 30.40 | Yes: reserved screen QT bend space overlaps Feather |
| Remote flat | 92.17 × 64.58 × 30.40 | None |
| Remote edge | 101.41 × 64.58 × 41.26 | None |
| Remote close | 88.17 × 64.58 × 30.40 | None |

The close variant moves the knob 4 mm inward relative to the flat baseline. Its
estimated encoder-body-to-screen-PCB edge gap is only about 1.37 mm; add mounting
and indexing geometry before relying on that spacing. The generic panel socket
sets the 30.4 mm depth in the flat variants, so choosing the real socket matters.
The stacked intersection involves a provisional **bend clearance**, not proof
that the actual assembled boards intersect.

Envelope sizes exclude the external knob, case walls, bezel, stand, fasteners,
wire harness and strain relief. They include the provisional cable/socket boxes.
Do not compare them directly with V1's complete case dimensions.

Collision checks compare the display/encoder groups against the Feather and USB
clearance, the display against the encoder, and the panel socket against those
parts. They also include the modeled QT clearance boxes. An empty intersection
means **no collision among these modeled envelopes**, not a validated assembly.
Intended internal contact within each component group is excluded. Actual cable
bends, mounting posts, tool access and electrical behavior remain untested.

Updated results with the encoder connected to the **Feather**:

| Candidate | Modeled envelope, W × H × D (mm) | Modeled intersection |
|---|---|---|
| stacked-close | 88.17 × 74.04 × 30.40 | None |
| stacked | 92.17 × 74.04 × 30.40 | None |
| remote-flat | 93.19 × 56.58 × 30.40 | None |
| remote-edge | 101.41 × 56.58 × 41.26 | None |
| remote-close | 89.19 × 56.58 × 30.40 | None |

The stacked model still reserves space beyond the Feather for its QT plug/bend;
that space is behind the display plane, so the final shell can accommodate it
without necessarily increasing the front bezel. Physical routing must confirm this.

## Direction for the next case

First try the existing **stacked Feather**, with the encoder cable connected to the
Feather's own STEMMA QT port. Both boards' CAD files connect QT pins 3/4 directly
to SDA/SCL on their Feather headers; `firmware/main.go` already initializes
`machine.I2C1` with `machine.SDA_PIN` and `machine.SCL_PIN`. No firmware change is
needed. Power off before moving the cable. Touch remains connected over the headers.
Adafruit confirms the shared bus in its [Feather pinout guide](https://learn.adafruit.com/adafruit-feather-rp2040-pico/pinouts).

The study now reserves QT plug/bend space at the Feather port instead of at the
screen port. This removes the plugged cable from the LCD edge; it does not remove
the bare screen socket or the need for cable space elsewhere. Test the real bend
behind the Feather before fixing case height. The riser remains an optional second
step if USB/depth still calls for it. The flat remote arrangement remains a candidate,
while standing the Feather on edge costs depth in the current study.

The proposed "riser" is a short Feather-specific harness/adapter, **not PCIe**.
The two rows have 16 and 12 positions at 2.54 mm pitch. A one-to-one connection
preserves the existing wiring, but its orientation and pin mapping must be checked
against both boards before power. No ready-made compatible riser has been verified.
Ordinary jumper/socket housings may erase the space savings. A short custom ribbon
adapter is a candidate; SPI signal quality must be checked on hardware, and the
current display clock may need reducing. CAD cannot establish that.

Next inputs: actual panel-mount USB cable geometry, riser termination dimensions,
and eventually assembled stack/QT-bend measurements. Then add the face, indexing
features, mounting posts and sloped base around the chosen component arrangement.

## Regenerating the dimension extraction

`scripts/v2-dimensions.py` expects these vendor files (not needed for normal renders):

- `build/tft-cad/Adafruit 2.4in TFT FeatherWing V2.brd`
- `build/feather-cad/Adafruit Feather RP2040.brd`
- `build/encoder-cad/Adafruit I2C QT Rotary Encoder.brd`

Download from the linked vendor repositories and run `python3 scripts/v2-dimensions.py`.
The extraction records XY facts only; it does not infer component heights.


## First print: closer-knob front fit

[`front-fit.scad`](front-fit.scad) produces
[`../stl/v2-front-fit-m25.stl`](../stl/v2-front-fit-m25.stl). This is a shallow,
open-backed mounting coupon, not a replacement case. It preserves V1's corrected
screen-window offset, 2.2 mm M2.5 pilot holes and
encoder indexing rails. Screen supports are now **5.468 mm** high: the original
4.96 mm plus 0.020 inch (0.508 mm), following physical fit feedback that the
shorter supports compressed the LCD. The screen orientation stays the same as the assembled
V1; the knob remains on the right from the readable side.

- Overall 93.57 × 61 × 5.468 mm; face is 2 mm thick.
- Knob center is 9.76 mm closer to the screen PCB than in `tapered.scad`.
- Encoder PCB overlaps the screen PCB outline by 2 mm at different heights.
- Existing M2.5 × 4 mm screen screws and the encoder's nut/washer fit the intended
  mounting scheme. No threaded inserts. The pilot bottoms are 3.468 mm below the
  support surface; do not substitute long screws that could pierce the face.
- Print broad flat face down, posts/rails up, 0.2 mm layers and 3 walls. No supports
  are intended. Use the same material as the successful V1.
- With USB unplugged, fit the screen and encoder without forcing either. Check
  the four supports seat, LCD opening aligns, the encoder body fits the rails,
  board backs/solder joints do not touch, and knob turns/presses freely. Move the
  QT cable to the Feather and check its bend with the Feather still stacked.
- This coupon has no rear walls or case fasteners. It cannot validate USB routing,
  a future lid/stand, or a Feather riser. All electronic assembly Z envelopes remain
  estimates; the physical coupon is intended to resolve the closer-knob fit.

Regenerate with `make enclosure-v2-fit`. `part="preview"` shows component proxies;
`part="clearance"` intersects the coupon with rigid board/body proxies and a 6 mm
screen-screw access envelope. On the first export the clearance intersection was
empty, and the STL was one connected watertight mesh with its minimum Z at zero.

## Panel USB candidates (not selected)

[DataPro 1678](https://www.datapro.net/products/usb-c-panel-mount-extension-cable.html)
has a 6-inch option, USB 2.0 compatibility, included 4-40 mounting screws and a
published drawing/STEP model. Its flange, round cable and plug need checking against
the compact layout; the current generic panel socket is not a model of this cable.
[Adafruit 6069](https://www.adafruit.com/product/6069) is another round panel-mount
option. Neither has been selected or purchased. A printed capture bracket for the
user's existing flexible extension is also worth evaluating before adding a cable.

The taller-post revision is also exported as `../stl/v2-front-fit-m25-taller-posts.stl`
to distinguish it from the earlier 4.96 mm-support print.
