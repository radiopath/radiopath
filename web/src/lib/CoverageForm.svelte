<script lang="ts" module>
	import type { Coverage } from '#lib/api.ts';

	export type CoverageValues = Record<
		| 'name'
		| 'site_id'
		| 'frequency_mhz'
		| 'polarization'
		| 'tx_power_dbm'
		| 'tx_gain_dbi'
		| 'tx_line_loss_db'
		| 'tx_antenna_id'
		| 'tx_azimuth_deg'
		| 'tx_tilt_deg'
		| 'rx_height_m'
		| 'rx_gain_dbi'
		| 'rx_line_loss_db'
		| 'rx_sensitivity_dbm'
		| 'range_km'
		| 'resolution_m',
		string
	>;

	/** A number as form text; empty for a missing value. */
	const str = (v: number | null | undefined) => (v == null ? '' : String(v));

	/** The form values of a coverage, or the defaults of a new one. */
	export function coverageValues(c?: Coverage): CoverageValues {
		if (!c)
			return {
				name: '',
				site_id: '',
				frequency_mhz: '145',
				polarization: 'vertical',
				tx_power_dbm: '40',
				tx_gain_dbi: '0',
				tx_line_loss_db: '0',
				tx_antenna_id: '',
				tx_azimuth_deg: '0',
				tx_tilt_deg: '0',
				rx_height_m: '2',
				rx_gain_dbi: '0',
				rx_line_loss_db: '0',
				rx_sensitivity_dbm: '-120',
				range_km: '30',
				resolution_m: '100'
			};
		return {
			name: c.name,
			site_id: str(c.site_id),
			frequency_mhz: str(c.frequency_mhz),
			polarization: c.polarization,
			tx_power_dbm: str(c.tx_power_dbm),
			tx_gain_dbi: str(c.tx_gain_dbi ?? 0),
			tx_line_loss_db: str(c.tx_line_loss_db ?? 0),
			tx_antenna_id: str(c.tx_antenna_id),
			tx_azimuth_deg: str(c.tx_azimuth_deg ?? 0),
			tx_tilt_deg: str(c.tx_tilt_deg ?? 0),
			rx_height_m: str(c.rx_height_m),
			rx_gain_dbi: str(c.rx_gain_dbi ?? 0),
			rx_line_loss_db: str(c.rx_line_loss_db ?? 0),
			rx_sensitivity_dbm: str(c.rx_sensitivity_dbm),
			range_km: str(c.range_km),
			resolution_m: str(c.resolution_m)
		};
	}
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { api, call, parseNum, type CoverageInput } from '#lib/api.ts';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { NativeSelect } from '#lib/components/ui/native-select/index.ts';
	import Field from '#lib/Field.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';
	import LegendTable from '#lib/LegendTable.svelte';

	// legend is the custom legend of the coverage a new one is copied from.
	let { id, initial, legend }: { id?: number; initial: CoverageValues; legend?: Pick<Coverage, 'legend' | 'legend_floor'> } = $props();
	const queryClient = useQueryClient();
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));
	const antennas = createQuery(() => ({ queryKey: ['antennas'], queryFn: () => call(api.GET('/api/antennas')) }));
	// svelte-ignore state_referenced_locally
	let v = $state({ ...initial });
	let busy = $state(false);
	const txAntenna = $derived(antennas.data?.find((a) => String(a.id) === v.tx_antenna_id));

	// A new coverage starts at the first site.
	$effect(() => {
		if (!v.site_id && sites.data?.length) v.site_id = String(sites.data[0].id);
	});

	const labels: Partial<Record<keyof CoverageValues, string>> = {
		frequency_mhz: 'Frequency',
		tx_power_dbm: 'TX power',
		tx_gain_dbi: 'TX gain',
		tx_line_loss_db: 'TX line loss',
		tx_azimuth_deg: 'TX azimuth',
		tx_tilt_deg: 'TX tilt',
		rx_height_m: 'RX height',
		rx_gain_dbi: 'RX gain',
		rx_line_loss_db: 'RX line loss',
		rx_sensitivity_dbm: 'RX sensitivity',
		range_km: 'Range',
		resolution_m: 'Resolution'
	};

	/** Turns the text fields into the API body; empty optional fields count as 0, fields that are no number give one message each. */
	function body(): CoverageInput {
		const n: Partial<Record<keyof CoverageValues, number>> = {};
		const nan: string[] = [];
		for (const [k, label] of Object.entries(labels) as [keyof CoverageValues, string][]) {
			if (v[k].trim() === '') continue;
			n[k] = parseNum(v[k]);
			if (isNaN(n[k]!)) nan.push(`${label}: not a number`);
		}
		if (nan.length) throw invalid(...nan);
		return {
			name: v.name,
			site_id: Number(v.site_id),
			frequency_mhz: n.frequency_mhz ?? NaN,
			polarization: v.polarization as CoverageInput['polarization'],
			tx_power_dbm: n.tx_power_dbm ?? NaN,
			tx_gain_dbi: n.tx_gain_dbi,
			tx_line_loss_db: n.tx_line_loss_db,
			tx_antenna_id: v.tx_antenna_id ? Number(v.tx_antenna_id) : undefined,
			tx_azimuth_deg: n.tx_azimuth_deg,
			tx_tilt_deg: n.tx_tilt_deg,
			rx_height_m: n.rx_height_m ?? NaN,
			rx_gain_dbi: n.rx_gain_dbi,
			rx_line_loss_db: n.rx_line_loss_db,
			rx_sensitivity_dbm: n.rx_sensitivity_dbm ?? NaN,
			range_km: n.range_km ?? NaN,
			resolution_m: n.resolution_m ?? NaN
		};
	}

	/** Saves the coverage and opens its page; a copy takes the legend along. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const b = body();
			const saved = id
				? await call(api.PUT('/api/coverages/{id}', { params: { path: { id } }, body: b }))
				: await call(api.POST('/api/coverages', { body: { ...b, legend: legend?.legend.map((l) => ({ min_db: l.min_db, color: l.color })) } }));
			toast.success(`Coverage ${saved.name} saved`);
			await queryClient.invalidateQueries({ queryKey: ['coverages'] });
			await goto(`/coverages/${saved.id}`);
		} catch (err) {
			notifyError(err);
			busy = false;
		}
	}
</script>

{#if sites.data && !sites.data.length}
	<Alert.Root class="mb-4 border-warn/50 text-warn"><span>Create a <a class="underline" href="/sites/new">site</a> first.</span></Alert.Root>
{/if}
{#snippet num(name: keyof CoverageValues, label: string, placeholder = '')}
	<Field wide {label}><Input class="max-w-68 font-mono" {name} inputmode="decimal" {placeholder} bind:value={v[name]} /></Field>
{/snippet}

<form class="grid gap-6 rounded-lg border bg-card p-4" onsubmit={submit}>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Coverage</legend>
		<Field wide label="Name"><Input class="max-w-68" name="name" required bind:value={v.name} /></Field>
		<Field wide label="Site (TX)">
			<NativeSelect class="w-68" name="site_id" bind:value={v.site_id}>
				{#each sites.data ?? [] as s (s.id)}<option value={String(s.id)}>{s.name}</option>{/each}
			</NativeSelect>
		</Field>
		{@render num('frequency_mhz', 'Frequency (MHz)')}
		<Field wide label="Polarization">
			<NativeSelect class="w-68" name="polarization" bind:value={v.polarization}>
				<option value="vertical">vertical</option>
				<option value="horizontal">horizontal</option>
			</NativeSelect>
		</Field>
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Transmitter</legend>
		{@render num('tx_power_dbm', 'TX power (dBm)')}
		<Field wide label="Antenna gain (dBi)">
			{#if txAntenna}
				<Input class="max-w-68 font-mono" name="tx_gain_dbi" readonly value={txAntenna.gain_dbi.toFixed(1)} title="from the selected pattern" />
			{:else}
				<Input class="max-w-68 font-mono" name="tx_gain_dbi" inputmode="decimal" bind:value={v.tx_gain_dbi} />
			{/if}
		</Field>
		{@render num('tx_line_loss_db', 'Line loss (dB)')}
		<Field wide label="Pattern">
			<NativeSelect class="w-68" name="tx_antenna_id" bind:value={v.tx_antenna_id}>
				<option value="">omnidirectional</option>
				{#each antennas.data ?? [] as a (a.id)}<option value={String(a.id)}>{a.name}</option>{/each}
			</NativeSelect>
		</Field>
		{@render num('tx_azimuth_deg', 'Main lobe azimuth (deg)')}
		{@render num('tx_tilt_deg', 'Mechanical tilt (deg)', '0')}
		<p class="text-xs text-muted-foreground">
			The gain above is the main lobe gain; the pattern only takes away from it. 0 deg is north, the tilt is positive downwards.
		</p>
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Receiver (mobile / portable)</legend>
		{@render num('rx_height_m', 'Antenna height above ground (m)')}
		{@render num('rx_gain_dbi', 'Antenna gain (dBi)')}
		{@render num('rx_line_loss_db', 'Line loss (dB)')}
		{@render num('rx_sensitivity_dbm', 'Sensitivity (dBm)')}
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Area</legend>
		{@render num('range_km', 'Range (km)')}
		{@render num('resolution_m', 'Resolution (m)')}
		<p class="text-xs text-muted-foreground">
			Range / resolution must not exceed 1000 pixels per radius. Radials are spaced one pixel apart at the outer edge (360 to 3600); compute time
			grows roughly with (range / resolution)&sup3;.
		</p>
	</fieldset>
	{#if legend}
		<fieldset class="grid gap-3">
			<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Legend, copied along</legend>
			<div class="max-w-md"><LegendTable c={legend} /></div>
		</fieldset>
	{/if}
	<div class="flex items-center gap-3">
		<Button type="submit" disabled={busy}>Save</Button>
		<a class="text-primary" href="/coverages">Cancel</a>
	</div>
</form>
