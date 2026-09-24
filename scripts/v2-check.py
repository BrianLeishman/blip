#!/usr/bin/env python3
"""Render the bounded packing candidates and report their proxy collisions."""
from pathlib import Path
import subprocess,json
root=Path(__file__).resolve().parents[1]
out=root/'build/v2';out.mkdir(parents=True,exist_ok=True)
results=[]
for name,layout,gap in [('stacked-close','stacked',-2),('stacked','stacked',2),('remote-flat','remote-flat',2),('remote-edge','remote-edge',2),('remote-close','remote-flat',-2)]:
 row={'candidate':name,'layout':layout,'board_gap_mm':gap,'qt_source':'feather'}
 for view in ['envelope','collision']:
  target=out/f'{name}-{view}.stl'
  target.unlink(missing_ok=True)
  result=subprocess.run(['openscad','--hardwarnings','-o',str(target),'-D',f'layout="{layout}"','-D',f'view="{view}"','-D',f'board_gap={gap}',str(root/'enclosure/v2/packing.scad')],capture_output=True,text=True)
  log=result.stdout+result.stderr
  if 'WARNING:' in log or 'ERROR:' in log:raise RuntimeError(log)
  if view=='collision':
   if not target.exists() and 'Current top level object is empty.' not in log:raise RuntimeError(log)
   row['proxy_collision']=target.exists()
  else:
   if result.returncode:raise RuntimeError(log)
   v=[list(map(float,line.split()[1:])) for line in target.read_text().splitlines() if line.strip().startswith('vertex ')]
   row['envelope_mm']=[round(max(p[i] for p in v)-min(p[i] for p in v),2) for i in range(3)]
 results.append(row)
(out/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps(results,indent=2))
