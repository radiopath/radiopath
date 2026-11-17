<script lang="ts">
	import Copy from '@lucide/svelte/icons/copy';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Plus from '@lucide/svelte/icons/plus';
	import Radar from '@lucide/svelte/icons/radar';
	import Trash from '@lucide/svelte/icons/trash';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, type Coverage } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Table from '#lib/components/ui/table/index.ts';
	import { ask } from '#lib/confirm.svelte.ts';
	import EmptyState from '#lib/EmptyState.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import ListSkeleton from '#lib/ListSkeleton.svelte';
	import { notifyError } from '#lib/notify.ts';
	import JobBadge from '#lib/JobBadge.svelte';
	import PageHead from '#lib/PageHead.svelte';
	import RowActions from '#lib/RowActions.svelte';

	const queryClient = useQueryClient();
	const coverages = createQuery(() => ({ queryKey: ['coverages'], queryFn: () => call(api.GET('/api/coverages')) }));
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));
	const nSites = $derived(sites.data?.length ?? 0);

	/** Deletes a coverage after confirmation. */
	async function remove(c: Coverage) {
		if (!(await ask({ title: `Delete coverage ${c.name}?`, action: 'Delete', destructive: true }))) return;
		try {
			await call(api.DELETE('/api/coverages/{id}', { params: { path: { id: c.id } } }));
			toast.success(`Coverage ${c.name} deleted`);
			await queryClient.invalidateQueries({ queryKey: ['coverages'] });
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<PageHead title="Coverage">
	{#if nSites}
		<Button href="/coverages/new"><Plus />New coverage</Button>
	{:else}
		<Button disabled title="A coverage needs a site"><Plus />New coverage</Button>
	{/if}
</PageHead>
<FormErrors error={coverages.error} />
{#if coverages.isPending || sites.isPending}
	<ListSkeleton />
{:else if coverages.data && sites.data}
	{@const list = coverages.data}
	{#if list.length}
		<div class="rounded-lg border bg-card">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Name</Table.Head>
						<Table.Head class="hidden sm:table-cell">Site (TX)</Table.Head>
						<Table.Head class="hidden text-right md:table-cell">Frequency</Table.Head>
						<Table.Head class="hidden text-right md:table-cell">Range</Table.Head>
						<Table.Head>Computed</Table.Head>
						<Table.Head class="w-12"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each list as c (c.id)}
						<Table.Row>
							<Table.Cell>
								<a class="block py-1 text-primary" href="/coverages/{c.id}">{c.name}</a>
								<span class="text-xs text-muted-foreground sm:hidden">{c.site.name}, {c.range_km.toFixed(1)} km</span>
							</Table.Cell>
							<Table.Cell class="hidden sm:table-cell">{c.site.name}</Table.Cell>
							<Table.Cell class="hidden text-right font-mono tabular-nums md:table-cell">{c.frequency_mhz.toFixed(3)} MHz</Table.Cell>
							<Table.Cell class="hidden text-right font-mono tabular-nums md:table-cell">{c.range_km.toFixed(1)} km</Table.Cell>
							<Table.Cell>
								{#if c.job.state === 'queued' || c.job.state === 'running' || c.job.state === 'failed'}
									<JobBadge state={c.job.state} />
								{:else if c.computed_at}
									<span class="font-mono text-xs">{new Date(c.computed_at).toLocaleString('sv-SE').slice(0, 16)}</span>
								{:else}
									<span class="text-muted-foreground">not yet</span>
								{/if}
							</Table.Cell>
							<Table.Cell class="text-right">
								<RowActions
									actions={[
										{ label: 'Edit', icon: Pencil, href: `/coverages/${c.id}/edit` },
										{ label: 'Duplicate', icon: Copy, href: `/coverages/new?from=${c.id}` },
										{ label: 'Delete', icon: Trash, run: () => remove(c), destructive: true }
									]}
								/>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
		<p class="table-note mt-2 text-xs text-muted-foreground">{list.length} coverage{list.length === 1 ? '' : 's'}</p>
	{:else}
		<EmptyState icon={Radar}>
			{#if nSites}
				No coverages yet. <a class="text-primary" href="/coverages/new">Add the first one</a>.
			{:else}
				A coverage starts at a site. <a class="text-primary" href="/sites/new">Create one</a> first.
			{/if}
		</EmptyState>
	{/if}
{/if}
