<script lang="ts" module>
	/** The colour of a reliability in percent: red at 0 through yellow to green at 100. */
	export function heat(pct: number): string {
		return `hsl(${pct * 1.2} 75% 45%)`;
	}
</script>

<script lang="ts">
	import type { HFMap } from '$lib/api';
	import land from '$lib/land.json';

	type Pt = { name: string; lat: number; lon: number };
	// A flat world map, x = longitude and y = latitude, so every cell of the model is a rectangle of the same size.
	// The land outline is Natural Earth 110m, already projected into the viewBox (scripts/build-land.py).
	let { tx, rx, map, frame, longPath = false, month }: { tx: Pt; rx: Pt; map: HFMap; frame: number; longPath?: boolean; month: number } = $props();

	const step = $derived(map.step_deg);
	const values = $derived(map.frames[frame]?.reliability_pct ?? []);

	/** The x of a longitude, 0 to 360. */
	function px(lon: number): number {
		return lon + 180;
	}

	/** The y of a latitude, 0 to 180. */
	function py(lat: number): number {
		return 90 - lat;
	}

	/** The great circle from a to b as an SVG path: the short way, or the rest of the circle for the long path; empty for antipodes. */
	function greatCircle(a: Pt, b: Pt, long: boolean): string {
		const rad = Math.PI / 180;
		/** The unit vector of a position on the sphere. */
		const vec = (p: Pt) => [Math.cos(p.lat * rad) * Math.cos(p.lon * rad), Math.cos(p.lat * rad) * Math.sin(p.lon * rad), Math.sin(p.lat * rad)];
		const va = vec(a);
		const vb = vec(b);
		const d = Math.acos(Math.max(-1, Math.min(1, va[0] * vb[0] + va[1] * vb[1] + va[2] * vb[2])));
		if (d < 1e-6 || Math.PI - d < 1e-6) return '';
		// u is the unit vector at a pointing along the circle towards b, so a point at angle t is a*cos t + u*sin t.
		const u = vb.map((x, i) => (x - va[i] * Math.cos(d)) / Math.sin(d));
		const span = long ? -(2 * Math.PI - d) : d;
		const parts: string[] = [];
		let prevLon = a.lon;
		let pen = 'M';
		for (let i = 0; i <= 180; i++) {
			const t = (span * i) / 180;
			const q = va.map((x, j) => x * Math.cos(t) + u[j] * Math.sin(t));
			const lat = Math.asin(q[2]) / rad;
			const lon = Math.atan2(q[1], q[0]) / rad;
			if (Math.abs(lon - prevLon) > 180) pen = 'M';
			parts.push(`${pen}${px(lon).toFixed(1)} ${py(lat).toFixed(1)}`);
			pen = 'L';
			prevLon = lon;
		}
		return parts.join('');
	}

	const path = $derived(greatCircle(tx, rx, longPath));

	// The sun on the 15th of the month at the hour shown: declination by Cooper's formula, longitude from the hour (no equation of time, a few degrees).
	const sun = $derived.by(() => {
		const day = 15 + Math.round(30.4 * (month - 1));
		const decl = 23.45 * Math.sin(((2 * Math.PI) / 365) * (284 + day));
		const hour = map.frames[frame]?.hour_utc ?? 12;
		return { lat: decl, lon: ((180 - 15 * hour + 540) % 360) - 180 };
	});

	/** The night side as an SVG path: the terminator across all longitudes, closed over the pole that is dark. */
	function night(sunLat: number, sunLon: number): string {
		const rad = Math.PI / 180;
		// At the equinox the terminator is a meridian pair; a hair of declination keeps the formula finite.
		const t = Math.tan(Math.max(0.01, Math.abs(sunLat)) * rad) * Math.sign(sunLat || 1);
		const pts: string[] = [];
		for (let x = 0; x <= 360; x += 2) {
			const lat = Math.atan(-Math.cos((x - 180 - sunLon) * rad) / t) / rad;
			pts.push(`${x} ${py(lat).toFixed(1)}`);
		}
		const edge = sunLat > 0 ? 180 : 0;
		return `M${pts.join('L')}L360 ${edge}L0 ${edge}Z`;
	}
</script>

<svg viewBox="0 0 360 180" class="h-auto w-full rounded border" role="img" aria-label="World map">
	<rect width="360" height="180" fill="var(--cp-line-soft)" />
	<path d={land.path} fill="var(--cp-surface)" stroke="var(--cp-line)" stroke-width="0.3" />
	{#each [-60, -30, 0, 30, 60] as lat (lat)}
		<line x1="0" x2="360" y1={py(lat)} y2={py(lat)} stroke="var(--cp-line)" stroke-width="0.2" />
	{/each}
	{#each [-120, -60, 0, 60, 120] as lon (lon)}
		<line x1={px(lon)} x2={px(lon)} y1="0" y2="180" stroke="var(--cp-line)" stroke-width="0.2" />
	{/each}
	{#each values as v, i (i)}
		{#if v >= 0}
			{@const r = Math.floor(i / map.cols)}
			{@const c = i % map.cols}
			<rect x={c * step} y={r * step} width={step} height={step} fill={heat(v)} fill-opacity="0.6">
				<title>{90 - step * (r + 0.5)}°, {-180 + step * (c + 0.5)}°: {v} %</title>
			</rect>
		{/if}
	{/each}
	<path d={night(sun.lat, sun.lon)} data-night fill="var(--cp-fg)" fill-opacity="0.18" />
	<circle cx={px(sun.lon)} cy={py(sun.lat)} r="2" data-sun fill="var(--cp-warn)"><title>Sun</title></circle>
	{#if path}
		<path d={path} data-path fill="none" stroke="var(--cp-fg)" stroke-width="0.8" stroke-dasharray={longPath ? '2 1.5' : undefined} />
	{/if}
	{#each [['A', tx, 'var(--cp-accent)', 'var(--cp-surface)'], ['B', rx, 'var(--cp-surface)', 'var(--cp-accent)']] as const as [end, s, fill, stroke] (end)}
		<circle cx={px(s.lon)} cy={py(s.lat)} r="2.2" {fill} {stroke} stroke-width="1"><title>{end}: {s.name}</title></circle>
		<text
			x={px(s.lon) + 3.5}
			y={py(s.lat) + 2.2}
			font-size="6"
			font-weight="600"
			fill="var(--cp-fg)"
			stroke="var(--cp-surface)"
			stroke-width="0.6"
			paint-order="stroke">{end}</text
		>
	{/each}
</svg>
