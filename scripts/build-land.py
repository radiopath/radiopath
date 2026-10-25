#!/usr/bin/env python3
"""Turns the Natural Earth 110m land GeoJSON into web/src/lib/land.json: one SVG path in the viewBox of the
HF world map (x = longitude + 180, y = 90 - latitude, one decimal). Natural Earth is public domain.

    curl -sSLO https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_land.geojson
    python3 scripts/build-land.py ne_110m_land.geojson
"""
import json
import sys

geo = json.load(open(sys.argv[1]))
rings = []
for f in geo["features"]:
    g = f["geometry"]
    for poly in [g["coordinates"]] if g["type"] == "Polygon" else g["coordinates"]:
        for ring in poly:
            pts = []
            for lon, lat in ring:
                p = f"{round(lon + 180, 1):g} {round(90 - lat, 1):g}"
                if not pts or pts[-1] != p:
                    pts.append(p)
            if len(pts) >= 3:
                rings.append("M" + "L".join(pts) + "Z")
json.dump({"path": "".join(rings)}, open("web/src/lib/land.json", "w"), separators=(",", ":"))
