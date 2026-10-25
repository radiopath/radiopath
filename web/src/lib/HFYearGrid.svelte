<script lang="ts" module>
	export type GridRun = { key: string; label: string; points: HFPoint[] };
</script>

<script lang="ts">
	import type { HFPoint } from '$lib/api';
	import { heat } from '$lib/HFHeatmap.svelte';
	import InfoButton from '$lib/InfoButton.svelte';

	// The months of a range at a glance: one grid per frequency, a row per month, a column per hour, the cell coloured by reliability.
	let { runs, value, selected, onpick }: { runs: GridRun[]; value: (p: HFPoint) => number | null; selected: string; onpick: (key: string) => void } =
		$props();

	const points = $derived(runs.flatMap((r) => r.points));
	const freqs = $derived([...new Set(points.map((p) => p.frequency_mhz))].sort((a, b) => a - b));
	const hours = $derived([...new Set(points.map((p) => p.hour_utc))].sort((a, b) => a - b));

	/** The reliability of a month at an hour and frequency; null when the model gave none. */
	function at(r: GridRun, h: number, f: number): number | null {
		const p = r.points.find((q) => q.hour_utc === h && q.frequency_mhz === f);
		return p ? value(p) : null;
	}
</script>

<section class="relative grid gap-4 rounded-lg border bg-card p-3" aria-label="Months at a glance">
	<InfoButton
		label="About the months at a glance"
		text="One grid per frequency, a row per month and a column per UTC hour; the colour is the circuit reliability like on the map, green works on most days, red almost never. Click a month to show its map, charts and table below."
	/>
	<h2 class="pr-6 text-sm font-medium">Months at a glance</h2>
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
								<button
									type="button"
									class={['hover:underline', r.key === selected && 'font-medium text-primary']}
									aria-current={r.key === selected ? 'true' : undefined}
									onclick={() => onpick(r.key)}>{r.label}</button
								>
							</td>
							{#each hours as h (h)}
								{@const v = at(r, h, f)}
								<td
									class="h-5 min-w-4 rounded-sm"
									style:background-color={v == null ? undefined : `color-mix(in srgb, ${heat(v)} 70%, transparent)`}
									title="{r.label}, {String(h).padStart(2, '0')}:00 UTC: {v == null ? 'no value' : `${v.toFixed(0)} %`}"
								></td>
							{/each}
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/each}
</section>
