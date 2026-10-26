# Maps, legends and reports

How coverage rasters are merged and coloured, and what the duplicate, report and theme features do behind the
scenes.

A computed coverage page can show the other computed rasters together with its
own, with one opacity slider for all of them. Ticked rasters are merged by the
server into one image (`/coverages/{id}/composite.png?with=3,5`): where they
overlap, the best link margin wins, so a green cell is never hidden by a yellow
one from another coverage. The merged raster is coloured with the legend of
the coverage whose page it is on. For that the worker stores, next to the
coloured PNG, the margins themselves as an 8-bit grey PNG (`result_margin`,
whole dB); rasters computed before that column existed are merged at the
accuracy of their colour classes.

The legend is per coverage: "Edit legend" on the coverage page opens a dialog
with up to 8 rows of "margin at least N dB" and a colour, as Radio Mobile does
it. The default is 30 / 20 / 10 / 0 dB in green to orange; a cell below every
threshold or without data stays transparent. The legend is applied when a
raster is served (`image.png`, the merged view, the PDF), so changing it needs
no recomputation; it is stored in `legend_db` / `legend_colors` (NULL = default)
and a duplicate keeps it. Results computed before margins were stored keep the
colours they were rendered with until recomputed.

"Duplicate" on a site, link or coverage opens the "new" form with the values of the original (name suffixed " (copy)"); nothing is stored until you save it.

"Print report" on a link or coverage page returns `/links/{id}/report.pdf` or
`/coverages/{id}/report.pdf`: one A4 page with the sites, the radio parameters,
the result table, the terrain profile and a map of the path (or the coverage
raster over the map). The PDF is written by the server with `go-pdf/fpdf`
(core fonts, no font files), so it looks the same in every browser and can be
saved or printed from the browser's PDF viewer. The map is composed from the
same tiles the proxy serves (`internal/tiles.Static`); when the tile server is
unreachable the report still comes out, with a note in place of the map.

"Print report" on the HF page posts the predicted months to `/api/hf/reports`.
HF predictions are not stored, so the server predicts them again rather than
printing figures sent by the browser; right after "Predict" the finished jobs
answer from the job table. The report has no map: the conditions, the
reliability of every month, hour and frequency as coloured cells, and per
month a page of its own with the MUF and reliability charts and a table per
hour.

Light and dark theme are picked from the system setting and can be switched in
the header; the choice is stored in `localStorage` and applied before first
paint. Colours are CSS variables (`--cp-*`) redefined under `[data-theme=dark]`,
which also covers Leaflet's controls, the OSM tiles (inverted in dark mode,
overlays and markers untouched) and the terrain profile SVG. Interface text is
IBM Plex Sans, JetBrains Mono is reserved for numbers and results.
