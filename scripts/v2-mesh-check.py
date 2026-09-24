#!/usr/bin/env python3
"""Validate V2 ASCII STL exports: closed edges, connected parts, bed contact."""
from collections import Counter
from pathlib import Path
import json

root = Path(__file__).resolve().parents[1]
results = []
for filename, expected in [('v2-front-m25.stl', 1), ('v2-stand.stl', 1),
                           ('v2-usb-plate.stl', 1), ('v2-print-layout.stl', 3)]:
    vertices = [tuple(map(float, line.split()[1:]))
                for line in (root / 'enclosure/stl' / filename).read_text().splitlines()
                if line.strip().startswith('vertex ')]
    assert vertices and len(vertices) % 3 == 0, filename
    edges = Counter()
    graph = {}
    for i in range(0, len(vertices), 3):
        triangle = vertices[i:i + 3]
        for a, b in zip(triangle, triangle[1:] + triangle[:1]):
            edges[tuple(sorted((a, b)))] += 1
            graph.setdefault(a, set()).add(b)
            graph.setdefault(b, set()).add(a)
    assert all(count == 2 for count in edges.values()), f'{filename}: open/nonmanifold edge'
    seen = set()
    components = []
    for vertex in graph:
        if vertex in seen:
            continue
        todo, component = [vertex], []
        while todo:
            vertex = todo.pop()
            if vertex in seen:
                continue
            seen.add(vertex)
            component.append(vertex)
            todo.extend(graph[vertex] - seen)
        assert abs(min(p[2] for p in component)) < 0.0001, f'{filename}: part off bed'
        components.append(component)
    assert len(components) == expected, f'{filename}: unexpected disconnected regions'
    bounds = [[min(p[i] for p in vertices), max(p[i] for p in vertices)] for i in range(3)]
    results.append({'file': filename, 'connected_parts': len(components),
                    'watertight_edges': True, 'bounds_mm': bounds})
print(json.dumps(results, indent=2))
