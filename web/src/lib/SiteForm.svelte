<script lang="ts">
	import { goto } from '$app/navigation';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, parseNum, type SiteInput } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';
	import { sitePicker } from '#lib/map.ts';

	let { id, initial }: { id?: number; initial: { name: string; lat: string; lon: string; antenna_height_m: string } } = $props();
	const queryClient = useQueryClient();
	// Text fields, so a typo shows up as "not a number" instead of being swallowed by the browser. The form is rendered once its data is there.
	// svelte-ignore state_referenced_locally
	let v = $state({ ...initial });
	let busy = $state(false);
	let el: HTMLDivElement;
	let picker: ReturnType<typeof sitePicker>;

	$effect(() => {
		picker = sitePicker(el, (lat, lon) => ((v.lat = String(lat)), (v.lon = String(lon))));
		const lat = parseNum(initial.lat),
			lon = parseNum(initial.lon);
		if (!isNaN(lat) && !isNaN(lon)) picker.place(lat, lon);
		return () => picker.remove();
	});

	/** Moves the marker to coordinates typed into the fields. */
	function typed() {
		const lat = parseNum(v.lat),
			lon = parseNum(v.lon);
		if (!isNaN(lat) && !isNaN(lon)) picker.place(lat, lon);
	}

	/** Saves the site and goes back to the list; fields that are no number are reported before asking the server. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const body: SiteInput = { name: v.name, lat: parseNum(v.lat), lon: parseNum(v.lon), antenna_height_m: parseNum(v.antenna_height_m) };
		const labels = { lat: 'Latitude', lon: 'Longitude', antenna_height_m: 'Antenna height' } as const;
		const nan = (Object.keys(labels) as (keyof typeof labels)[]).filter((k) => isNaN(body[k])).map((k) => `${labels[k]}: not a number`);
		if (nan.length) return notifyError(invalid(...nan));
		busy = true;
		try {
			const saved = id ? await call(api.PUT('/api/sites/{id}', { params: { path: { id } }, body })) : await call(api.POST('/api/sites', { body }));
			toast.success(`Site ${saved.name} saved`);
			await queryClient.invalidateQueries({ queryKey: ['sites'] });
			await goto('/sites');
		} catch (err) {
			notifyError(err);
			busy = false;
		}
	}
</script>

<div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
	<form class="grid gap-3 rounded-lg border bg-card p-4" onsubmit={submit}>
		<Field label="Name"><Input name="name" required bind:value={v.name} /></Field>
		<Field label="Latitude (deg, WGS84)"
			><Input class="font-mono" name="lat" inputmode="decimal" required bind:value={v.lat} onchange={typed} /></Field
		>
		<Field label="Longitude (deg, WGS84)"
			><Input class="font-mono" name="lon" inputmode="decimal" required bind:value={v.lon} onchange={typed} /></Field
		>
		<Field label="Antenna height above ground (m)">
			<Input class="font-mono" name="antenna_height_m" inputmode="decimal" required bind:value={v.antenna_height_m} />
		</Field>
		<div class="mt-2 flex items-center gap-3">
			<Button type="submit" disabled={busy}>Save</Button>
			<a class="text-primary" href="/sites">Cancel</a>
		</div>
		<p class="text-xs text-muted-foreground">Click the map to set the position, or drag the marker.</p>
	</form>
	<div class="map" bind:this={el}></div>
</div>
