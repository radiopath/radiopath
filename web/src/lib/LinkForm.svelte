<script lang="ts" module>
	import type { Link } from '#lib/api.ts';

	export type LinkValues = Record<
		| 'name'
		| 'site_a_id'
		| 'site_b_id'
		| 'frequency_mhz'
		| 'clutter_loss_db'
		| 'polarization'
		| 'tx_power_dbm'
		| 'tx_gain_dbi'
		| 'tx_line_loss_db'
		| 'tx_antenna_id'
		| 'tx_azimuth_deg'
		| 'tx_tilt_deg'
		| 'rx_gain_dbi'
		| 'rx_line_loss_db'
		| 'rx_sensitivity_dbm'
		| 'rx_antenna_id'
		| 'rx_azimuth_deg'
		| 'rx_tilt_deg'
		| 'measured_rx_dbm',
		string
	>;

	/** A number as form text; empty for a missing value. */
	const str = (v: number | null | undefined) => (v == null ? '' : String(v));

	/** The form values of a link, or the defaults of a new one. */
	export function linkValues(l?: Link): LinkValues {
		if (!l)
			return {
				name: '',
				site_a_id: '',
				site_b_id: '',
				frequency_mhz: '145',
				clutter_loss_db: '0',
				polarization: 'vertical',
				tx_power_dbm: '40',
				tx_gain_dbi: '0',
				tx_line_loss_db: '0',
				tx_antenna_id: '',
				tx_azimuth_deg: '',
				tx_tilt_deg: '0',
				rx_gain_dbi: '0',
				rx_line_loss_db: '0',
				rx_sensitivity_dbm: '-120',
				rx_antenna_id: '',
				rx_azimuth_deg: '',
				rx_tilt_deg: '0',
				measured_rx_dbm: ''
			};
		return {
			name: l.name,
			site_a_id: str(l.site_a_id),
			site_b_id: str(l.site_b_id),
			frequency_mhz: str(l.frequency_mhz),
			clutter_loss_db: str(l.clutter_loss_db ?? 0),
			polarization: l.polarization,
			tx_power_dbm: str(l.tx_power_dbm),
			tx_gain_dbi: str(l.tx_gain_dbi ?? 0),
			tx_line_loss_db: str(l.tx_line_loss_db ?? 0),
			tx_antenna_id: str(l.tx_antenna_id),
			tx_azimuth_deg: str(l.tx_azimuth_deg),
			tx_tilt_deg: str(l.tx_tilt_deg ?? 0),
			rx_gain_dbi: str(l.rx_gain_dbi ?? 0),
			rx_line_loss_db: str(l.rx_line_loss_db ?? 0),
			rx_sensitivity_dbm: str(l.rx_sensitivity_dbm),
			rx_antenna_id: str(l.rx_antenna_id),
			rx_azimuth_deg: str(l.rx_azimuth_deg),
			rx_tilt_deg: str(l.rx_tilt_deg ?? 0),
			measured_rx_dbm: str(l.measured_rx_dbm)
		};
	}
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { api, call, parseNum, type LinkInput } from '#lib/api.ts';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { NativeSelect } from '#lib/components/ui/native-select/index.ts';
	import Field from '#lib/Field.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';

	let { id, initial }: { id?: number; initial: LinkValues } = $props();
	const queryClient = useQueryClient();
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));
	const antennas = createQuery(() => ({ queryKey: ['antennas'], queryFn: () => call(api.GET('/api/antennas')) }));
	// svelte-ignore state_referenced_locally
	let v = $state({ ...initial });
	let busy = $state(false);
	const txAntenna = $derived(antennas.data?.find((a) => String(a.id) === v.tx_antenna_id));
	const rxAntenna = $derived(antennas.data?.find((a) => String(a.id) === v.rx_antenna_id));

	// A new link starts from the first two sites.
	$effect(() => {
		const list = sites.data;
		if (!list?.length) return;
		if (!v.site_a_id) v.site_a_id = String(list[0].id);
		if (!v.site_b_id) v.site_b_id = String(list[Math.min(1, list.length - 1)].id);
	});

	const labels: Partial<Record<keyof LinkValues, string>> = {
		frequency_mhz: 'Frequency',
		clutter_loss_db: 'Extra path loss',
		tx_power_dbm: 'TX power',
		tx_gain_dbi: 'TX gain',
		tx_line_loss_db: 'TX line loss',
		tx_azimuth_deg: 'TX azimuth',
		tx_tilt_deg: 'TX tilt',
		rx_gain_dbi: 'RX gain',
		rx_line_loss_db: 'RX line loss',
		rx_sensitivity_dbm: 'RX sensitivity',
		rx_azimuth_deg: 'RX azimuth',
		rx_tilt_deg: 'RX tilt',
		measured_rx_dbm: 'Measured RX level'
	};

	/** Turns the text fields into the API body; empty optional fields are left out, fields that are no number give one message each. */
	function body(): LinkInput {
		const n: Partial<Record<keyof LinkValues, number>> = {};
		const nan: string[] = [];
		for (const [k, label] of Object.entries(labels) as [keyof LinkValues, string][]) {
			if (v[k].trim() === '') continue;
			n[k] = parseNum(v[k]);
			if (isNaN(n[k]!)) nan.push(`${label}: not a number`);
		}
		if (nan.length) throw invalid(...nan);
		return {
			name: v.name,
			site_a_id: Number(v.site_a_id),
			site_b_id: Number(v.site_b_id),
			frequency_mhz: n.frequency_mhz ?? NaN,
			polarization: v.polarization as LinkInput['polarization'],
			clutter_loss_db: n.clutter_loss_db,
			tx_power_dbm: n.tx_power_dbm ?? NaN,
			tx_gain_dbi: n.tx_gain_dbi,
			tx_line_loss_db: n.tx_line_loss_db,
			tx_antenna_id: v.tx_antenna_id ? Number(v.tx_antenna_id) : undefined,
			tx_azimuth_deg: n.tx_azimuth_deg,
			tx_tilt_deg: n.tx_tilt_deg,
			rx_gain_dbi: n.rx_gain_dbi,
			rx_line_loss_db: n.rx_line_loss_db,
			rx_sensitivity_dbm: n.rx_sensitivity_dbm ?? NaN,
			rx_antenna_id: v.rx_antenna_id ? Number(v.rx_antenna_id) : undefined,
			rx_azimuth_deg: n.rx_azimuth_deg,
			rx_tilt_deg: n.rx_tilt_deg,
			measured_rx_dbm: n.measured_rx_dbm
		};
	}

	/** Saves the link and opens its page. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const b = body();
			const saved = id
				? await call(api.PUT('/api/links/{id}', { params: { path: { id } }, body: b }))
				: await call(api.POST('/api/links', { body: b }));
			toast.success(`Link ${saved.name} saved`);
			await queryClient.invalidateQueries({ queryKey: ['links'] });
			await goto(`/links/${saved.id}`);
		} catch (err) {
			notifyError(err);
			busy = false;
		}
	}
</script>

{#if sites.data && sites.data.length < 2}
	<Alert.Root class="mb-4 border-warn/50 text-warn"
		><span>Create at least two <a class="underline" href="/sites/new">sites</a> first.</span></Alert.Root
	>
{/if}
{#snippet num(name: keyof LinkValues, label: string, placeholder = '')}
	<Field wide {label}><Input class="max-w-68 font-mono" {name} inputmode="decimal" {placeholder} bind:value={v[name]} /></Field>
{/snippet}
{#snippet gain(name: 'tx_gain_dbi' | 'rx_gain_dbi', antenna: typeof txAntenna)}
	<Field wide label="Antenna gain (dBi)">
		{#if antenna}
			<Input class="max-w-68 font-mono" {name} readonly value={antenna.gain_dbi.toFixed(1)} title="from the selected pattern" />
		{:else}
			<Input class="max-w-68 font-mono" {name} inputmode="decimal" bind:value={v[name]} />
		{/if}
	</Field>
{/snippet}
{#snippet pattern(name: 'tx_antenna_id' | 'rx_antenna_id')}
	<Field wide label="Pattern">
		<NativeSelect class="w-68" {name} bind:value={v[name]}>
			<option value="">omnidirectional</option>
			{#each antennas.data ?? [] as a (a.id)}<option value={String(a.id)}>{a.name}</option>{/each}
		</NativeSelect>
	</Field>
{/snippet}

<form class="grid gap-6 rounded-lg border bg-card p-4" onsubmit={submit}>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Link</legend>
		<Field wide label="Name"><Input class="max-w-68" name="name" required bind:value={v.name} /></Field>
		{#each [['site_a_id', 'Site A (TX)'], ['site_b_id', 'Site B (RX)']] as const as [name, label] (name)}
			<Field wide {label}>
				<NativeSelect class="w-68" {name} bind:value={v[name]}>
					{#each sites.data ?? [] as s (s.id)}<option value={String(s.id)}>{s.name}</option>{/each}
				</NativeSelect>
			</Field>
		{/each}
		{@render num('frequency_mhz', 'Frequency (MHz)')}
		<Field wide label="Extra path loss (dB)">
			<span class="flex items-center gap-2">
				<Input class="max-w-68 font-mono" name="clutter_loss_db" inputmode="decimal" bind:value={v.clutter_loss_db} />
				<span
					class="inline-flex size-4 cursor-help items-center justify-center border text-[10px] text-muted-foreground"
					title="Extra loss on top of the model: buildings, screening, or woods the canopy layer does not have. Trees at both ends are computed from the canopy layer (ITU-R P.833) when one is configured and shown on the detail page. Without one, a rule of thumb at 5 to 6 GHz is about 1 dB per metre of foliage, saturating near 30 to 40 dB behind woodland; a few dB at VHF. Best set from the difference between the measured and the predicted level below."
					>?</span
				>
			</span>
		</Field>
		<Field wide label="Polarization">
			<NativeSelect class="w-68" name="polarization" bind:value={v.polarization}>
				<option value="vertical">vertical</option>
				<option value="horizontal">horizontal</option>
			</NativeSelect>
		</Field>
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Transmitter (A)</legend>
		{@render num('tx_power_dbm', 'TX power (dBm)')}
		{@render gain('tx_gain_dbi', txAntenna)}
		{@render num('tx_line_loss_db', 'Line loss (dB)')}
		{@render pattern('tx_antenna_id')}
		{@render num('tx_azimuth_deg', 'Main lobe azimuth (deg)', 'towards B')}
		{@render num('tx_tilt_deg', 'Mechanical tilt (deg)', '0')}
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Receiver (B)</legend>
		{@render gain('rx_gain_dbi', rxAntenna)}
		{@render num('rx_line_loss_db', 'Line loss (dB)')}
		{@render num('rx_sensitivity_dbm', 'Sensitivity (dBm)')}
		{@render pattern('rx_antenna_id')}
		{@render num('rx_azimuth_deg', 'Main lobe azimuth (deg)', 'towards A')}
		{@render num('rx_tilt_deg', 'Mechanical tilt (deg)', '0')}
		<p class="text-xs text-muted-foreground">
			An empty azimuth aims the antenna at the other site. The tilt is positive downwards; the elevation cut of the pattern is read at the take-off
			angle of the ray. The gain above is the main lobe gain; the pattern only takes away from it.
		</p>
	</fieldset>
	<fieldset class="grid gap-3">
		<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Measurement</legend>
		{@render num('measured_rx_dbm', 'Measured RX level (dBm)', 'optional')}
		<p class="text-xs text-muted-foreground">
			The level actually read at site B once the link is up. The detail page shows how far the prediction is off.
		</p>
	</fieldset>
	<div class="flex items-center gap-3">
		<Button type="submit" disabled={busy}>Save</Button>
		<a class="text-primary" href="/links">Cancel</a>
	</div>
</form>
