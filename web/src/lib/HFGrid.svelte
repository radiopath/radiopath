<script lang="ts" module>
	export type GridRun = { key: string; label: string; points: HFPoint[] };

	/** A model value as text; the API's -307 means the quantity does not apply to the path. */
	export function val(x: number, digits = 1): string {
		return x === -307 ? '–' : x.toFixed(digits);
	}
</script>

<script lang="ts">
	import type { HFPoint } from '$lib/api';
	import { heat } from '$lib/HFHeatmap.svelte';
	import InfoButton from '$lib/InfoButton.svelte';

	// The prediction at a glance: one grid per frequency, a row per month, a column per hour, the cell coloured by reliability.
	let { runs, value, selected, onpick }: { runs: GridRun[]; value: (p: HFPoint) => number | null; selected: string; onpick: (key: string) => void } =
		$props();

	const points = $derived(runs.flatMap((r) => r.points));
	const freqs = $derived([...new Set(points.map((p) => p.frequency_mhz))].sort((a, b) => a - b));
	const hours = $derived([...new Set(points.map((p) => p.hour_utc))].sort((a, b) => a - b));

	const info =
		'One grid per frequency, a row per month and a column per UTC hour. The cell is the circuit reliability, the share of the days of the month on which the signal to noise ratio reaches the one your mode needs, coloured like the map: green works on most days, red almost never. Hover a cell for the median SNR in the bandwidth of your mode, the median field strength at the receiver, the median MUF of the hour and the propagation mode (the dominant way the signal gets there, for example 2F2: two hops off the F2 layer; none beyond 7000 km). A dash means the quantity does not apply to the path. With several months, click a month to show its map and charts.';

	/** The point of a month at an hour and frequency. */
	function at(r: GridRun, h: number, f: number): HFPoint | undefined {
		return r.points.find((q) => q.hour_utc === h && q.frequency_mhz === f);
	}

	/** The tooltip of a cell: when, reliability and the figures behind it. */
	function title(r: GridRun, h: number, p: HFPoint | undefined, v: number | null): string {
		const when = `${r.label}, ${String(h).padStart(2, '0')}:00 UTC`;
		if (!p) return `${when}: not predicted`;
		const mode = p.distance_km > 7000 ? '' : `, ${p.dominant_mode}`;
		return `${when}: ${v == null ? '–' : v.toFixed(0) + ' %'}, SNR ${val(p.snr_db)} dB, field ${val(p.field_strength_dbuv_m)} dBµV/m, MUF ${val(p.muf_50_mhz)} MHz${mode}`;
	}
</script>

<section class="relative grid gap-4 rounded-lg border bg-card p-3" aria-label="Reliability by hour">
	<InfoButton label="About the reliability grid" text={info} />
	<h2 class="pr-6 text-sm font-medium">Circuit reliability by hour and frequency</h2>
	{#each freqs as f (f)}
		<div class="overflow-x-auto">
			<table class="w-full border-separate border-spacing-0.5 text-xs" aria-label="Reliability at {f} MHz by month and hour">
				<thead>
					<tr>
						<th class="text-left font-medium whitespace-nowrap">{f} MHz</th>
						{#each hours as h (h)}<th class="font-mono font-normal text-muted-foreground tabular-nums">{String(h).padStart(2, '0')}</th>{/each}
					</tr>
				</thead>
				<tbody>
					{#each runs as r (r.key)}
						<tr>
							<td class="pr-2 whitespace-nowrap">
								{#if runs.length > 1}
									<button
										type="button"
										class={['hover:underline', r.key === selected && 'font-medium text-primary']}
										aria-current={r.key === selected ? 'true' : undefined}
										onclick={() => onpick(r.key)}>{r.label}</button
									>
								{:else}
									{r.label}
								{/if}
							</td>
							{#each hours as h (h)}
								{@const p = at(r, h, f)}
								{@const v = p ? value(p) : null}
								<td
									class="h-5 min-w-6 rounded-sm text-center font-mono text-[10px] tabular-nums"
									style:background-color={v == null ? undefined : `color-mix(in srgb, ${heat(v)} 70%, transparent)`}
									title={title(r, h, p, v)}>{v == null ? '–' : v.toFixed(0)}</td
								>
							{/each}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/each}
</section>
