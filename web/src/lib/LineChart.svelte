<script lang="ts" module>
	export type Pt = { x: number; y: number };
	export type Series = { label: string; color: string; points: Pt[] };
	export type Band = { label: string; color: string; lo: Pt[]; hi: Pt[] };
	export type Ref = { label: string; color: string; y: number };
</script>

<script lang="ts">
	import InfoButton from '$lib/InfoButton.svelte';

	// A line chart over the hours of a day as inline SVG, so no chart library is needed.
	let {
		title,
		unit,
		series,
		band,
		refs = [],
		yMax,
		info
	}: { title: string; unit: string; series: Series[]; band?: Band; refs?: Ref[]; yMax?: number; info?: string } = $props();

	const W = 640;
	const H = 240;
	const m = { l: 40, r: 12, t: 10, b: 26 };

	/** The top of the y axis and the distance between its ticks for the largest value v, at most five ticks. */
	function scale(v: number): { max: number; step: number } {
		const step = [1, 2, 5, 10, 20, 25, 50, 100].find((s) => v / s <= 5) ?? 100;
		return { max: Math.ceil(v / step) * step || step, step };
	}

	const sc = $derived(
		scale(yMax ?? Math.max(0, ...series.flatMap((s) => s.points.map((p) => p.y)), ...(band?.hi.map((p) => p.y) ?? []), ...refs.map((r) => r.y)))
	);
	const ticks = $derived(Array.from({ length: Math.round(sc.max / sc.step) + 1 }, (_, i) => i * sc.step));

	/** The x position of an hour of the day, 0 to 24. */
	function px(hour: number): number {
		return m.l + (hour / 24) * (W - m.l - m.r);
	}

	/** The y position of a value on the axis. */
	function py(value: number): number {
		return H - m.b - (value / sc.max) * (H - m.t - m.b);
	}

	/** The SVG path through points. */
	function line(pts: Pt[]): string {
		return pts.map((p, i) => `${i ? 'L' : 'M'}${px(p.x)} ${py(p.y)}`).join(' ');
	}

	/** The corner points of the band between its lower and upper edge. */
	function area(b: Band): string {
		return [...b.lo, ...[...b.hi].reverse()].map((p) => `${px(p.x)},${py(p.y)}`).join(' ');
	}
</script>

<figure class="relative rounded-lg border bg-card p-3">
	<figcaption class="mb-2 pr-6 text-sm font-medium">{title} ({unit}) by UTC hour</figcaption>
	{#if info}<InfoButton label="About the {title} chart" text={info} />{/if}
	<svg viewBox="0 0 {W} {H}" class="h-auto w-full" role="img" aria-label="{title} ({unit})">
		{#each ticks as t (t)}
			<line x1={m.l} x2={W - m.r} y1={py(t)} y2={py(t)} stroke="var(--cp-line-soft)" />
			<text x={m.l - 6} y={py(t) + 4} text-anchor="end" font-size="11" class="fill-muted-foreground">{t}</text>
		{/each}
		{#each [0, 6, 12, 18, 24] as h (h)}
			<text x={px(h)} y={H - 8} text-anchor="middle" font-size="11" class="fill-muted-foreground">{h}</text>
		{/each}
		<line x1={m.l} x2={W - m.r} y1={py(0)} y2={py(0)} stroke="var(--cp-line)" />
		{#if band}
			<polygon points={area(band)} fill={band.color} fill-opacity="0.2" />
		{/if}
		{#each refs as r (r.label)}
			<line x1={m.l} x2={W - m.r} y1={py(r.y)} y2={py(r.y)} stroke={r.color} stroke-dasharray="4 3" />
		{/each}
		{#each series as s (s.label)}
			<path d={line(s.points)} fill="none" stroke={s.color} stroke-width="2" />
			{#each s.points as p (p.x)}
				<circle cx={px(p.x)} cy={py(p.y)} r="3" fill={s.color}><title>{s.label}: {p.x}:00 UTC, {p.y.toFixed(1)} {unit}</title></circle>
			{/each}
		{/each}
	</svg>
	<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
		{#if band}
			<li class="flex items-center gap-1.5">
				<span class="inline-block h-2.5 w-4 rounded-sm" style:background-color={band.color} style:opacity="0.3"></span>{band.label}
			</li>
		{/if}
		{#each series as s (s.label)}
			<li class="flex items-center gap-1.5"><span class="inline-block h-0.5 w-4" style:background-color={s.color}></span>{s.label}</li>
		{/each}
		{#each refs as r (r.label)}
			<li class="flex items-center gap-1.5"><span class="inline-block w-4 border-t-2 border-dashed" style:border-color={r.color}></span>{r.label}</li>
		{/each}
	</ul>
</figure>
