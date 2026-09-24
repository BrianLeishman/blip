# OpenSCAD enclosure

`blip.scad` is the editable source. `stl/front.stl` and `stl/back.stl` are generated
from it. The default view is an exploded assembly. Select `part="fit"` to export
a shallow bezel/mounting test before printing the complete case. The fit test
uses 1.7 mm board pilot holes for M2 screws and 2.6 mm case pilot holes for
M3 screws to form threads directly in printed plastic. No inserts or captive nuts are needed. Test screw fit gently
before mounting the electronics; adjust `board_screw_pilot` or `case_screw_pilot` if your print is too tight
or too loose, especially with nylon screws. Do not force a screw that binds.

**Prototype: not physically fit-tested.** PCB outline and mounting-hole centers
come from Adafruit's V2 Eagle file. Screen stack height, viewing-window alignment,
encoder bushing fit, cable opening, and screw fit still need checking
against the assembled hardware. Do not treat these exports as a proven fit.

## Geometry and hardware

- 115 x 76 x 32 mm case; 2.4 mm walls and a 2 mm face.
- Screen PCB: 64.770 x 52.578 mm in landscape, holes inset 2.540 mm.
- Screen mounting pattern is mirrored horizontally about the unchanged window
  center, swapping the close and far pairs. This moves the PCB origin from
  x=8 to x=11.63 mm while retaining the mounting-hole spacing.
- Four screen posts with M2 pilot holes. LCD-facing sides are trimmed to
  2.0 mm from each screw center; the outer sides retain the original radius.
- Four case posts with M3 pilot holes; rear lid has 3.3 mm clearance holes.
- Encoder mounts through the face with its included washer/nut. Two 3 mm
  tall rails locate its metal body against rotation. Their 13.8 mm spacing
  includes 0.3 mm clearance per side around the nominal 13.2 mm body.
  Rails are rotated 45 degrees to match the encoder-to-breakout orientation;
  verify fit before tightening. Adjust `encoder_gap`/`encoder_angle` if needed.
- Broad top-edge USB service opening leaves clearance for the cable; refine after fitting.
- Flat rear lid with vents; rubber feet attach to its outside.

The revised screen posts are 2.54 mm shorter, leaving 2.96 mm of blind-hole
depth. Use M2 x 4 mm screws with a 1.6 mm PCB (2.4 mm engagement), or check
your actual stack so screw protrusion stays below 2.96 mm. The earlier
6 mm screen screws are too long without suitable spacers.
The rear lid is 2.4 mm thick; a 6 mm screw leaves 3.6 mm engagement in the shell.
Do not tighten against the LCD glass.
The knob must retain enough axial clearance to click without rubbing the face.
The current shell is flat; an angled desk stand can be added after hardware fit.

Print front face-down (the SCAD export already places that face on Z=0), and back
flat. Suggested starting settings: 0.2 mm layers and three perimeters. The USB
opening involves a bridge; use supports locally if required
by your printer/material. Printer and filament are user-supplied.

```sh
make enclosure
openscad -o build/fit.stl -D 'part="fit"' enclosure/blip.scad
```

Physical fit feedback: with the screen readable, USB exits below it. The
current full shell's top service opening is therefore not validated and must
be rerouted before a full-case print. The shallow fit test does not depend on
USB opening position.

Use `stl/bezel-fit-mirrored-short.stl` for the revised combined fit test.
Previous fit exports are superseded. For a smaller encoder-only trial, export
`part="encoder-fit"`; it checks body indexing and knob click clearance.

Dimensional sources:
- https://github.com/adafruit/Adafruit-2.4-TFT-FeatherWing-PCB
  (`Adafruit 2.4in TFT FeatherWing V2.brd`)
- https://learn.adafruit.com/adafruit-2-4-tft-touch-screen-featherwing/downloads
- https://www.adafruit.com/product/377

The vendor CAD was read for dimensions and is not included in this repository.

