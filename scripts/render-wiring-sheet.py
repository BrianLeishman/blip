#!/usr/bin/env python3
"""Generate the printable wiring map. Requires reportlab; no device access.

Header order verified against Adafruit Eagle CAD:
https://github.com/adafruit/Adafruit-Feather-RP2040-PCB
https://github.com/adafruit/Adafruit-2.4-TFT-FeatherWing-PCB
View transformations: USB/RST end up; the Feather reverse face mirrors left/right.
"""

import os
from pathlib import Path

from reportlab.graphics import renderPDF, renderSVG
from reportlab.graphics.shapes import Circle, Drawing, Line, Rect, String
from reportlab.lib.colors import HexColor
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont

# Embed a TrueType font so the printable sheet does not depend on a viewer's
# substitutes for the PDF built-in fonts. Override these on another platform.
pdfmetrics.registerFont(TTFont('Arial', os.environ.get('BLIP_FONT', '/System/Library/Fonts/Supplemental/Arial.ttf')))
pdfmetrics.registerFont(TTFont('Arial-Bold', os.environ.get('BLIP_FONT_BOLD', '/System/Library/Fonts/Supplemental/Arial Bold.ttf')))

ROOT = Path(__file__).resolve().parents[1]
PDF = ROOT / 'output/pdf/blip-soldering-map.pdf'
SVG = ROOT / 'docs/images/blip-soldering-map.svg'
W, H = 792, 612  # US Letter, landscape
d = Drawing(W, H)
INK, MUTED = '#173044', '#526878'


def rect(x, y, w, h, fill, stroke=None, radius=0):
    d.add(Rect(x, H-y-h, w, h, rx=radius, ry=radius,
               fillColor=HexColor(fill), strokeColor=HexColor(stroke) if stroke else None))


def text(x, y, value, size=10, color=INK, bold=False, anchor='start'):
    d.add(String(x, H-y, value, fontName='Arial-Bold' if bold else 'Arial',
                 fontSize=size, fillColor=HexColor(color), textAnchor=anchor))


def line(x1, y1, x2, y2, color='#cbd5db', width=0.6):
    d.add(Line(x1, H-y1, x2, H-y2, strokeColor=HexColor(color), strokeWidth=width))


# Letters identify wires, never GPIO numbers or physical header pin numbers.
wires = [
    ('A', '3V / 3.3V', '3V / 3.3V', '3.3 V power - use the second position from RST', '#b9342c'),
    ('B', 'GND', 'GND', 'Ground / power return', '#263238'),
    ('C', 'SCK', 'SCK / SCLK', 'SPI clock: timing for screen commands and pixels', '#a65c00'),
    ('D', 'MO / MOSI', 'MO / MOSI', 'SPI data from Feather to screen', '#7b479c'),
    ('E', 'MI / MISO', 'MI / MISO', 'SPI return data: LCD state readback (new ninth wire)', '#285db0'),
    ('F', '9 / D9', '9 / D9 (TFTCS)', 'Screen chip select', '#257344'),
    ('G', '10 / D10', '10 / D10 (TFTDC)', 'Selects command bytes versus pixel/data bytes', '#087d85'),
    ('H', 'SDA', 'SDA', 'I2C data: touchscreen on the screen board', '#93502f'),
    ('I', 'SCL', 'SCL', 'I2C clock: touchscreen on the screen board', '#4c597f'),
]
colors = {w[0]: w[-1] for w in wires}
feather_left = [
    ('RST', None), ('3V', 'A'), ('3V unused', None), ('GND', 'B'),
    ('A0', None), ('A1', None), ('A2', None), ('A3', None),
    ('24', None), ('25', None), ('SCK', 'C'), ('MO', 'D'),
    ('MI', 'E'), ('RX', None), ('TX', None), ('4', None),
]
wing_left = [
    ('RST', None), ('3V', 'A'), ('AREF unused', None), ('GND', 'B'),
    ('A0', None), ('A1', None), ('A2', None), ('A3', None),
    ('A4', None), ('A5', None), ('SCK', 'C'), ('MO', 'D'),
    ('MI', 'E'), ('RX', None), ('TX', None), ('G', None),
]
right = [
    ('BAT', None), ('EN', None), ('USB', None), ('13', None),
    ('12', None), ('11', None), ('10', 'G'), ('9', 'F'),
    ('6', None), ('5', None), ('SCL', 'I'), ('SDA', 'H'),
]


def header_pin(x, y, label, wire, side):
    color = colors[wire] if wire else '#9aa8b0'
    d.add(Circle(x, H-y, 3.4, fillColor=HexColor(color), strokeColor=None))
    lx = x-10 if side == 'left' else x+10
    text(lx, y+3, label, 8.8, color if wire else MUTED,
         bold=bool(wire), anchor='end' if side == 'left' else 'start')
    if wire:
        bx = x+12 if side == 'left' else x-12
        d.add(Circle(bx, H-y, 5.1, fillColor=HexColor(color), strokeColor=None))
        text(bx, y+2.5, wire, 6.8, '#ffffff', True, 'middle')


