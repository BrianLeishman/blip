# Compact V2 enclosure

This is the complete **first full-case prototype** around the physically successful
V2 front fit. The screen/window alignment, closer knob, indexing rails and taller
5.468 mm screen posts share the same source geometry as the confirmed fit coupon.
The rear shell, Feather mounting and final cable routing have not yet been printed
and physically verified.

## Physical build feedback — 2026-09-24

The user assembled the preceding printed case and reports that Blip is together
and working. Slightly flexing the plastic let the rotary board fit, but it remains
tight. The subsequent rear-wall and Feather pin-slot revisions in the current
STLs have **not** been printed; the working assembly does not validate those changes.

For the next mechanical revision, the user requests **four Feather mounting posts
instead of two, with slightly more height**. The wiring beneath the Feather is
crowded, and the pins were bent inward; outward routing may help. The required
height increase has not been measured. These mounting changes are still pending:
the current model retains two 4.3 mm posts and the new pin-clearance slots. Any
height change must preserve clearance to the LCD/socket assembly.

## Files and printing

- [`../stl/v2-print-layout.stl`](../stl/v2-print-layout.stl): all three separate parts,
  arranged with their intended print faces on Z=0. Approximate footprint 199 × 96 mm.
- [`../stl/v2-front-m25.stl`](../stl/v2-front-m25.stl): flat front plate and mounting
  posts, face down. The enclosure sidewalls are on the rear piece.
- [`../stl/v2-stand.stl`](../stl/v2-stand.stl): rear shell plus hollow 45-degree stand,
  base down. Includes the sidewalls and relocated Feather's two mounting posts.
- [`../stl/v2-usb-plate.stl`](../stl/v2-usb-plate.stl): removable rear cable plate, flat.
- [`assembly.scad`](assembly.scad): full parametric source. Default is the assembled
  preview. `part="internals"` shows the approximate electronic component layout.
- [`front-geometry.scad`](front-geometry.scad): shared, successful front geometry.

Generate all print STLs with `make enclosure-v2`. The combined STL is a **print
layout**, not a fused assembled object. Split into objects in the slicer if printing
parts separately or assigning support settings. `part="assembled"` is for inspecting
the assembled case; do not print that mode as a single fused object.

Use 0.2 mm layers and 4 wall loops with a nominal 0.4 mm nozzle. No supports should
be needed on the front or USB plate. Inspect the stand's sloping cover, Feather
posts and USB opening in the slicer; use local removable supports if needed, with
access through the open underside. Keep supports out of small screw pilots.

## Size and layout

The outer front/rear footprint is **97.57 × 65 mm**, and the housing thickness
normal to the screen is **28 mm**. V1's widened rear envelope was 116 × 76.24 × 32 mm.
Those dimensions exclude the angled stand and knob. The stand holds the screen at
45 degrees and retains underside access, driver bores and flat base areas for feet.
Its lower footprint tucks inward 4 mm per side relative to the test bezel (6 mm
relative to the expanded rear shell) and 3 mm at the rear relative to the original
stand, with 8 mm
footprint corner radii and a 1.2 mm bottom roundover. The rounded, tapered shell
blends into a flat rear USB-plate landing. The successful front geometry, rear-cover
interface, Feather mounts and USB plate are unchanged by the base-shape revision.
The internal cavity follows the taper, retaining a nominal 3 mm wall allowance.

### Rotary clearance correction

The original full case put 1.6 mm walls inside the successful 93.57 × 61 mm test
bezel footprint. Its nominal 0.4 mm clearance beside the rotary PCB proved too
tight in the real assembly. The replacement moves **all enclosure sidewalls onto
the rear shell and outside the complete test-bezel outline**: 0.4 mm outward
clearance plus 1.6 mm wall thickness. The front gains only a flat 2 mm perimeter
flange; its screen posts, opening, encoder rails and shaft position are unchanged.
The inward registering skirt is removed. The PCB's nominal right-edge clearance
is now 2.4 mm, and the rear wall stops 0.15 mm short of the front flange.

Use the corrected front and rear STLs together; this changes the part split and
outer footprint, so it is not a front-only replacement for the previous rear shell.
The four case screw posts and M3 fasteners retain their positions and lengths.

The Feather is **unplugged from the screen sockets**, rotated so USB points toward
the knob side, and flipped with its **component side toward the LCD and pin side
facing the rear cover**. In the front-face-down coordinate system its pin-side PCB
surface is Z=21.3 mm. Two M2.5 mounts descend from the rear cover to that surface.
The PCB is supported on its pin side; screw heads go on its component side.
Assemble the Feather onto the rear cover before closing the shell.