Encoder dimensions: [Adafruit encoder PCB](https://github.com/adafruit/Adafruit-I2C-QT-Rotary-Encoder-PCB),
`PEC11+SWITCH` package (13.2 mm body outline, center-mounted at MR135).

## Compact enclosure with 45-degree stand

`slim.scad` is the current enclosure. `blip.scad` retains the larger fitting
reference. The face is 104 x 61 mm, with the corrected screen offset and
4.96 mm PCB mounting height. Shell plus rear cover depth is 32 mm. The stand
holds the face at 45 degrees; assembled desk footprint is approximately
104 x 68 mm, about 70 mm high, excluding rubber feet.

Build all current parts with `make enclosure-slim`:

- `stl/slim-front-m25.stl`: front shell, relieved screen posts, encoder rails.
- `stl/slim-stand.stl`: hollow stand and rear cover, printed as one part.
- `stl/slim-usb-plate.stl`: replaceable rear cable plate.
- `stl/slim-face-fit-m25.stl`: optional shallow fit print using the same mounts.

Hardware:

- Four M2.5 x 4 mm pan/button-head screws for the screen PCB. Vendor CAD
  specifies 2.5 mm drilled mounting holes; check free passage on the actual
  plated board. Printed pilot holes are 2.2 mm. The nominal 1.6 mm PCB leaves
  2.4 mm screw engagement in 2.96 mm-deep blind holes. Do not use 8 mm screws.
- Four M3 x 8 mm screws fasten the stand/rear cover to the shell (2.4 mm cover,
  5.6 mm engagement). Four 7.5 mm access bores in the stand admit the heads
  and a slim driver. Two are reached from the back, two from underneath.
- Two M3 x 6 mm screws fasten the rear cable plate. A small zip tie secures
  the extension body using the adjacent slots; do not clamp bare ribbon wire.
- Encoder's supplied nut and washer, plus the existing rubber feet.
- No inserts or captive nuts required.

Assembly: fit the screen using the short screws, install the encoder between
its indexing rails and secure its nut, connect QT and USB, route the USB
extension through the 24 x 12 mm rear-cover transfer opening into the hollow
stand, and attach the stand using the four M3 screws. The cover has an inset
locating skirt. Secure the extension at the rear plate, attach the plate,
and place rubber feet on the four solid underside corner areas. The large
underside opening provides access to wiring and the lower cover screws.

USB routing is designed for a short flexible **data-capable** USB-C male-to-
female extension, preferably with a low-profile right-angle male connector.
The current rear plate has a generic 18 x 10 mm pass-through and tie slots;
it is not a precision panel mount for an identified cable. Adjust `usb_slot_w`
and `usb_slot_h`, or replace just this plate, once the extension is selected.
The lower shell service opening accommodates the plug; exact connector/body
clearance and cable bend radius remain physical-fit checks. The QT board gap
is nominally 4.13 mm, so check the QT connector/cable clearance too.

Print the front face down, the stand base down, and the cable plate flat,
as exported. Start with 0.2 mm layers and 3-4 perimeters. Inspect the slicer:
local supports may be useful under the shell USB opening and the stand's
rear cable opening; avoid filling the entire hollow stand with supports.
The sloped roof starts on a supporting front lip. All parts remain prototypes
until physically assembled; rendering alone does not validate hardware fit.

## Tapered cable-pocket revision (September 23)

`tapered.scad` is a separate fit prototype; `slim.scad` remains unchanged.
The user confirmed USB currently exits the top with the knob on the right.
This revision rotates the PCB mount pattern 180 degrees about the unchanged
screen opening so USB exits below the display. The front shell has been physically installed and the user reports it clears,
with little spare room. Firmware now uses Rotation270 with the matching
touchscreen mapping for this orientation. Base fit is still pending.

The front stays 104 x 61 mm with the same screen opening and knob placement.
Behind it, the case flares to 116 x 76.24 mm: 6 mm per side and an additional
15.24 mm (0.600 inch) below the original lower inner wall. Side relief reaches
full width at the 4.96 mm PCB plane; lower relief reaches full size 12 mm behind
the front face. These are provisional cable envelopes, not measured plug models.
The lower pocket routes through a 40 x 18.24 mm opening into the hollow stand.
The stand matches the enlarged rear cover and still holds the face at 45 degrees.
The underside is raised to keep the enlarged shell above the desk.

Build with `make enclosure-tapered`:

- `stl/tapered-front-m25.stl`
- `stl/tapered-stand.stl`
- `stl/tapered-face-fit-m25.stl` (optional shallow mounting check)

Reuse `stl/slim-usb-plate.stl` and the M2.5 x 4 mm / M3 hardware described above.
The left PCB bosses are clipped at the front edge after rotation; no inserts.
The rear connector plate remains a generic opening, pending actual female
connector measurements. Do not assume it is a precise clamp for the new cable.

Print front face-down with supports as needed under the outward flare (the
side flare is steep); stand base-down. Inspect support access in the slicer.
Verify USB connector depth, bend clearance, QT routing, and the rotated screen
alignment before a final print. The shallow coupon checks mounting, not cables.

## V2 packing study

The [V2 component study](v2/README.md) uses vendor PCB outlines and explicit
connector-clearance estimates to compare stacked and relocated Feather layouts.
Open `v2/packing.scad` to inspect; run `make enclosure-v2-check` for envelope and
collision reports. This is separate from the assembled V1 tapered enclosure.

## Compact V2 full assembly

See [V2 assembly instructions](v2/ASSEMBLY.md) for the 97.57 × 65 × 28 mm case,
upside-down relocated Feather and required low-profile wiring. Run
`make enclosure-v2`; `stl/v2-print-layout.stl` contains all three printable parts.
