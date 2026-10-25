<script lang="ts" module>
	export type Pt = { x: number; y: number };
	export type Series = { label: string; color: string; points: Pt[] };
	export type Band = { label: string; color: string; lo: Pt[]; hi: Pt[] };
	export type Ref = { label: string; color: string; y: number };
</script>

<script lang="ts">
	import InfoButton from '$lib/InfoButton.svelte';

	// A line chart over the hours of a day as inline SVG, so no chart library is needed. hover is the hour under the
	// pointer; bound by the page, the charts share one crosshair.
	let {
		title,
		unit,
		series,
		band,
		refs = [],
		yMax,
		info,
		hover = $bindable(null)
	}: { title: string; unit: string; series: Series[]; band?: Band; refs?: Ref[]; yMax?: number; info?: string; hover?: number | null } = $props();

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

	const xs = $derived([...new Set(series.flatMap((s) => s.points.map((p) => p.x)))].sort((a, b) => a - b));

	/** Moves the crosshair to the plotted hour nearest to the pointer. */
	function track(e: PointerEvent) {
		const r = e.currentTarget as SVGSVGElement;
		const x = ((e.clientX - r.getBoundingClientRect().left) / r.clientWidth) * W;
		const hour = ((x - m.l) / (W - m.l - m.r)) * 24;
		hover = xs.length ? xs.reduce((best, h) => (Math.abs(h - hour) < Math.abs(best - hour) ? h : best)) : null;
	}

	/** The value of a series at the hovered hour, if it has one. */
	function at(pts: Pt[]): number | undefined {
		return pts.find((p) => p.x === hover)?.y;
	}

	/** The rows of the tooltip: every series and the band edges with a value at the hovered hour. */
	const rows = $derived.by(() => {
		if (hover == null) return [];
		const out = series.flatMap((s) => {
			const y = at(s.points);
			return y == null ? [] : [{ label: s.label, color: s.color, text: `${y.toFixed(1)} ${unit}` }];
		});
		const lo = band && at(band.lo);
		const hi = band && at(band.hi);
		if (band && lo != null && hi != null) out.push({ label: band.label, color: band.color, text: `${lo.toFixed(1)} to ${hi.toFixed(1)} ${unit}` });
		return out;
	});
</script>

<figure class="relative rounded-lg border bg-card p-3">
	<figcaption class="mb-2 pr-6 text-sm font-medium">{title} ({unit}) by UTC hour</figcaption>
	{#if info}<InfoButton label="About the {title} chart" text={info} />{/if}
	<svg
		viewBox="0 0 {W} {H}"
		class="h-auto w-full"
		role="img"
		aria-label="{title} ({unit})"
		onpointermove={track}
		onpointerleave={() => (hover = null)}
	>
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
				<circle cx={px(p.x)} cy={py(p.y)} r={p.x === hover ? 5 : 3} fill={s.color}
					><title>{s.label}: {p.x}:00 UTC, {p.y.toFixed(1)} {unit}</title></circle
				>
			{/each}
		{/each}
		{#if hover != null}
			<line x1={px(hover)} x2={px(hover)} y1={m.t} y2={H - m.b} stroke="var(--cp-fg)" stroke-width="1" stroke-dasharray="3 3" data-crosshair />
		{/if}
	</svg>
	{#if hover != null && rows.length}
		<div
			class="pointer-events-none absolute top-10 z-10 rounded-md border bg-popover p-2 text-xs text-popover-foreground shadow-md"
			style:left={hover <= 12 ? `calc(${(px(hover) / W) * 100}% + 8px)` : undefined}
			style:right={hover > 12 ? `calc(${100 - (px(hover) / W) * 100}% + 8px)` : undefined}
			data-tooltip
		>
			<p class="mb-1 font-mono font-medium tabular-nums">{String(hover).padStart(2, '0')}:00 UTC</p>
			{#each rows as r (r.label)}
				<p class="flex items-center gap-1.5 whitespace-nowrap">
					<span class="inline-block size-2 rounded-sm" style:background-color={r.color}></span>{r.label}:
					<span class="font-mono tabular-nums">{r.text}</span>
				</p>
			{/each}
		</div>
	{/if}
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
