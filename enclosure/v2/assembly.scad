// Blip V2 compact case. All dimensions mm. MIT.
// Shared front preserves the physically tested screen/knob fit.
include <front-geometry.scad>
part="preview"; // [preview,print-layout,assembled,front,stand,usb-plate,back,section,internals,hardware-clearance,case-clearance]
depth=28; // Compact wiring required; full-height jumper housings will not fit.
wall=1.6;
shell_clearance=0.4; // Rear walls sit OUTSIDE the entire successful fit-bezel outline.
seam_gap=0.15;
back_t=2.4;
case_pilot=2.6; // M3 threads form in printed plastic.
case_clearance=3.3;
case_post_r=3.5;
stand_wall=3;
stand_base=3;
stand_foot=4;
stand_taper=4; // Per-side inset at the foot; cover interface stays full width.
stand_rear_inset=3;
stand_corner=8;
stand_roundover=1.2;
stand_y0=depth/sqrt(2);
stand_y1=(depth+height)/sqrt(2);
usb_x=36;
usb_z=18;
usb_plate_w=48; usb_plate_h=26; usb_plate_t=2.4;
usb_slot_w=18; usb_slot_h=10; usb_screw_spacing=40;
// Relocated Feather: components toward LCD, header/pin side toward rear cover.
feather_x=12; feather_y=16; feather_z=21.3; // z is the pin-side PCB surface.
feather_wire_height=3; // General wire space; header rows have deeper dedicated slots.
feather_pin_height=10; // Reserved projection from PCB into base; header length is an estimate.
// Vendor header pad extents transformed into the front's coordinates.
// Each row includes the 2.54 mm header carrier footprint.
feather_header_rows=[[feather_x+5.08,feather_y+1.27,40.64],
                     [feather_x+5.08,feather_y+21.6535,30.48]];
feather_holes_local=[[2.54,2.54],[2.54,20.32]];
module feather_pose() {
 translate([feather_x+50.8,feather_y,feather_z]) rotate([180,0,180]) children();
}
module feather_holes() {
 for(p=feather_holes_local) translate([feather_x+50.8-p[0],feather_y+p[1],0]) children();
}
module feather_mounts() {
 // Posts start at the pin-side PCB surface; holes open toward the LCD.
 feather_holes() translate([0,0,feather_z-depth+back_t])
  difference(){cylinder(h=depth-back_t-feather_z+0.1,r=2.5);translate([0,0,-0.1])cylinder(h=5,d=2.2);}
}
module feather_pcb_proxy() {
 feather_pose() difference() {
  cube([50.8,22.86,1.6]);
  for(p=feather_holes_local) translate([p[0],p[1],-0.1]) cylinder(h=1.8,d=2.5);
 }
}
module feather_components_proxy() {
 // Approximate full-board component envelope, relieved around mounting holes.
 feather_pose() difference() {
  translate([0,0,1.6])cube([50.8,22.86,5.9]);
  for(p=feather_holes_local)translate([p[0],p[1],0])cylinder(h=8,r=2.5);
 }
}
module feather_wiring_proxy() {
 // Actual wires route between these pads; this is a conservative board-wide space.
 difference() {
  translate([feather_x,feather_y,feather_z+0.01])cube([50.8,22.86,feather_wire_height]);
  feather_holes()translate([0,0,feather_z])cylinder(h=feather_wire_height+1,r=2.7);
 }
}
module feather_pins_proxy() {
 for(row=feather_header_rows)
  translate([row[0],row[1]-1.27,feather_z+0.01])cube([row[2],2.54,feather_pin_height]);
}
module feather_pin_reliefs() {
 // These pass through the rear cover into the hollow stand, never out of the case.
 // Preserve the two mount roots where the slot's clearance margin approaches them.
 difference() {
  for(row=feather_header_rows)
   translate([row[0]-0.6,row[1]-1.9,feather_z-0.1])cube([row[2]+1.2,3.8,feather_pin_height+0.2]);
  feather_holes()translate([0,0,feather_z-0.2])cylinder(h=feather_pin_height+0.4,r=2.5);
 }
}
module feather_usb_proxy() {
 // Existing flexible right-angle cable: 0.600 inch projection, estimated cross-section.
 feather_pose() translate([-15.24,5.93,0])cube([15.24,11,5.5]);
}
module feather_qt_proxy() {
 feather_pose()translate([45.8,4.255,1.6])cube([13,8,3.5]);
}
module screen_headers_proxy(extra=0) {
 // Dual-row socket outlines from the vendor footprint, not the old narrow proxies.
 translate([board_x,board_y+board_h,board_z+1.6])rotate([0,0,-90]) {
  translate([5.08,37.719-4.064,0])cube([40.64,8.128,4.5+extra]);
  translate([15.24,37.719+18.796,0])cube([30.48,8.128,4.5+extra]);
 }
}
module electronics_proxy() {
 screen_pcb();lcd();encoder_body();encoder_pcb();screen_headers_proxy(2);
 feather_pcb_proxy();feather_components_proxy();feather_wiring_proxy();feather_pins_proxy();feather_usb_proxy();feather_qt_proxy();
}

