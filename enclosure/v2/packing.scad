// blip V2 component packing study, NOT a print-ready enclosure.
// XY from vendor Eagle CAD. Amber volumes are provisional connector/bend space.
include <dimensions.scad>
layout = "stacked"; // [stacked,remote-flat,remote-edge]
qt_source="feather"; // [feather,screen]
view = "assembly"; // [assembly,exploded,collision,envelope]
$fn=32;
// Z placements and cable envelopes are provisional; published overall heights
// constrain the boxes but do not determine the assembled component stack.
pcb_t=1.6;
lcd_depth=3.4;
header_height=4.5; // Provisional split of Adafruit's 9.5 mm overall screen height.
component_height=5.9; // With 1.6 mm PCB, matches published Feather 7.5 mm.
encoder_back_depth=12;
knob_d=16; // 450-BA761 drawing linked from Adafruit #377.
knob_h=14.5;
board_gap=2;
qt_overhang=8; // Plug and gentle wire bend, not just bare socket.
usb_overhang=15.24; // User's current plug clearance; proxy, not a bend model.
wire_gap=3;
// Screen lies landscape; x increases right, y down, z behind display face.
sw=screen_xy[1]; sh=screen_xy[0];
screen_z=lcd_depth;
encoder_x=sw+board_gap;
encoder_y=(sh-encoder_xy[1])/2;
// Existing board components and header bodies still occupy space after unplugging.
screen_back=screen_z+pcb_t+header_height;
remote_z=max(screen_back,lcd_depth+encoder_back_depth)+wire_gap;
feather_z=layout=="stacked" ? screen_back : remote_z;
feather_x=layout=="stacked" ? 37.719 : (sw-feather_xy[0])/2;
feather_y=layout=="stacked" ? 0 : (sh-feather_xy[1])/2;
exploded=view=="exploded" ? 20 : 0;

module board(size,holes) {
 difference() {
  linear_extrude(pcb_t) hull() for(x=[2.54,size[0]-2.54],y=[2.54,size[1]-2.54]) translate([x,y]) circle(r=2.54);
  for(h=holes) translate([h[0],h[1],-0.1]) cylinder(h=pcb_t+0.2,d=h[2]);
 }
}
module screen_pose() { translate([0,sh,screen_z]) rotate([0,0,-90]) children(); }
module screen_pcb() { screen_pose() board(screen_xy,screen_holes); }
module screen_headers() {
 // Feather header row XY comes from MS2 footprint at [0,37.719], R0.
 screen_pose() {
  translate([5.08,37.719-1.27,pcb_t]) cube([40.64,2.54,header_height]);
  translate([15.24,37.719+21.59,pcb_t]) cube([30.48,2.54,header_height]);
 }
}
module screen_lcd() {
 // 60.15 x 42.6 package outline from TFT footprint layer 51.
 translate([1.175,(sh-42.6)/2,0]) cube([60.15,42.6,lcd_depth]);
}
module screen_qt_space() {
 // CONN1 [49.53,49.149] becomes [49.149,3.048] in landscape.
 if(qt_source=="screen") translate([49.149-4,-qt_overhang,screen_z+pcb_t]) cube([8,qt_overhang+5,5]);
}
module encoder_pcb() {translate([encoder_x,encoder_y,lcd_depth+encoder_back_depth-pcb_t]) board(encoder_xy,encoder_holes);}
module encoder_body() {
 translate([encoder_x+12.7,encoder_y+12.7,lcd_depth]) rotate([0,0,45])
  translate([-6.6,-6.6,0]) cube([13.2,13.2,encoder_back_depth-pcb_t]);
}
module encoder_qt_space() {
 // Choose the lower port by rotating breakout 90 degrees in its front plane.
 translate([encoder_x+8.7,encoder_y+25.4,lcd_depth+encoder_back_depth-5]) cube([8,qt_overhang,5]);
}
module feather_pose() {
 if(layout=="stacked") translate([37.719,sh,feather_z+exploded]) rotate([0,0,-90]) children();
 else if(layout=="remote-edge") translate([6,sh-8,remote_z+exploded]) rotate([90,0,0]) children();
 else translate([feather_x+feather_xy[0],feather_y+feather_xy[1],feather_z+exploded]) rotate([0,0,180]) children();
}
module feather_pcb() {feather_pose() board(feather_xy,feather_holes);}
module feather_components() {feather_pose() translate([0,0,pcb_t]) cube([feather_xy[0],feather_xy[1],component_height]);}
module feather_qt_space() {
 // CONN1 at [47.752,8.255], facing the far end of the Feather.
 if(qt_source=="feather") feather_pose()
  translate([feather_xy[0]-5,8.255-4,pcb_t]) cube([5+qt_overhang,8,5]);
}
module feather_usb_space() {feather_pose() translate([-usb_overhang,6,0]) cube([usb_overhang,11,7]);}
// Generic case-mounted socket body; no chosen product or screw pattern yet.
module panel_socket_space() {translate([72,sh-8,remote_z+2]) cube([16,12,10]);}
module hardware() {
 color("#237f69") screen_pcb();
 color("#111b26") screen_lcd();
 color("#303039") screen_headers();
 color("#8360b8") encoder_pcb();
 color("silver") encoder_body();
 color("#2485bd") feather_pcb();
 color([0.15,0.42,0.7,0.45]) feather_components();
 color("#394754") translate([encoder_x+12.7,encoder_y+12.7,-knob_h]) cylinder(h=knob_h,d=knob_d);
 color([1,0.55,0.12,0.4]) {screen_qt_space();encoder_qt_space();feather_usb_space();feather_qt_space();panel_socket_space();}
}
module collision() {
 // Test separate rigid/clearance volumes, excluding intended mounts/contact.
 intersection() {union(){screen_pcb();screen_headers();screen_lcd();screen_qt_space();encoder_pcb();encoder_body();encoder_qt_space();} union(){feather_pcb();feather_components();feather_usb_space();feather_qt_space();}}
 intersection() {union(){screen_pcb();screen_headers();screen_lcd();screen_qt_space();} union(){encoder_pcb();encoder_body();encoder_qt_space();}}
 intersection() {panel_socket_space();union(){screen_pcb();screen_headers();screen_qt_space();encoder_pcb();encoder_body();encoder_qt_space();feather_pcb();feather_components();feather_usb_space();feather_qt_space();}}
}
if(view=="envelope") {screen_pcb();screen_lcd();screen_headers();screen_qt_space();encoder_pcb();encoder_body();encoder_qt_space();feather_pcb();feather_components();feather_usb_space();feather_qt_space();panel_socket_space();}
else if(view=="collision") collision();
else {hardware();color("red") collision();}
