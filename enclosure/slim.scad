// blip: Brian's Little Information Panel
// Original enclosure, MIT. Units: mm. See README for unverified fit parameters.
// Print front face-down as generated; back flat as generated.
part = "desk-preview"; // [desk-preview,assembly,front,stand,usb-plate,back,fit,encoder-fit]
// Compact enclosure with a screw-on hollow 45-degree stand/rear cover.
$fn = 48;
width = 104;
height = 61;
depth = 32;
wall = 2.4;
face = 2;
corner_radius = 5;
clearance = 0.3;
// Adafruit V2 PCB in landscape: 64.770 x 52.578, holes 59.690 x 47.498.

board_w = 64.770;
board_h = 52.578;
board_y = (height-board_h)/2;
board_x = 3.4;
hole_inset = 2.540;
post_radius = 3.2;
post_inward = 2.0; // Relieve LCD-facing sides; retain 0.90 mm around M2.5 pilot.
// Distance from front exterior to screen-facing PCB surface. Verify on assembly.
board_z = 7.5 - 0.1*25.4; // Physical fit correction: 2.54 mm closer to face.
window_w = 50.5;
window_h = 38.9;
window_x = board_x + board_w - 34.20; // Preserve corrected PCB/window offset.
window_y = board_y + 26.34;
knob_x = 85;
knob_y = height/2;
shaft_hole = 7.4;
// PEC11+SWITCH body footprint from Adafruit encoder PCB, rotated 45 degrees.
// Short rails locate the body; shaft nut retains it axially. Verify actual fit.
encoder_body_h = 13.2;
encoder_gap = 0.3; // Per-side clearance.
encoder_angle = 45;
encoder_rail_length = 8;
encoder_rail_wall = 1.8;
encoder_rail_height = 3;
board_screw_pilot = 2.2; // M2.5 board screws; tune after fit print.
case_screw_pilot = 2.6; // M3 case screws form threads in plastic.
case_screw_clearance = 3.3; // Lid: M3 screws pass freely into shell posts.
back_thickness = 2.4;
stand_wall = 3;
stand_base = 3;
stand_inset = 0;
stand_foot = 4;
screw_access = 7.5; // Driver/head access through stand, not through rear cover.
stand_y0 = depth/sqrt(2);
stand_y1 = (depth+height)/sqrt(2);
stand_z1 = height/sqrt(2)+stand_foot;
usb_rear_x = 38;
usb_rear_z = 18;
usb_plate_w = 48;
usb_plate_h = 26;
usb_plate_t = 2.4;
usb_slot_w = 18; // Universal pigtail opening; tailor plate to chosen extension.
usb_slot_h = 10;
usb_screw_spacing = 40;
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
 // Four posts in the electronics-free strips above/below the encoder.
 for(x=[74,width-6],y=[6,height-6]) translate([x,y,0]) children();
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
    linear_extrude(depth-back_thickness) outline(width,height,corner_radius);
    translate([wall,wall,face]) linear_extrude(depth)
      outline(width-2*wall,height-2*wall,corner_radius-wall);
   }
   // Display posts attach to the face; screws run in from the PCB side.
   display_posts();
   encoder_index();
   screw_positions() cylinder(h=depth-back_thickness,d=7);
  }
  translate([window_x-window_w/2,window_y-window_h/2,-1])
   cube([window_w,window_h,face+2]);
  translate([knob_x,knob_y,-1]) cylinder(h=face+2,d=shaft_hole);
  // Blind pilot holes leave the exterior face intact; no inserts or nuts.
  display_positions()
   translate([0,0,face]) cylinder(h=board_z+1,d=board_screw_pilot);
  screw_positions()
   translate([0,0,face]) cylinder(h=depth,d=case_screw_pilot);
  // Temporary bottom service opening; rear cable routing is still pending.
  translate([usb_x,height-wall-1,usb_z]) cube([usb_width,wall+2,usb_height]);
 }
}
module back() {
 difference() {
  union() {
   linear_extrude(back_thickness) outline(width,height,corner_radius);
   // Locating skirt fits inside shell, clear of the M3 posts.
   difference() {
    translate([wall+clearance,wall+clearance,-2]) linear_extrude(2.1)
     difference() {
      outline(width-2*(wall+clearance),height-2*(wall+clearance),2);
      translate([1.2,1.2]) outline(width-2*(wall+clearance+1.2),height-2*(wall+clearance+1.2),1);
     }
    screw_positions() translate([0,0,-3]) cylinder(h=4,d=7+2*clearance);
   }
  }
  screw_positions() translate([0,0,-3]) cylinder(h=back_thickness+4,d=case_screw_clearance);
  // Cable transfer into the hollow stand, accessible before closing the shell.
  translate([window_x-12,height-18,-3]) cube([24,12,back_thickness+4]);
  for(x=[18:7:60]) translate([x,18,-3]) cube([2,14,back_thickness+4]);
 }
}
module fit() {
 // Alignment and screw-fit test using the same pilot holes as the full shell.
 intersection() {front();translate([-1,-1,-1]) cube([width+2,height+2,max(board_z,face+encoder_rail_height)+1]);}
}
module closed_preview() {
 color("#25303c") front();
 color("#354252") translate([0,0,depth-back_thickness]) back();
 // Visual placeholders only: LCD surface and knob, not vendor models.
 color("#101c25") translate([window_x-window_w/2,window_y-window_h/2,face+0.3])
  cube([window_w,window_h,0.5]);
 color("#46596b") translate([knob_x,knob_y,-12]) cylinder(h=12,d=18);
}
// Assembly pose: readable face points forward/up at 45 degrees.
module desk_pose() {
 translate([0,height/sqrt(2),(height+depth)/sqrt(2)+stand_foot])
  rotate([225,0,0]) children();
}
module wedge() {
 translate([stand_inset,0,0]) rotate([90,0,90])
  linear_extrude(width-2*stand_inset)
   polygon([[stand_y0,0],[stand_y1,0],[stand_y1,stand_z1+0.05],
            [stand_y0,stand_foot+0.05]]);
}
module stand() {
 difference() {
  union() {
   // Rear cover joins the wedge; the M3 screws also fasten the stand.
   desk_pose() translate([0,0,depth-back_thickness]) back();
   difference() {
    wedge();
    // Hollow interior; front lip supports the start of the 45-degree roof.
    translate([stand_inset+stand_wall,stand_y0+stand_wall,stand_base])
     cube([width-2*(stand_inset+stand_wall),
           stand_y1-stand_y0-2*stand_wall,height+depth]);
    // Underside access keeps wiring reachable and avoids a solid wedge print.
    translate([10,stand_y0+7,-1]) cube([width-20,stand_y1-stand_y0-14,stand_base+2]);
   }
  }
  // Removable rear cable plate; bosses stay outside the central opening.
  translate([usb_rear_x-16,stand_y1-stand_wall-1,usb_rear_z-9])
   cube([32,stand_wall+2,18]);
  for(dx=[-usb_screw_spacing/2,usb_screw_spacing/2])
   translate([usb_rear_x+dx,stand_y1+1,usb_rear_z]) rotate([90,0,0])
    cylinder(h=stand_wall+2,d=case_screw_pilot);
  // Coaxial bores reach each rear-cover screw from below/behind the stand.
  // Start beyond the cover so its 3.3 mm clearance holes remain intact.
  desk_pose() screw_positions() translate([0,0,depth+0.1])
   cylinder(h=height*2,d=screw_access);
 }
}
module usb_plate() {
 difference() {
  linear_extrude(usb_plate_t) outline(usb_plate_w,usb_plate_h,2);
  translate([(usb_plate_w-usb_slot_w)/2,(usb_plate_h-usb_slot_h)/2,-1])
   cube([usb_slot_w,usb_slot_h,usb_plate_t+2]);
  for(dx=[-usb_screw_spacing/2,usb_screw_spacing/2])
   translate([usb_plate_w/2+dx,usb_plate_h/2,-1])
    cylinder(h=usb_plate_t+2,d=case_screw_clearance);
  // A small zip tie secures the extension body to the rear plate.
  for(dx=[-usb_slot_w/2-3,usb_slot_w/2+3])
   translate([usb_plate_w/2+dx-1,usb_plate_h/2-2,-1])
    cube([2,4,usb_plate_t+2]);
 }
}
module rear_plate_pose() {
 translate([usb_rear_x-usb_plate_w/2,stand_y1+usb_plate_t,usb_rear_z-usb_plate_h/2])
  rotate([90,0,0]) children();
}
module stand_print() {
 // Base down. The front lip supports the start of the sloped rear cover.
 stand();
}
module desk_preview() {
 desk_pose() {
  color("#25303c") front();
  color("#101c25") translate([window_x-window_w/2,window_y-window_h/2,face+0.3])
   cube([window_w,window_h,0.5]);
  color("#46596b") translate([knob_x,knob_y,-12]) cylinder(h=12,d=18);
 }
 color("#172029") stand();
 color("#354252") rear_plate_pose() usb_plate();
}
if(part=="desk-preview") desk_preview();
else if(part=="front") front();
else if(part=="stand") stand_print();
else if(part=="usb-plate") usb_plate();
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
