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
	let { tx, rx, map, frame }: { tx: Pt; rx: Pt; map: HFMap; frame: number } = $props();

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
