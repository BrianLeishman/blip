# OpenSCAD enclosure

`blip.scad` is the editable source. `stl/front.stl` and `stl/back.stl` are generated
from it. The default view is an exploded assembly. Select `part="fit"` to export
a shallow bezel/mounting test before printing the complete case.

**Prototype: not physically fit-tested.** PCB outline and mounting-hole centers
come from Adafruit's V2 Eagle file. Screen stack height, viewing-window alignment,
encoder bushing fit, cable opening, and nylon nut clearances still need checking
against the assembled hardware. Do not treat these exports as a proven fit.

## Geometry and hardware

- 115 x 76 x 32 mm case; 2.4 mm walls and a 2 mm face.
- Screen PCB: 64.770 x 52.578 mm in landscape, holes inset 2.540 mm.
- Four screen posts with side-loading M2.5 captive-nut pockets.
- Four case posts accept captive nuts; rear lid uses M2.5 screws.
- Encoder mounts through the face with its included washer/nut.
- Broad top-edge USB service opening leaves clearance for the cable; refine after fitting.
- Flat rear lid with vents; rubber feet attach to its outside.

Use the kit's 6 mm screws for initial screen/lid fitting, changing to 4 or 10 mm
as the measured stack requires. Do not tighten against the LCD glass. Nut pockets
are parametric (`nut_af`, `nut_depth`) because plastic nuts and printers vary.
The knob must retain enough axial clearance to click without rubbing the face.
The current shell is flat; an angled desk stand can be added after hardware fit.

Print front face-down (the SCAD export already places that face on Z=0), and back
flat. Suggested starting settings: 0.2 mm layers and three perimeters. The USB
opening and nut pockets involve short bridges; use supports locally if required
by your printer/material. Printer and filament are user-supplied.

```sh
make enclosure
openscad -o build/fit.stl -D 'part="fit"' enclosure/blip.scad
```

The firmware uses landscape rotation 90 degrees, placing the USB cable above
the readable screen. Confirm this orientation before committing to the case.

Dimensional sources:
- https://github.com/adafruit/Adafruit-2.4-TFT-FeatherWing-PCB
  (`Adafruit 2.4in TFT FeatherWing V2.brd`)
- https://learn.adafruit.com/adafruit-2-4-tft-touch-screen-featherwing/downloads
- https://www.adafruit.com/product/377

The vendor CAD was read for dimensions and is not included in this repository.
