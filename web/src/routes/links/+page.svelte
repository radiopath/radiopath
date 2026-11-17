<script lang="ts">
	import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right';
	import Copy from '@lucide/svelte/icons/copy';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Plus from '@lucide/svelte/icons/plus';
	import Trash from '@lucide/svelte/icons/trash';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, type Link } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Table from '#lib/components/ui/table/index.ts';
	import { ask } from '#lib/confirm.svelte.ts';
	import EmptyState from '#lib/EmptyState.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import ListSkeleton from '#lib/ListSkeleton.svelte';
	import { notifyError } from '#lib/notify.ts';
	import PageHead from '#lib/PageHead.svelte';
	import RowActions from '#lib/RowActions.svelte';

	const queryClient = useQueryClient();
	const links = createQuery(() => ({ queryKey: ['links'], queryFn: () => call(api.GET('/api/links')) }));
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));
	const nSites = $derived(sites.data?.length ?? 0);

	/** Deletes a link after confirmation. */
	async function remove(l: Link) {
		if (!(await ask({ title: `Delete link ${l.name}?`, action: 'Delete', destructive: true }))) return;
		try {
			await call(api.DELETE('/api/links/{id}', { params: { path: { id: l.id } } }));
			toast.success(`Link ${l.name} deleted`);
			await queryClient.invalidateQueries({ queryKey: ['links'] });
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<PageHead title="Links">
	{#if nSites >= 2}
		<Button href="/links/new"><Plus />New link</Button>
	{:else}
		<Button disabled title="A link needs two sites"><Plus />New link</Button>
	{/if}
</PageHead>
<FormErrors error={links.error} />
{#if links.isPending || sites.isPending}
	<ListSkeleton />
{:else if links.data && sites.data}
	{@const list = links.data}
	{#if list.length}
		<div class="rounded-lg border bg-card">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Name</Table.Head>
						<Table.Head class="hidden sm:table-cell">Site A (TX)</Table.Head>
						<Table.Head class="hidden sm:table-cell">Site B (RX)</Table.Head>
						<Table.Head class="hidden text-right sm:table-cell">Frequency</Table.Head>
						<Table.Head class="w-12"></Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each list as l (l.id)}
						<Table.Row>
							<Table.Cell>
								<a class="block py-1 text-primary" href="/links/{l.id}">{l.name}</a>
								<span class="text-xs text-muted-foreground sm:hidden">{l.site_a.name} → {l.site_b.name}, {l.frequency_mhz.toFixed(3)} MHz</span>
							</Table.Cell>
							<Table.Cell class="hidden sm:table-cell">{l.site_a.name}</Table.Cell>
							<Table.Cell class="hidden sm:table-cell">{l.site_b.name}</Table.Cell>
							<Table.Cell class="hidden text-right font-mono tabular-nums sm:table-cell">{l.frequency_mhz.toFixed(3)} MHz</Table.Cell>
							<Table.Cell class="text-right">
								<RowActions
									actions={[
										{ label: 'Edit', icon: Pencil, href: `/links/${l.id}/edit` },
										{ label: 'Duplicate', icon: Copy, href: `/links/new?from=${l.id}` },
										{ label: 'Delete', icon: Trash, run: () => remove(l), destructive: true }
									]}
								/>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
		<p class="table-note mt-2 text-xs text-muted-foreground">{list.length} link{list.length === 1 ? '' : 's'}</p>
	{:else}
		<EmptyState icon={ArrowLeftRight}>
			{#if nSites >= 2}
				No links yet. <a class="text-primary" href="/links/new">Add the first one</a>.
			{:else if nSites}
				A link needs two sites. <a class="text-primary" href="/sites/new">Add the second one</a> first.
			{:else}
				A link connects two sites. <a class="text-primary" href="/sites/new">Create your sites</a> first.
			{/if}
		</EmptyState>
	{/if}
{/if}
