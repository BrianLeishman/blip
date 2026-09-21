// blip: Brian's Little Information Panel
// Original enclosure, MIT. Units: mm. See README for unverified fit parameters.
// Print front face-down as generated; back flat as generated.
part = "assembly"; // [assembly,front,back,fit]
$fn = 48;
width = 115;
height = 76;
depth = 32;
wall = 2.4;
face = 2;
corner_radius = 5;
clearance = 0.3;
// Adafruit V2 PCB in landscape: 64.770 x 52.578, holes 59.690 x 47.498.
board_x = 8;
board_y = 12;
board_w = 64.770;
board_h = 52.578;
hole_inset = 2.540;
// Distance from front exterior to screen-facing PCB surface. Verify on assembly.
board_z = 7.5;
window_w = 50.5;
window_h = 38.9;
window_x = board_x + 34.20;
window_y = board_y + 26.34;
knob_x = 94;
knob_y = 38;
shaft_hole = 7.4;
screw_hole = 2.8;
nut_af = 5.2; // M2.5 nut across flats + printing allowance; measure nylon nuts.
nut_depth = 2.2;
back_thickness = 2.4;
// Broad service opening accommodates the Feather USB plug without a panel extension.
// Narrow/relocate after confirming the physical stack and cable boot.
usb_x = board_x + 8;
usb_width = 51;
usb_z = 10;
usb_height = 17;

module outline(w,h,r) {
 hull() for(x=[r,w-r], y=[r,h-r]) translate([x,y]) circle(r=r);
}
module screw_positions() {
 for(x=[6,width-6],y=[6,height-6]) translate([x,y,0]) children();
}
module display_positions() {
 for(x=[hole_inset,board_w-hole_inset],y=[hole_inset,board_h-hole_inset])
  translate([board_x+x,board_y+y,0]) children();
}
module nut_pocket(z) {translate([0,0,z]) cylinder(h=nut_depth+0.1,d=nut_af/cos(30),$fn=6);}
module front() {
 difference() {
  union() {
   linear_extrude(face) outline(width,height,corner_radius);
   difference() {
    linear_extrude(depth) outline(width,height,corner_radius);
    translate([wall,wall,face]) linear_extrude(depth)
      outline(width-2*wall,height-2*wall,corner_radius-wall);
   }
   // Display posts attach to the face; screws run in from the PCB side.
   display_positions() cylinder(h=board_z,d=6.4);
   screw_positions() cylinder(h=depth-back_thickness,d=8);
  }
  translate([window_x-window_w/2,window_y-window_h/2,-1])
   cube([window_w,window_h,face+2]);
  translate([knob_x,knob_y,-1]) cylinder(h=face+2,d=shaft_hole);
  display_positions() {
   translate([0,0,face]) cylinder(h=board_z+1,d=screw_hole);
   nut_pocket(face);
   // Side-loading captive nut pocket. Front remains intact.
   translate([-3.3,-nut_af/2,face]) cube([6.6,nut_af,nut_depth]);
  }
  screw_positions() {
   translate([0,0,face]) cylinder(h=depth,d=screw_hole);
   nut_pocket(depth-back_thickness-5);
   translate([-4.1,-nut_af/2,depth-back_thickness-5]) cube([8.2,nut_af,nut_depth]);
  }
  translate([usb_x,height-wall-1,usb_z]) cube([usb_width,wall+2,usb_height]);
 }
}
module back() {
 difference() {
  linear_extrude(back_thickness) outline(width,height,corner_radius);
  screw_positions() translate([0,0,-1]) cylinder(h=back_thickness+2,d=screw_hole);
  // Small vent slots; the lid can be removed to reach BOOTSEL/RESET.
  for(x=[18:7:65]) translate([x,25,-1]) cube([2,20,back_thickness+2]);
 }
}
module fit() {
 // Cheap first print: bezel and mounting geometry only, no tall case walls.
 intersection() {front();translate([-1,-1,-1]) cube([width+2,height+2,board_z+1]);}
}
if(part=="front") front();
else if(part=="back") back();
else if(part=="fit") fit();
else {
 color("#343c48") front();
 color("#687885") translate([0,0,depth+8]) back();
 // Illustrative component envelopes, not printable/vendor models.
 %translate([board_x,board_y,board_z]) cube([board_w,board_h,1.6]);
 %translate([knob_x,knob_y,-12]) cylinder(h=12,d=18);
}
