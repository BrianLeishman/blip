// V2 front fit coupon. Geometry is shared with the complete enclosure.
include <front-geometry.scad>
part="fit"; // [fit,preview,clearance]
if(part=="fit") fit();
else if(part=="clearance") clearance();
else {
 color("#d6dce4") fit();
 color([0.1,0.5,0.35,0.6]) screen_pcb();
 color("#151e2b") lcd();
 color([0.5,0.3,0.7,0.6]) encoder_pcb();
 color("silver") encoder_body();
 color("#394754") translate([knob_x,knob_y,-14.5]) cylinder(h=14.5,d=16);
 color("red") clearance();
}
