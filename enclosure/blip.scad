// blip: Brian's Little Information Panel
// Original enclosure, MIT. Units: mm. See README for unverified fit parameters.
// Print front face-down as generated; back flat as generated.
part = "assembly"; // [assembly,front,back,fit,encoder-fit]
$fn = 48;
width = 115;
height = 76;
depth = 32;
wall = 2.4;
face = 2;
corner_radius = 5;
clearance = 0.3;
// Adafruit V2 PCB in landscape: 64.770 x 52.578, holes 59.690 x 47.498.
board_y = 12;
board_w = 64.770;
board_h = 52.578;
hole_inset = 2.540;
post_radius = 3.2;
post_inward = 2.0; // Relieve LCD-facing sides; retain 1.15 mm around M2 pilot.
// Distance from front exterior to screen-facing PCB surface. Verify on assembly.
board_z = 7.5 - 0.1*25.4; // Physical fit correction: 2.54 mm closer to face.
window_w = 50.5;
window_h = 38.9;
window_x = 8 + 34.20; // Opening stays fixed; only the board mounts move.
window_y = board_y + 26.34;
// Mirror the original PCB mounting pattern about the fixed window center.
board_x = 2*window_x - 8 - board_w;
knob_x = 94;
knob_y = 38;
shaft_hole = 7.4;
// PEC11+SWITCH body footprint from Adafruit encoder PCB, rotated 45 degrees.
// Short rails locate the body; shaft nut retains it axially. Verify actual fit.
encoder_body_h = 13.2;
encoder_gap = 0.3; // Per-side clearance.
encoder_angle = 45;
encoder_rail_length = 8;
encoder_rail_wall = 1.8;
encoder_rail_height = 3;
board_screw_pilot = 1.7; // M2 board screws; tune after fit print.
case_screw_pilot = 2.6; // M3 case screws form threads in plastic.
case_screw_clearance = 3.3; // Lid: M3 screws pass freely into shell posts.
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
module display_posts() {
 for(x=[hole_inset,board_w-hole_inset],y=[hole_inset,board_h-hole_inset])
  translate([board_x+x,board_y+y,0]) intersection() {
   cylinder(h=board_z,d=2*post_radius);
   // Clip both inward-facing sides at each corner, preserving hole centers.
   translate([x<board_w/2 ? -post_radius : -post_inward,
              y<board_h/2 ? -post_radius : -post_inward,0])
    cube([post_radius+post_inward,post_radius+post_inward,board_z]);
  }
}
module encoder_index() {
 translate([knob_x,knob_y,0]) rotate([0,0,encoder_angle])
  for(side=[-1,1])
   translate([-encoder_rail_length/2,
    side>0 ? encoder_body_h/2+encoder_gap : -encoder_body_h/2-encoder_gap-encoder_rail_wall,
    face-0.1])
    cube([encoder_rail_length,encoder_rail_wall,encoder_rail_height+0.1]);
}
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
   display_posts();
   encoder_index();
   screw_positions() cylinder(h=depth-back_thickness,d=8);
  }
  translate([window_x-window_w/2,window_y-window_h/2,-1])
   cube([window_w,window_h,face+2]);
  translate([knob_x,knob_y,-1]) cylinder(h=face+2,d=shaft_hole);
  // Blind pilot holes leave the exterior face intact; no inserts or nuts.
  display_positions()
   translate([0,0,face]) cylinder(h=board_z+1,d=board_screw_pilot);
  screw_positions()
   translate([0,0,face]) cylinder(h=depth,d=case_screw_pilot);
  translate([usb_x,height-wall-1,usb_z]) cube([usb_width,wall+2,usb_height]);
 }
}
module back() {
 difference() {
  linear_extrude(back_thickness) outline(width,height,corner_radius);
  screw_positions() translate([0,0,-1]) cylinder(h=back_thickness+2,d=case_screw_clearance);
  // Small vent slots; the lid can be removed to reach BOOTSEL/RESET.
  for(x=[18:7:65]) translate([x,25,-1]) cube([2,20,back_thickness+2]);
 }
}
module fit() {
 // Alignment and screw-fit test using the same pilot holes as the full shell.
 intersection() {front();translate([-1,-1,-1]) cube([width+2,height+2,max(board_z,face+encoder_rail_height)+1]);}
}
if(part=="front") front();
else if(part=="back") back();
else if(part=="fit") fit();
else if(part=="encoder-fit") intersection() {
 front();
 translate([knob_x-16,knob_y-16,0]) cube([32,32,face+encoder_rail_height]);
}
else {
 color("#343c48") front();
 color("#687885") translate([0,0,depth+8]) back();
 // Illustrative component envelopes, not printable/vendor models.
 %translate([board_x,board_y,board_z]) cube([board_w,board_h,1.6]);
 %translate([knob_x,knob_y,-12]) cylinder(h=12,d=18);
}