The **4.3 mm mount height alone does not clear full-length headers or the battery
JST socket**. Two slots now let the header rows project through the rear cover
into the hollow stand. Keep **pins toward the base, components/battery socket
toward the LCD**; the opposite orientation is not the validated arrangement.
The reserved pin projection is **10 mm from the PCB**, an allowance rather than
a measurement of this user's headers. These openings preserve the Feather's
position and the screen clearance instead of lengthening the posts toward the LCD.

The closest modeled Feather components are 2.232 mm behind the screen sockets.
There is a **2 mm low-profile wiring envelope above those sockets**, and a **3 mm
wiring envelope above the Feather's pin-side surface**, supplemented by the two
10 mm header-row allowances through the rear cover. A further 1.3 mm remains
between the general wire envelope and the rear cover. These are clearance budgets,
not measurements of the existing jumper housings.

**The 0.700-inch jumper housings are still outside the wiring allowance.** The
Feather's pins can use the new slots if their projection is at most 10 mm; the
screen connections still require low-profile wiring. Route wires
flat around the board edges and keep joints insulated; do not rely on the lid to
press the wiring down. Removing unused header plastic/sockets
is optional if more routing room is wanted, not a prerequisite assumed by the CAD.

## Hardware and assembly

- Four M2.5 × 4 mm screws for the screen, in 2.2 mm plastic pilot holes.
- Two M2.5 × 4 mm screws for the relocated Feather, also in 2.2 mm pilots.
- Four M3 × 8 mm screws for the case, through 3.3 mm cover holes into 2.6 mm pilots.
- Two M3 × 6 mm screws for the rear USB plate.
- Existing encoder nut/washer, flexible USB extension, a small cable tie, and feet.
- No thread inserts required. No nylon standoff length is assumed.

1. Fit the screen and encoder onto the flat front plate using the already verified
   orientation. Keep the encoder's QT lead routed inward toward the Feather.
2. Prepare and verify the compact eight-wire harness below, with power unplugged.
   Test the electronics before final fastening.
3. Fit the upside-down Feather onto the two rear-cover posts, guiding its header
   rows through the clearance slots. The metal USB socket
   points toward the knob side. Check the actual connector body and solder joints
   against the modeled clearances before tightening.
4. Keep the encoder QT cable on the **Feather**, not the screen. Route its slack
   and the USB extension through the 18 × 24 mm transfer opening into the hollow
   stand, away from case screw posts and the display.
5. Fasten the case through the rear/underside access bores. Secure the extension
   at the rear plate and attach the plate. Add the feet.

The USB plate deliberately reuses V1's generic 18 × 10 mm pass-through and tie slots.
It accommodates the existing extension; it is **not a dimensioned snap-fit mount**
for its female housing. A later true panel connector can use a revised plate alone.
USB plug projection uses the earlier 15.24 mm allowance; its 11 × 5.5 mm cross-section
is still an estimate. Actual cable bend radius and free slack are not proven by CAD.

## Eight-wire connection

Feather to matching FeatherWing header socket: **3.3V, GND, SCK, MOSI (MO), D9,
D10, SDA, SCL**. D9 is TFT chip select; D10 is data/command. The current firmware
writes the display without MISO and polls touch without the IRQ pin. Leave the
unused SD card out. RST can be added to preserve the Wing reset button; EN is
needed only if retaining its power switch behavior.

**Screen power uses the second socket from RST on its 16-position header.** The
third is AREF, although the corresponding Feather RP2040 position is another 3.3V
pin. The initial remote-wiring failure was fixed by moving power to the correct
screen socket. The user then confirmed that the display, touch and encoder worked.
See [Adafruit V2 pinouts](https://learn.adafruit.com/adafruit-2-4-tft-touch-screen-featherwing/pinouts-v2)
and the vendor CAD linked in [README.md](README.md).

## Verification scope

The exports passed `python3 scripts/v2-mesh-check.py`: each individual part is a
connected watertight mesh, and the combined layout contains exactly three parts,
all touching Z=0. Full CGAL renders of `hardware-clearance` and `case-clearance`
were empty (no modeled intersections).

The model checks rigid screen/encoder envelopes, the dual-row socket outlines,
Feather PCB and approximate components, reserved low-profile wire volumes, QT/USB
plug volumes, mounts, screw-head clearance and case-to-case intersections. Intended
mount contact is excluded. Component heights, individual cable routing, wire bend
radii and the final cable termination details are estimates; this is not an electrical
or complete mechanical simulation. The first full enclosure still needs a real fit.
