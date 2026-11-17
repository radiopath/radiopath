<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { CoverageDetail, CoverageView } from '#lib/api.ts';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import CoverageMap from '#lib/CoverageMap.svelte';
	import { day } from '#lib/format.ts';
	import LegendTable from '#lib/LegendTable.svelte';
	import PageHead from '#lib/PageHead.svelte';
	import StatTiles from '#lib/StatTiles.svelte';

	// path is where report and rasters live: /coverages/<id> for the owner, /s/c/<token> for a visitor.
	let {
		d,
		path,
		shared = false,
		badge,
		actions,
		status,
		legend,
		save
	}: {
		d: CoverageDetail;
		path: string;
		shared?: boolean;
		badge?: Snippet;
		actions?: Snippet;
		status?: Snippet;
		legend?: Snippet;
		save?: (v: CoverageView) => void;
	} = $props();
</script>

<PageHead title={d.name} {badge}>
	{@render actions?.()}
	<Button variant="outline" href="{path}/report.pdf" target="_blank" rel="noopener" data-sveltekit-reload>Print report</Button>
</PageHead>
{#if shared}
	<Alert.Root class="mb-4"
		>Shared read-only view{#if d.share}, valid until {day(d.share.expires_at)}{/if}.</Alert.Root
	>
{/if}
{@render status?.()}
{#if d.note}<Alert.Root class="mb-4 border-warn/50 text-warn">{d.note}</Alert.Root>{/if}
<StatTiles
	tiles={[
		{ label: 'Range', value: `${d.range_km.toFixed(1)} km`, sub: `${d.resolution_m.toFixed(0)} m resolution` },
		{ label: 'Frequency', value: `${d.frequency_mhz.toFixed(3)} MHz`, sub: d.polarization },
		{ label: 'Transmitter', value: `${d.tx_power_dbm.toFixed(1)} dBm`, sub: d.tx_antenna || 'omnidirectional' },
		{
			label: 'Computed',
			value: d.computed_at ? new Date(d.computed_at).toLocaleString('sv-SE').slice(0, 16) : 'not yet',
			sub: d.computed_at ? `in ${d.compute_time}` : undefined
		}
	]}
/>
<div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
	<div>
		<h2 class="mb-2 text-xs font-medium text-muted-foreground">Parameters</h2>
		<table class="kv">
			<tbody>
				<tr>
					<td>Site (TX)</td>
					<td>
						{d.site.name}<br /><span class="font-mono text-xs text-muted-foreground"
							>{d.site.lat.toFixed(5)}, {d.site.lon.toFixed(5)}, antenna {d.site.antenna_height_m.toFixed(1)} m above ground</span
						>
					</td>
				</tr>
				<tr><td>Frequency / polarization</td><td class="num">{d.frequency_mhz.toFixed(3)} MHz, {d.polarization}</td></tr>
				<tr>
					<td>TX power / gain / line loss</td>
					<td class="num">{d.tx_power_dbm.toFixed(1)} dBm / {(d.tx_gain_dbi ?? 0).toFixed(1)} dBi / {(d.tx_line_loss_db ?? 0).toFixed(1)} dB</td>
				</tr>
				<tr>
					<td>RX height / gain / line loss</td>
					<td class="num">{d.rx_height_m.toFixed(1)} m / {(d.rx_gain_dbi ?? 0).toFixed(1)} dBi / {(d.rx_line_loss_db ?? 0).toFixed(1)} dB</td>
				</tr>
				<tr>
					<td>TX pattern</td>
					<td class="num">
						{#if d.tx_antenna}
							{d.tx_antenna}, main lobe {(d.tx_azimuth_deg ?? 0).toFixed(0)} deg{#if d.tx_tilt_deg}, tilt {d.tx_tilt_deg.toFixed(1)} deg{/if}
						{:else}<span class="text-muted-foreground">omnidirectional</span>{/if}
					</td>
				</tr>
				<tr><td>RX sensitivity</td><td class="num">{d.rx_sensitivity_dbm.toFixed(1)} dBm</td></tr>
				<tr><td>Range / resolution</td><td class="num">{d.range_km.toFixed(1)} km / {d.resolution_m.toFixed(0)} m</td></tr>
			</tbody>
		</table>

		<h2 class="mt-6 mb-2 text-xs font-medium text-muted-foreground">Result</h2>
		{#if d.computed_at && d.bounds}
			<table class="kv">
				<tbody>
					<tr><td>Computed</td><td class="num">{new Date(d.computed_at).toLocaleString('sv-SE')} in {d.compute_time}</td></tr>
					<tr><td>Bounds N / S</td><td class="num">{d.bounds.north.toFixed(5)} / {d.bounds.south.toFixed(5)}</td></tr>
					<tr><td>Bounds W / E</td><td class="num">{d.bounds.west.toFixed(5)} / {d.bounds.east.toFixed(5)}</td></tr>
					<tr>
						<td>Raster</td>
						<td class="num"><a class="text-primary" href={d.image_url} data-sveltekit-reload>PNG</a>, {d.resolution_m.toFixed(0)} m per pixel</td>
					</tr>
				</tbody>
			</table>
		{:else if d.job.state === 'idle'}
			<p class="text-muted-foreground">
				Not computed yet.{#if !shared}&nbsp;Press Compute.{/if}
			</p>
		{/if}

		<h2 class="mt-6 mb-2 text-xs font-medium text-muted-foreground">Legend (link margin)</h2>
		<LegendTable c={d} />
		{#if legend}<p class="mt-2">{@render legend()}</p>{/if}
	</div>
	<div>
		<!-- a new raster or legend redraws the map -->
		{#key `${d.image_url} ${d.legend.map((b) => b.min_db + b.color).join()}`}
			<CoverageMap {d} {save} />
		{/key}
	</div>
</div>
