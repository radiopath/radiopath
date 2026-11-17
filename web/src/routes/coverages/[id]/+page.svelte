<script lang="ts">
	import Copy from '@lucide/svelte/icons/copy';
	import Pencil from '@lucide/svelte/icons/pencil';
	import { page } from '$app/state';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, type CoverageView } from '#lib/api.ts';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import CoverageDetail from '#lib/CoverageDetail.svelte';
	import { clock } from '#lib/format.ts';
	import DetailSkeleton from '#lib/DetailSkeleton.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import JobBadge from '#lib/JobBadge.svelte';
	import LegendEditor from '#lib/LegendEditor.svelte';
	import { notifyError } from '#lib/notify.ts';
	import RowActions from '#lib/RowActions.svelte';
	import ShareDialog from '#lib/ShareDialog.svelte';

	const id = Number(page.params.id);
	const path = { params: { path: { id } } };
	const queryClient = useQueryClient();
	const c = createQuery(() => ({ queryKey: ['coverages', id], queryFn: () => call(api.GET('/api/coverages/{id}', path)) }));
	// The job as the event stream reports it, ahead of the next load of the coverage.
	let live: { state: string; since?: string } | null = $state(null);
	const job = $derived(live ?? (c.data ? { state: c.data.job.state, since: c.data.job.started_at ?? c.data.job.queued_at ?? undefined } : null));
	const busy = $derived(job?.state === 'queued' || job?.state === 'running');

	/** Reloads the coverage and the list. */
	const reload = () => queryClient.invalidateQueries({ queryKey: ['coverages'] });

	// Follows a queued or running job; the final state reloads the coverage with its result.
	$effect(() => {
		if (!busy) return;
		const es = new EventSource(`/coverages/${id}/events`);
		es.onmessage = (ev) => {
			const j = JSON.parse(ev.data);
			live = j;
			if (j.state !== 'queued' && j.state !== 'running') {
				es.close();
				if (j.state === 'done') toast.success('Coverage computed');
				else if (j.state === 'failed') toast.error(j.error);
				reload().then(() => (live = null));
			}
		};
		// the server closes the stream after the final state; EventSource would reconnect forever
		es.onerror = () => {
			if (es.readyState === EventSource.CLOSED) reload().then(() => (live = null));
		};
		return () => es.close();
	});

	/** Queues the computation. */
	async function compute() {
		try {
			await call(api.POST('/api/coverages/{id}/compute', path));
			live = { state: 'queued', since: new Date().toISOString() };
			await reload();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Saves the map view; a failure only costs the view. */
	const save = (v: CoverageView) => api.PUT('/api/coverages/{id}/view', { ...path, body: v });
</script>

{#if c.isPending}
	<DetailSkeleton />
{:else if c.data && job}
	{@const d = c.data}
	<CoverageDetail {d} path="/coverages/{id}" {save}>
		{#snippet badge()}<JobBadge id="job-badge" state={job.state} />{/snippet}
		{#snippet actions()}
			<Button variant="outline" class="hidden sm:inline-flex" href="/coverages">Back</Button>
			<Button variant="outline" class="hidden sm:inline-flex" href="/coverages/{id}/edit">Edit</Button>
			<Button variant="outline" class="hidden sm:inline-flex" href="/coverages/new?from={id}">Duplicate</Button>
			<div class="sm:hidden">
				<RowActions
					actions={[
						{ label: 'Edit', icon: Pencil, href: `/coverages/${id}/edit` },
						{ label: 'Duplicate', icon: Copy, href: `/coverages/new?from=${id}` }
					]}
				/>
			</div>
			<ShareDialog
				what="coverage"
				hint={d.overlays.length ? ' It shows the map as you have it here, including the rasters on top.' : ''}
				share={d.share}
				create={() => api.PUT('/api/coverages/{id}/share', path)}
				revoke={() => api.DELETE('/api/coverages/{id}/share', path)}
				onchange={reload}
			/>
			{#if busy}
				<Button variant="outline" disabled>Computing&hellip;</Button>
			{:else}
				<Button onclick={compute}>{d.computed_at ? 'Recompute' : 'Compute'}</Button>
			{/if}
		{/snippet}
		{#snippet status()}
			{#if job.state === 'queued' && job.since}
				<Alert.Root class="mb-4">Queued since {clock(job.since)}, waiting for a worker.</Alert.Root>
			{:else if job.state === 'running' && job.since}
				<Alert.Root class="mb-4">Computing since {clock(job.since)}.</Alert.Root>
			{:else if d.job.state === 'failed'}
				<Alert.Root variant="destructive" class="mb-4">{d.job.error}</Alert.Root>
			{/if}
		{/snippet}
		{#snippet legend()}<LegendEditor {d} onchange={reload} />{/snippet}
	</CoverageDetail>
{:else}
	<FormErrors error={c.error} />
{/if}
