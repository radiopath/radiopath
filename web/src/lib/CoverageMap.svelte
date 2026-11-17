<script lang="ts">
	import L from 'leaflet';
	import { onMount } from 'svelte';
	import type { CoverageDetail, CoverageView } from '#lib/api.ts';
	import { baseMap } from '#lib/map.ts';

	// save stores the view for the owner; a visitor's map has none and only shows what was saved.
	let { d, save }: { d: CoverageDetail; save?: (v: CoverageView) => void } = $props();
	// svelte-ignore state_referenced_locally
	let opacity = $state(d.view.opacity);
	// svelte-ignore state_referenced_locally
	let circles = $state(d.view.circles);
	// svelte-ignore state_referenced_locally
	let checked = $state(Object.fromEntries(d.overlays.map((o) => [o.id, o.checked])) as Record<number, boolean>);
	let el: HTMLDivElement;
	let m: L.Map;
	let circleLayer: L.LayerGroup;
	let base: L.ImageOverlay | null = null;
	let composite: L.ImageOverlay | null = null;
	let compositeSrc = '';
	let seq = 0;
	const extra: Record<number, { marker: L.Marker; circle: L.Circle }> = {};

	/** The ids of the ticked overlays. */
	const ticked = () => d.overlays.filter((o) => checked[o.id]).map((o) => o.id);

	/** Sets the opacity of every raster on the map. */
	function applyOpacity() {
		m.eachLayer((l) => {
			if (l instanceof L.ImageOverlay) l.setOpacity(opacity / 100);
		});
	}

	/** Stores the current view. */
	const store = () => save?.({ opacity, overlays: ticked(), circles });

	/** Removes the merged raster and frees its blob. */
	function dropComposite() {
		if (composite) m.removeLayer(composite);
		if (compositeSrc) URL.revokeObjectURL(compositeSrc);
		composite = null;
		compositeSrc = '';
	}

	/** Shows the own raster alone, or the server-side merge with the ticked overlays instead of it. */
	async function refresh() {
		const on = ticked();
		const n = ++seq;
		if (!on.length) {
			dropComposite();
			if (base && !m.hasLayer(base)) {
				base.addTo(m);
				applyOpacity();
			}
			return;
		}
		const res = await fetch(`${d.composite_url}?with=${on.join(',')}`).catch(() => null);
		if (!res?.ok || n !== seq) return;
		const b = res.headers.get('X-Raster-Bounds')!.split(',').map(Number); // S,W,N,E
		const blob = await res.blob();
		if (n !== seq) return;
		dropComposite();
		compositeSrc = URL.createObjectURL(blob);
		composite = L.imageOverlay(
			compositeSrc,
			[
				[b[0], b[1]],
				[b[2], b[3]]
			],
			{ opacity: opacity / 100 }
		).addTo(m);
		if (base) m.removeLayer(base);
	}

	/** Adds or removes the marker and range circle of an overlay's site. */
	function mark(id: number) {
		const o = d.overlays.find((x) => x.id === id)!;
		if (checked[id] && !extra[id]) {
			const ll = L.latLng(o.site.lat, o.site.lon);
			extra[id] = {
				marker: L.marker(ll).bindPopup(o.site.name).addTo(m),
				circle: L.circle(ll, { radius: o.range_km * 1000, className: 'range-circle', fill: false }).addTo(circleLayer)
			};
			const bounds = L.latLngBounds([o.bounds.south, o.bounds.west], [o.bounds.north, o.bounds.east]);
			if (!m.getBounds().intersects(bounds)) m.fitBounds(bounds);
		} else if (!checked[id] && extra[id]) {
			m.removeLayer(extra[id].marker);
			circleLayer.removeLayer(extra[id].circle);
			delete extra[id];
		}
	}

	onMount(() => {
		m = baseMap(el);
		circleLayer = L.layerGroup().addTo(m);
		const site = L.latLng(d.site.lat, d.site.lon);
		L.marker(site).addTo(m).bindPopup(d.site.name);
		m.fitBounds(site.toBounds(2 * d.range_km * 1000));
		L.circle(site, { radius: d.range_km * 1000, className: 'range-circle', fill: false }).addTo(circleLayer);
		if (d.image_url && d.bounds) {
			const b = d.bounds;
			base = L.imageOverlay(
				d.image_url,
				[
					[b.south, b.west],
					[b.north, b.east]
				],
				{ opacity: opacity / 100 }
			).addTo(m);
		}
		for (const id of ticked()) mark(id);
		if (ticked().length) refresh();
		if (!circles) m.removeLayer(circleLayer);
		return () => {
			dropComposite();
			m.remove();
		};
	});
</script>

<div class="map" bind:this={el}></div>
{#if d.image_url || d.overlays.length}
	<div class="mt-3 rounded-lg border bg-card p-3">
		<div class="flex flex-wrap items-center gap-3">
			<label class="text-xs text-muted-foreground" for="opacity">Raster opacity</label>
			<input
				class="min-w-0 flex-1 accent-primary sm:w-40 sm:flex-none"
				id="opacity"
				type="range"
				min="10"
				max="100"
				step="5"
				bind:value={opacity}
				oninput={applyOpacity}
				onchange={store}
			/>
			<span class="font-mono text-xs">{opacity} %</span>
			<label class="flex min-h-10 items-center gap-2 text-xs text-muted-foreground">
				<input
					id="circles"
					type="checkbox"
					class="size-4 accent-primary"
					bind:checked={circles}
					onchange={() => {
						if (circles) m.addLayer(circleLayer);
						else m.removeLayer(circleLayer);
						store();
					}}
				/> Range circles
			</label>
		</div>
		{#if d.overlays.length}
			<div class="mt-3 border-t pt-2">
				<p class="mb-1 text-xs text-muted-foreground">{save ? 'Also show' : 'Also shown'}</p>
				<div class="flex flex-col gap-1">
					{#each d.overlays as o (o.id)}
						<label class="flex min-h-10 flex-wrap items-center gap-2 text-[13px]">
							<input
								type="checkbox"
								class="size-4 accent-primary"
								data-overlay={o.id}
								bind:checked={checked[o.id]}
								onchange={() => {
									mark(o.id);
									refresh();
									store();
								}}
							/>
							{o.name}<span class="font-mono text-xs text-muted-foreground">{o.site.name}, {o.frequency_mhz.toFixed(3)} MHz</span>
						</label>
					{/each}
				</div>
				<p class="mt-2 text-xs text-muted-foreground">Same legend for all rasters; where they overlap, the best margin is shown.</p>
			</div>
		{/if}
	</div>
{/if}