module case_positions() {
 for(x=[72.5,width-5],y=[6,height-6]) translate([x,y,0]) children();
}
module outer_outline() {offset(delta=shell_clearance+wall)outline();}
module front() {
 difference() {
  union() {
   fit();
   // Only a flat perimeter flange is added to the proven fit plate.
   linear_extrude(face,convexity=10)
    difference(){outer_outline();offset(delta=-0.1)outline();}
   case_positions() cylinder(h=depth-back_t,r=case_post_r);
  }
  case_positions() translate([0,0,face]) cylinder(h=depth,d=case_pilot);
 }
}
module back() {
 difference() {
  union() {
   linear_extrude(back_t) outer_outline();
   feather_mounts();
   // Walls belong to the rear piece. Their inside boundary clears the complete
   // test bezel footprint; no wall or registering skirt steals its flat area.
   translate([0,0,face+seam_gap-(depth-back_t)])
    linear_extrude(depth-back_t-face-seam_gap+0.1,convexity=10)
     difference(){outer_outline();offset(delta=shell_clearance)outline();}
  }
  case_positions() translate([0,0,-3]) cylinder(h=back_t+4,d=case_clearance);
  // Large wiring/service transfer into the hollow stand.
  translate([68,19,-3]) cube([18,24,back_t+4]);
  translate([0,0,-(depth-back_t)])feather_pin_reliefs();
 }
}
module desk_pose() {
 translate([0,height/sqrt(2),(height+depth)/sqrt(2)+stand_foot]) rotate([225,0,0]) children();
}
module stand_footprint(inset=0) {
 // Rounded footprint tucked under the display, with a broad flat contact ring.
 r=max(1,stand_corner-inset);
 x=stand_taper+inset;
 y=stand_y0+2+inset;
 w=width-2*x;
 d=stand_y1-stand_rear_inset-inset-y;
 hull() for(px=[x+r,x+w-r],py=[y+r,y+d-r])
  translate([px,py])circle(r=r);
}
module stand_roof(inset=0) {
 // Bury the loft inside the cover so its edge cannot form coincident sliver faces.
 // The cover itself supplies the exact outer mating outline.
 desk_pose() translate([0,0,depth-0.5]) linear_extrude(0.1)
  offset(delta=-inset-0.3) outer_outline();
}
module usb_landing() {
 // Retain a flat rear patch so the existing removable plate still seats properly.
 translate([usb_x-usb_plate_w/2,stand_y1,usb_z-usb_plate_h/2]) rotate([90,0,0])
  linear_extrude(0.1) hull()
   for(x=[3,usb_plate_w-3],y=[3,usb_plate_h-3])translate([x,y])circle(r=3);
}
module stand_envelope() {
 hull() {
  stand_roof();
  usb_landing();
  // Quarter-circle samples soften the transition onto the desk without spheres
  // or a costly Minkowski sum. The lowest section remains exactly at Z=0.
  for(a=[0:15:90]) translate([0,0,stand_roundover*(1-cos(a))])
   linear_extrude(0.02) stand_footprint(stand_roundover*(1-sin(a)));
 }
}
module stand_cavity() {
 // Follow the taper instead of punching a rectangular void through the sides.
 hull() {
  stand_roof(stand_wall);
  translate([0,0,stand_base])linear_extrude(0.1)stand_footprint(stand_wall);
 }
}
module stand() {
 difference() {
  union() {
   desk_pose() translate([0,0,depth-back_t]) back();
   difference() {
    stand_envelope();
    stand_cavity();
    translate([0,0,-1])linear_extrude(stand_base+2)stand_footprint(7);
   }
  }
  // The wider shell overhangs the old rear plane. Recess its plate-sized patch
  // back to that plane so the unchanged USB plate still seats flat.
  translate([usb_x-usb_plate_w/2,stand_y1+10,usb_z-usb_plate_h/2])rotate([90,0,0])
   linear_extrude(10)offset(delta=0.25)usb_plate_outline();
  translate([usb_x-16,stand_y1-10,usb_z-9]) cube([32,11,18]);
  for(dx=[-usb_screw_spacing/2,usb_screw_spacing/2]) translate([usb_x+dx,stand_y1+1,usb_z]) rotate([90,0,0]) cylinder(h=11,d=case_pilot);
  desk_pose() case_positions() translate([0,0,depth+0.1]) cylinder(h=height*2,d=7.5);
  desk_pose()feather_pin_reliefs();
 }
}
module usb_plate_outline() {
 hull() for(x=[2,usb_plate_w-2],y=[2,usb_plate_h-2])translate([x,y])circle(r=2);
}
module usb_plate() {
 difference() {
  linear_extrude(usb_plate_t)usb_plate_outline();
  translate([(usb_plate_w-usb_slot_w)/2,(usb_plate_h-usb_slot_h)/2,-1]) cube([usb_slot_w,usb_slot_h,usb_plate_t+2]);
  for(dx=[-usb_screw_spacing/2,usb_screw_spacing/2]) translate([usb_plate_w/2+dx,usb_plate_h/2,-1]) cylinder(h=usb_plate_t+2,d=case_clearance);
  for(dx=[-usb_slot_w/2-3,usb_slot_w/2+3]) translate([usb_plate_w/2+dx-1,usb_plate_h/2-2,-1]) cube([2,4,usb_plate_t+2]);
 }
}
module usb_pose() {
 translate([usb_x-usb_plate_w/2,stand_y1+usb_plate_t,usb_z-usb_plate_h/2]) rotate([90,0,0]) children();
}
module hardware_clearance() {
 // Known board/body envelopes, leaving intended PCB support contact out.
 intersection(){front();electronics_proxy();}
 intersection(){translate([0,0,depth-back_t])back();electronics_proxy();}
 // Check the full-length header allowance against the actual hollow base as well.
 intersection(){stand();desk_pose()feather_pins_proxy();}
 intersection(){union(){screen_pcb();lcd();encoder_body();encoder_pcb();screen_headers_proxy(2);}union(){feather_pcb_proxy();feather_components_proxy();feather_usb_proxy();feather_qt_proxy();}}
 // Screws enter Feather mounts from its component side; exclude intentional board contact.
 intersection(){feather_holes()translate([0,0,feather_z-3.6])cylinder(h=2,d=4.5);union(){screen_pcb();screen_headers_proxy();encoder_body();encoder_pcb();}}
 // Full plug housings and arbitrary cable bends are deliberately not included.
}
module case_clearance() {
 intersection(){desk_pose() front();stand();}
 intersection(){stand();usb_pose() usb_plate();}
}
module preview() {
 desk_pose() {
  color("#344958") front();
  color("#101c25") translate([window_x-window_w/2,window_y-window_h/2,-0.1]) cube([window_w,window_h,0.2]);
  color("#252a32") translate([knob_x,knob_y,-14.5]) cylinder(h=14.5,d=16);
 }
 color("#24313e") stand();
 color("#657889") usb_pose() usb_plate();
}
if(part=="print-layout") {
 front();
 translate([width+8,-stand_y0,0])stand();
 translate([width+8,50,0])usb_plate();
}
else if(part=="assembled") {desk_pose()front();stand();usb_pose()usb_plate();}
else if(part=="front") front();
else if(part=="stand") stand();
else if(part=="usb-plate") usb_plate();
else if(part=="back") back();
else if(part=="hardware-clearance") hardware_clearance();
else if(part=="case-clearance") case_clearance();
else if(part=="internals") {
 color([0.2,0.3,0.4,0.2])front();
 color("green"){screen_pcb();screen_headers_proxy();}
 color("purple"){encoder_body();encoder_pcb();}
 color("blue"){feather_pcb_proxy();feather_components_proxy();}
 color([1,0.5,0,0.5]){feather_usb_proxy();feather_qt_proxy();feather_wiring_proxy();feather_pins_proxy();}
 // Use part="hardware-clearance" with a full CGAL render for collision checks.
}
else if(part=="section") intersection(){preview();translate([-1,-10,-1])cube([width/2+1,120,120]);}
else preview();
