<script lang="ts">
	import Copy from '@lucide/svelte/icons/copy';
	import MapPin from '@lucide/svelte/icons/map-pin';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Plus from '@lucide/svelte/icons/plus';
	import Trash from '@lucide/svelte/icons/trash';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, type Site } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Table from '#lib/components/ui/table/index.ts';
	import { ask } from '#lib/confirm.svelte.ts';
	import EmptyState from '#lib/EmptyState.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import ListSkeleton from '#lib/ListSkeleton.svelte';
	import { notifyError } from '#lib/notify.ts';
	import PageHead from '#lib/PageHead.svelte';
	import RowActions from '#lib/RowActions.svelte';
	import SitesMap from '#lib/SitesMap.svelte';

	const queryClient = useQueryClient();
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));

	/** Deletes a site after confirmation; a site still used by a link or coverage stays and the reason is shown. */
	async function remove(s: Site) {
		if (!(await ask({ title: `Delete site ${s.name}?`, action: 'Delete', destructive: true }))) return;
		try {
			await call(api.DELETE('/api/sites/{id}', { params: { path: { id: s.id } } }));
			toast.success(`Site ${s.name} deleted`);
			await queryClient.invalidateQueries({ queryKey: ['sites'] });
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<PageHead title="Sites">
	<Button href="/sites/new"><Plus />New site</Button>
</PageHead>
<FormErrors error={sites.error} />
{#if sites.isPending}
	<ListSkeleton />
{:else if sites.data}
	{@const list = sites.data}
	<div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
		<div>
			{#if list.length}
				<div class="rounded-lg border bg-card">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Name</Table.Head>
								<Table.Head class="hidden text-right sm:table-cell">Latitude</Table.Head>
								<Table.Head class="hidden text-right sm:table-cell">Longitude</Table.Head>
								<Table.Head class="text-right">Antenna</Table.Head>
								<Table.Head class="w-12"></Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each list as s (s.id)}
								<Table.Row>
									<Table.Cell>
										<a class="block py-1 text-primary" href="/sites/{s.id}">{s.name}</a>
										<span class="font-mono text-xs text-muted-foreground sm:hidden">{s.lat.toFixed(5)}, {s.lon.toFixed(5)}</span>
									</Table.Cell>
									<Table.Cell class="hidden text-right font-mono tabular-nums sm:table-cell">{s.lat.toFixed(5)}</Table.Cell>
									<Table.Cell class="hidden text-right font-mono tabular-nums sm:table-cell">{s.lon.toFixed(5)}</Table.Cell>
									<Table.Cell class="text-right font-mono tabular-nums">{s.antenna_height_m.toFixed(1)} m</Table.Cell>
									<Table.Cell class="text-right">
										<RowActions
											actions={[
												{ label: 'Edit', icon: Pencil, href: `/sites/${s.id}` },
												{ label: 'Duplicate', icon: Copy, href: `/sites/new?from=${s.id}` },
												{ label: 'Delete', icon: Trash, run: () => remove(s), destructive: true }
											]}
										/>
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
				<p class="table-note mt-2 text-xs text-muted-foreground">{list.length} site{list.length === 1 ? '' : 's'}</p>
			{:else}
				<EmptyState icon={MapPin}>No sites yet. <a class="text-primary" href="/sites/new">Add the first one</a>.</EmptyState>
			{/if}
		</div>
		<SitesMap sites={list} />
	</div>
{/if}
