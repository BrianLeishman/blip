#!/usr/bin/env python3
"""Extract measured XY dimensions from vendor Eagle files; no Z heights inferred."""
import json
from pathlib import Path
import xml.etree.ElementTree as ET
root = Path(__file__).resolve().parents[1]
sources = {
 'screen': ('build/tft-cad/Adafruit 2.4in TFT FeatherWing V2.brd', 'https://github.com/adafruit/Adafruit-2.4-TFT-FeatherWing-PCB'),
 'feather': ('build/feather-cad/Adafruit Feather RP2040.brd', 'https://github.com/adafruit/Adafruit-Feather-RP2040-PCB'),
 'encoder': ('build/encoder-cad/Adafruit I2C QT Rotary Encoder.brd', 'https://github.com/adafruit/Adafruit-I2C-QT-Rotary-Encoder-PCB'),
}
data = {}
for key, (path,url) in sources.items():
 tree=ET.parse(root/path)
 wires=[w for w in tree.findall('.//board/plain/wire') if w.get('layer')=='20']
 xs=[float(w.get(k)) for w in wires for k in ('x1','x2')]
 ys=[float(w.get(k)) for w in wires for k in ('y1','y2')]
 item={'source':url,'file':Path(path).name,'size_xy':[max(xs)-min(xs),max(ys)-min(ys)],'holes':[],'connectors':[]}
 for e in tree.findall('.//board/elements/element'):
  package=e.get('package'); x=float(e.get('x'));y=float(e.get('y'))
  if 'MOUNTINGHOLE' in package:
   p=tree.find('.//package[@name="'+package+'"]'); pad=p.find('pad')
   item['holes'].append([x,y,float(pad.get('drill'))])
  if any(t in package for t in ('JST','USB','FEATHERWING','1X16','1X12','PEC11','TFT_2.4')):
   item['connectors'].append({'name':e.get('name'),'package':package,'xy':[x,y],'rotation':e.get('rot','R0')})
 data[key]=item
(root/'enclosure/v2/vendor-dimensions.json').write_text(json.dumps(data,indent=2)+'\n')
lines=['// XY facts extracted from Adafruit Eagle CAD. See vendor-dimensions.json.', '// Board thickness and component heights are deliberately NOT inferred.']
for key,item in data.items():
 lines.append(f'{key}_xy = {json.dumps(item["size_xy"])};')
 lines.append(f'{key}_holes = {json.dumps(item["holes"])};')
(root/'enclosure/v2/dimensions.scad').write_text('\n'.join(lines)+'\n')
