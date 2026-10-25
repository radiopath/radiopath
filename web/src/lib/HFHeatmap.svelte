<script lang="ts" module>
	/** The colour of a reliability in percent: red at 0 through yellow to green at 100. */
	export function heat(pct: number): string {
		return `hsl(${pct * 1.2} 75% 45%)`;
	}
</script>

<script lang="ts">
	import L from 'leaflet';
	import type { HFMap } from '$lib/api';
	import { baseMap } from '$lib/map';

	type Pt = { name: string; lat: number; lon: number };
	let { tx, rx, map, frame }: { tx: Pt; rx: Pt; map: HFMap; frame: number } = $props();
	let el: HTMLDivElement;
	let cells = $state.raw<L.Rectangle[]>([]);

	// The map with one rectangle per cell and a marker for each end; the cells are coloured by the effect below.
	$effect(() => {
		const m = baseMap(el);
		const canvas = L.canvas();
		const half = map.step_deg / 2;
		const rects: L.Rectangle[] = [];
		for (let r = 0; r < map.rows; r++) {
			for (let c = 0; c < map.cols; c++) {
				const lat = 90 - map.step_deg * (r + 0.5);
				const lon = -180 + map.step_deg * (c + 0.5);
				rects.push(
					L.rectangle(
						[
							[lat - half, lon - half],
							[lat + half, lon + half]
						],
						{ renderer: canvas, stroke: false, interactive: false, fillOpacity: 0 }
					).addTo(m)
				);
			}
		}
		L.marker([tx.lat, tx.lon]).addTo(m).bindTooltip(`A (TX): ${tx.name}`);
		L.marker([rx.lat, rx.lon]).addTo(m).bindTooltip(`B (RX): ${rx.name}`);
		m.fitBounds(
			[
				[-60, -170],
				[80, 170]
			],
			{ padding: [10, 10] }
		);
		cells = rects;
		// The page grows when the charts arrive and may get a scrollbar, which narrows the map after Leaflet measured it.
		const resize = new ResizeObserver(() => m.invalidateSize());
		resize.observe(el);
		return () => {
			resize.disconnect();
			m.remove();
		};
	});

	// Another hour only recolours the cells; a cell the model could not calculate stays clear.
	$effect(() => {
		const values = map.frames[frame]?.reliability_pct ?? [];
		cells.forEach((rect, i) => {
			const v = values[i];
			rect.setStyle(v == null || v < 0 ? { fillOpacity: 0 } : { fillColor: heat(v), fillOpacity: 0.55 });
		});
	});
</script>

<div class="map" bind:this={el} role="img" aria-label="Reliability map"></div>
