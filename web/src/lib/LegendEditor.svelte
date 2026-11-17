<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { api, call, parseNum, type CoverageDetail, type LegendBin } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Dialog from '#lib/components/ui/dialog/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { invalid, notifyError } from '#lib/notify.ts';

	let { d, onchange }: { d: CoverageDetail; onchange: () => Promise<unknown> } = $props();
	// The server allows this many bins; empty rows are dropped.
	const maxBins = 8;
	const blank = '#9ca3af';
	let open = $state(false);
	let rows = $state<{ min: string; color: string }[]>([]);

	/** Opens the dialog with the legend in use and empty rows up to maxBins. */
	function edit() {
		rows = Array.from({ length: maxBins }, (_, i) => {
			const b = d.legend[i];
			return b ? { min: String(b.min_db), color: b.color } : { min: '', color: blank };
		});
		open = true;
	}

	/** Stores the bins, an empty list resets to the default legend; the page reloads the coverage. */
	async function put(bins: LegendBin[]) {
		try {
			await call(api.PUT('/api/coverages/{id}/legend', { params: { path: { id: d.id } }, body: { bins } }));
			toast.success(bins.length ? 'Legend saved' : 'Legend reset to the defaults');
			await onchange();
			open = false;
		} catch (err) {
			notifyError(err);
		}
	}

	/** Saves the filled rows; thresholds that are no number are reported before asking the server. */
	function submit(e: SubmitEvent) {
		e.preventDefault();
		const filled = rows.map((r, i) => ({ ...r, n: i + 1 })).filter((r) => r.min.trim() !== '');
		const nan = filled.filter((r) => isNaN(parseNum(r.min))).map((r) => `Threshold ${r.n}: not a number`);
		if (nan.length) return notifyError(invalid(...nan));
		put(filled.map((r) => ({ min_db: parseNum(r.min), color: r.color })));
	}
</script>

<Button variant="outline" onclick={edit}>Edit legend</Button>

<Dialog.Root bind:open>
	<Dialog.Content showCloseButton={false} id="legend">
		<Dialog.Header><Dialog.Title>Legend</Dialog.Title></Dialog.Header>
		<p class="text-xs text-muted-foreground">
			A cell takes the colour of the highest threshold its link margin reaches; below every threshold, or without data, it stays transparent. Leave a
			threshold empty to drop the row. The legend applies at once, also to the merged view and the report; nothing is recomputed.
		</p>
		<form class="grid gap-3" onsubmit={submit}>
			<table class="kv">
				<thead><tr><th>Margin at least (dB)</th><th>Colour</th></tr></thead>
				<tbody>
					{#each rows as r, i (i)}
						<tr>
							<td><Input class="w-28 font-mono" name="min_{i + 1}" inputmode="decimal" bind:value={r.min} /></td>
							<td><input class="h-8 w-12 cursor-pointer border bg-card p-0.5" type="color" name="color_{i + 1}" bind:value={r.color} /></td>
						</tr>
					{/each}
				</tbody>
			</table>
			<div class="flex flex-wrap gap-3">
				<Button type="submit">Save</Button>
				<Button variant="outline" onclick={() => put([])}>Reset to defaults</Button>
				<Button variant="outline" onclick={() => (open = false)}>Close</Button>
			</div>
		</form>
	</Dialog.Content>
</Dialog.Root>