def board(x, title, subtitle, mirror=False, wing=False):
    width = 232
    rect(x, 103, width, 244, '#f2f6f8', '#d9e2e7', 7)
    text(x+width/2, 121, title, 11, bold=True, anchor='middle')
    text(x+width/2, 136, subtitle, 8.2, MUTED, anchor='middle')
    cx, top, pitch = x+width/2, 166, 11.1
    left_x, right_x = cx-35, cx+35
    rect(left_x-8, 151, 86, 190, '#ffffff', '#8c9ea8', 5)
    if wing:
        text(cx, 158, 'RST end', 7.2, MUTED, anchor='middle')
    else:
        rect(cx-13, 145, 26, 13, '#dce4e8', '#8c9ea8', 2)
        text(cx, 154, 'USB-C', 6.4, anchor='middle')
    full = wing_left if wing else feather_left
    if mirror:
        for i, (label, wire) in enumerate(full):
            header_pin(right_x, top+i*pitch, label, wire, 'right')
        for i, (label, wire) in enumerate(right):
            header_pin(left_x, top+(i+4)*pitch, label, wire, 'left')
    else:
        for i, (label, wire) in enumerate(full):
            header_pin(left_x, top+i*pitch, label, wire, 'left')
        for i, (label, wire) in enumerate(right):
            header_pin(right_x, top+(i+4)*pitch, label, wire, 'right')
    center_label = ['LCD', 'behind', 'this PCB'] if wing else (['REVERSE', 'PCB', 'FACE'] if mirror else ['CHIP /', 'COMPONENT', 'SIDE'])
    for i, value in enumerate(center_label):
        text(cx, 241+i*10, value, 6.4, MUTED, True, 'middle')


rect(0, 0, W, H, '#ffffff')
text(28, 32, 'blip / soldering map', 23, bold=True)
text(28, 50, 'Feather RP2040 to 2.4-inch TFT FeatherWing V2  |  9 direct wires  |  Keep the existing Feather headers', 10, MUTED)
rect(28, 64, 736, 28, '#fff3d9', '#e4c578', 4)
text(39, 82, 'POWER OFF: unplug USB and any battery. Follow pin labels; the drawings below are not to scale.', 10, '#714500', True)

board(28, 'FEATHER: component face', 'USB-C at TOP; usual male-header solder side')
board(280, 'FEATHER: reverse face', 'USB-C at TOP; mirrored underside view', mirror=True)
board(532, 'SCREEN: header / pad map', 'Connector side facing you; RST end at TOP', wing=True)

text(28, 363, 'IMPORTANT: Feather A (3V) goes to screen A (3V). The next screen position is AREF - leave it unused.', 10, '#a23028', True)

columns = [28, 58, 179, 342, 764]
table_top, head, row_h = 376, 20, 18
rect(28, table_top, 736, head, '#173044')
for x, value in zip(columns, ['Wire', 'Feather pin', 'Screen pad / socket', 'What it does']):
    text(x+5, table_top+14, value, 9.2, '#ffffff', True)
for i, (wire, feather, wing, purpose, color) in enumerate(wires):
    y = table_top+head+i*row_h
    rect(28, y, 736, row_h, '#edf3f6' if i % 2 == 0 else '#ffffff')
    d.add(Circle(43, H-y-row_h/2, 6, fillColor=HexColor(color), strokeColor=None))
    text(43, y+12, wire, 8, '#ffffff', True, 'middle')
    text(columns[1]+5, y+12, feather, 9.3, bold=True)
    text(columns[2]+5, y+12, wing, 9.3, bold=True)
    text(columns[3]+5, y+12, purpose, 9.1)
    line(28, y+row_h, 764, y+row_h, '#d9e2e7')

text(28, 570, 'Letters/colors are wire IDs, not pin numbers. Gray pins stay unconnected. Knob: keep its QT cable plugged into the Feather.', 8.1, MUTED)
text(28, 581, 'Male-header solder joints are usually on the component face. Match the drawing to the face you are looking at; inspect for bridges.', 8.1, MUTED)
text(28, 592, 'Sources: Adafruit Feather RP2040 PCB + TFT FeatherWing V2 PCB / pinout guides. Layout is a reference; verify board labels.', 7.7, MUTED)

# Output directories are created only when the generator is run.
PDF.parent.mkdir(parents=True, exist_ok=True)
SVG.parent.mkdir(parents=True, exist_ok=True)
renderPDF.drawToFile(d, str(PDF), title='blip nine-wire soldering map', author='blip')
renderSVG.drawToFile(d, str(SVG))
svg = SVG.read_text()
svg = svg.replace('font-family: Arial-Bold;', 'font-family: Arial, sans-serif; font-weight: bold;')
svg = svg.replace('font-family: Arial;', 'font-family: Arial, sans-serif;')
SVG.write_text(svg)
print(PDF)
print(SVG)
