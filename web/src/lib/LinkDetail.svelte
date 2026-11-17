<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { LinkDetail } from '#lib/api.ts';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { day, offBoresight, signed } from '#lib/format.ts';
	import { linkMap } from '#lib/map.ts';
	import PageHead from '#lib/PageHead.svelte';
	import StatTiles from '#lib/StatTiles.svelte';

	// path is where the report lives: /links/<id> for the owner, /s/l/<token> for a visitor; actions are the owner's buttons.
	let { d, path, shared = false, actions }: { d: LinkDetail; path: string; shared?: boolean; actions?: Snippet } = $props();
	const r = $derived(d.analysis);
	let el: HTMLDivElement;

	$effect(() => {
		const m = linkMap(el, d.site_a, d.site_b);
		return () => m.remove();
	});
</script>

<PageHead title={d.name}>
	{@render actions?.()}
	<Button variant="outline" href="{path}/report.pdf" target="_blank" rel="noopener" data-sveltekit-reload>Print report</Button>
</PageHead>
{#if shared}
	<Alert.Root class="mb-4"
		>Shared read-only view{#if d.share}, valid until {day(d.share.expires_at)}{/if}.</Alert.Root
	>
{/if}
{#if r}
	<StatTiles
		tiles={[
			{
				label: 'Link margin, median',
				value: `${signed(r.margin_db)} dB`,
				sub: `${signed(r.margin_rel_db)} dB ${d.reliability_pct.toFixed(0)} % of the time`,
				tone: r.margin_db >= 0 ? 'ok' : 'bad'
			},
			{
				label: 'Received level',
				value: `${r.rx_level_dbm.toFixed(1)} dBm`,
				sub: d.measured_rx_dbm != null ? `measured ${d.measured_rx_dbm.toFixed(1)} dBm` : undefined
			},
			{ label: 'Distance', value: `${(r.distance_m / 1000).toFixed(2)} km`, sub: `bearing ${r.bearing_deg.toFixed(1)} deg` },
			{ label: 'Path loss (ITM)', value: `${r.path_loss_db.toFixed(1)} dB`, sub: r.mode }
		]}
	/>
{/if}
<div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-2">
	<div>
		<h2 class="mb-2 text-xs font-medium text-muted-foreground">Sites</h2>
		<table class="kv">
			<tbody>
				<tr><th></th><th>A (TX)</th><th>B (RX)</th></tr>
				<tr><td>Name</td><td>{d.site_a.name}</td><td>{d.site_b.name}</td></tr>
				<tr>
					<td>Position</td>
					<td class="font-mono text-xs">{d.site_a.lat.toFixed(5)}, {d.site_a.lon.toFixed(5)}</td>
					<td class="font-mono text-xs">{d.site_b.lat.toFixed(5)}, {d.site_b.lon.toFixed(5)}</td>
				</tr>
				<tr>
					<td>Antenna above ground</td>
					<td class="font-mono">{d.site_a.antenna_height_m.toFixed(1)} m</td>
					<td class="font-mono">{d.site_b.antenna_height_m.toFixed(1)} m</td>
				</tr>
				{#if r}
					<tr
						><td>Ground elevation</td><td class="font-mono">{r.terrain_a_m.toFixed(0)} m</td><td class="font-mono">{r.terrain_b_m.toFixed(0)} m</td
						></tr
					>
				{/if}
			</tbody>
		</table>

		<h2 class="mt-6 mb-2 text-xs font-medium text-muted-foreground">Radio</h2>
		<table class="kv">
			<tbody>
				<tr><td>Frequency</td><td class="num">{d.frequency_mhz.toFixed(3)} MHz</td></tr>
				<tr><td>Polarization</td><td class="num">{d.polarization}</td></tr>
				<tr><td>TX power</td><td class="num">{d.tx_power_dbm.toFixed(1)} dBm</td></tr>
				<tr><td>TX gain / line loss</td><td class="num">{(d.tx_gain_dbi ?? 0).toFixed(1)} dBi / {(d.tx_line_loss_db ?? 0).toFixed(1)} dB</td></tr>
				<tr><td>RX gain / line loss</td><td class="num">{(d.rx_gain_dbi ?? 0).toFixed(1)} dBi / {(d.rx_line_loss_db ?? 0).toFixed(1)} dB</td></tr>
				<tr><td>RX sensitivity</td><td class="num">{d.rx_sensitivity_dbm.toFixed(1)} dBm</td></tr>
				{#each [['TX', d.tx_antenna, d.tx_aim_deg, d.tx_azimuth_deg, d.tx_tilt_deg, 'B'], ['RX', d.rx_antenna, d.rx_aim_deg, d.rx_azimuth_deg, d.rx_tilt_deg, 'A']] as const as [end, name, aim, set, tilt, other] (end)}
					<tr>
						<td>{end} pattern</td>
						<td class="num">
							{#if name}
								{name}, {aim.toFixed(0)} deg{#if set == null}&nbsp;(towards {other}){/if}{#if tilt}, tilt {tilt.toFixed(1)} deg{/if}
							{:else}<span class="text-muted-foreground">omnidirectional</span>{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>

		<h2 class="mt-6 mb-2 text-xs font-medium text-muted-foreground">Result</h2>
		{#if !r}
			<Alert.Root variant="destructive">{d.error}</Alert.Root>
		{:else}
			<table class="kv">
				<tbody>
					<tr><td>Distance</td><td class="num">{(r.distance_m / 1000).toFixed(2)} km</td></tr>
					<tr><td>Bearing A to B</td><td class="num">{r.bearing_deg.toFixed(1)} deg</td></tr>
					<tr><td>Free space loss</td><td class="num">{r.free_space_loss_db.toFixed(1)} dB</td></tr>
					<tr><td>Path loss (ITM), median</td><td class="num">{r.path_loss_db.toFixed(1)} dB</td></tr>
					<tr><td>Path loss, {d.reliability_pct.toFixed(0)} % of the time</td><td class="num">{r.path_loss_rel_db.toFixed(1)} dB</td></tr>
					<tr><td>ITM propagation mode</td><td class="num">{r.mode}</td></tr>
					{#if d.tx_antenna}
						<tr>
							<td>TX pattern loss</td>
							<td class="num">
								{r.tx_pattern_loss_db.toFixed(1)} dB
								<span class="text-muted-foreground">(take-off {signed(r.tx_takeoff_deg)} deg, {offBoresight(r.tx_off_boresight_deg)})</span>
							</td>
						</tr>
					{/if}
					{#if d.rx_antenna}
						<tr>
							<td>RX pattern loss</td>
							<td class="num">
								{r.rx_pattern_loss_db.toFixed(1)} dB
								<span class="text-muted-foreground">(take-off {signed(r.rx_takeoff_deg)} deg, {offBoresight(r.rx_off_boresight_deg)})</span>
							</td>
						</tr>
					{/if}
					{#if r.has_canopy}
						<tr>
							<td>Vegetation loss (ITU-R P.833)</td>
							<td class="num">
								{r.vegetation_loss_db.toFixed(1)} dB
								<span class="text-muted-foreground">({r.canopy_path_a_m.toFixed(0)} m at A, {r.canopy_path_b_m.toFixed(0)} m at B in canopy)</span>
							</td>
						</tr>
					{/if}
					{#if r.clutter_loss_db}<tr><td>Extra path loss</td><td class="num">{r.clutter_loss_db.toFixed(1)} dB</td></tr>{/if}
					<tr><td>EIRP</td><td class="num">{r.eirp_dbm.toFixed(1)} dBm</td></tr>
					<tr><td>Received level</td><td class="num">{r.rx_level_dbm.toFixed(1)} dBm</td></tr>
					{#if d.measured_rx_dbm != null}
						{@const delta = d.measured_rx_dbm - r.rx_level_dbm}
						<tr>
							<td>Measured level</td>
							<td class="num">
								{d.measured_rx_dbm.toFixed(1)} dBm <span class={delta < 0 ? 'text-warn' : 'text-muted-foreground'}>({signed(delta)} dB)</span>
							</td>
						</tr>
					{/if}
					<tr>
						<td>Worst first Fresnel zone clearance</td>
						<td class="num">{r.worst_clearance_pct.toFixed(0)} % at {(r.worst_clearance_dist_m / 1000).toFixed(2)} km</td>
					</tr>
				</tbody>
			</table>
			{#if r.warnings.length}
				<Alert.Root class="mt-3 border-warn/50 text-warn">
					<ul class="list-disc pl-4">
						{#each r.warnings as w (w)}<li>{w}</li>{/each}
					</ul>
				</Alert.Root>
			{/if}
		{/if}
	</div>
	<div class="map" bind:this={el}></div>
</div>
{#if r}
	<div class="panel mt-5 rounded-lg border bg-card p-3">{@html r.profile_svg}</div>
	<p class="mt-2 text-xs text-muted-foreground">
		Terrain drawn with 4/3 earth curvature.{#if r.has_canopy}&nbsp;Green: tree canopy.{/if} Red: direct ray. Blue dashed: first Fresnel zone.
	</p>
{/if}
