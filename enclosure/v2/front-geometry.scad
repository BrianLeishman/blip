// Shared V2 front geometry: physically fit-tested screen and knob placement.
// Units: mm. Front flat face goes on the print bed. See README.md before assembly.
include <dimensions.scad>
$fn=64;
board_gap=-2; // 2 mm XY overlap; board depths differ. User confirmed the taller-post coupon fits.
board_w=screen_xy[1]; board_h=screen_xy[0];
board_x=3.4;
height=61;
// Preserve the successful tapered.scad opening-to-PCB offsets after its 180° flip.
board_y=(height-board_h)/2+0.102;
board_z=4.96+0.020*25.4; // Fit feedback: +0.508 mm to avoid compressing the LCD.
face=2;
window_x=board_x+34.20;
window_y=board_y+26.238;
window_w=50.5; window_h=38.9;
encoder_x=board_x+board_w+board_gap;
knob_x=encoder_x+encoder_xy[0]/2;
knob_y=height/2;
width=encoder_x+encoder_xy[0]+2;
shaft_hole=7.4;
post_radius=3.2; post_inward=2;
pilot=2.2; // M2.5 forms threads in plastic, same as successful V1.
rail_wall=1.8; rail_length=8; rail_height=3; rail_gap=0.3;

module outline() {
 hull() for(x=[4,width-4],y=[4,height-4]) translate([x,y]) circle(r=4);
}
module display_positions() {
 for(x=[2.54,board_w-2.54],y=[2.54,board_h-2.54])
  translate([board_x+x,board_y+y,0]) children();
}
module posts() {
 for(x=[2.54,board_w-2.54],y=[2.54,board_h-2.54])
  translate([board_x+x,board_y+y,0]) intersection() {
   cylinder(h=board_z,r=post_radius);
   translate([x<board_w/2 ? -post_radius : -post_inward,
              y<board_h/2 ? -post_radius : -post_inward,0])
    cube([post_radius+post_inward,post_radius+post_inward,board_z]);
  }
}
module rails() {
 translate([knob_x,knob_y,0]) rotate([0,0,45]) for(side=[-1,1])
  translate([-rail_length/2,side>0 ? 6.6+rail_gap : -6.6-rail_gap-rail_wall,face-0.1])
   cube([rail_length,rail_wall,rail_height+0.1]);
}
module fit() {
 difference() {
  union() {linear_extrude(face) outline();posts();rails();}
  translate([window_x-window_w/2,window_y-window_h/2,-0.1]) cube([window_w,window_h,face+0.2]);
  translate([knob_x,knob_y,-0.1]) cylinder(h=face+0.2,d=shaft_hole);
  display_positions() translate([0,0,face]) cylinder(h=board_z,d=pilot);
 }
}
// Conservative rigid envelopes only. Real solder joints/connectors are absent.
module screen_pcb() {translate([board_x,board_y,board_z+0.01]) cube([board_w,board_h,1.6]);}
module lcd() {
 // Keep front Z out of the proven face; only test bosses/rails against XY footprint.
 translate([board_x+1.175,board_y+(board_h-42.6)/2,face+0.01]) cube([60.15,42.6,board_z-face-0.02]);
}
module encoder_body() {
 translate([knob_x,knob_y,face+0.01]) rotate([0,0,45]) translate([-6.6,-6.6,0]) cube([13.2,13.2,10.4]);
}
module encoder_pcb() {translate([encoder_x,knob_y-12.7,face+10.4]) cube([25.4,25.4,1.6]);}
module clearance() {
 intersection() {fit();union(){screen_pcb();lcd();encoder_body();encoder_pcb();}}
 intersection() {union(){screen_pcb();lcd();} union(){encoder_body();encoder_pcb();}}
 // M2.5 screw head and driver access estimates, not final case fasteners.
 intersection() {union(){encoder_body();encoder_pcb();rails();} display_positions() translate([0,0,board_z+1.6]) cylinder(h=15,d=6);}
}
