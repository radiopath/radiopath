<script lang="ts">
	import Copy from '@lucide/svelte/icons/copy';
	import Pencil from '@lucide/svelte/icons/pencil';
	import { page } from '$app/state';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import DetailSkeleton from '#lib/DetailSkeleton.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import LinkDetail from '#lib/LinkDetail.svelte';
	import RowActions from '#lib/RowActions.svelte';
	import ShareDialog from '#lib/ShareDialog.svelte';

	const id = Number(page.params.id);
	const path = { params: { path: { id } } };
	const queryClient = useQueryClient();
	const l = createQuery(() => ({ queryKey: ['links', id], queryFn: () => call(api.GET('/api/links/{id}', path)) }));
</script>

{#if l.isPending}
	<DetailSkeleton />
{:else if l.data}
	<LinkDetail d={l.data} path="/links/{id}">
		{#snippet actions()}
			<Button variant="outline" class="hidden sm:inline-flex" href="/links">Back</Button>
			<Button variant="outline" class="hidden sm:inline-flex" href="/links/{id}/edit">Edit</Button>
			<Button variant="outline" class="hidden sm:inline-flex" href="/links/new?from={id}">Duplicate</Button>
			<div class="sm:hidden">
				<RowActions
					actions={[
						{ label: 'Edit', icon: Pencil, href: `/links/${id}/edit` },
						{ label: 'Duplicate', icon: Copy, href: `/links/new?from=${id}` }
					]}
				/>
			</div>
			<ShareDialog
				what="link"
				share={l.data.share}
				create={() => api.PUT('/api/links/{id}/share', path)}
				revoke={() => api.DELETE('/api/links/{id}/share', path)}
				onchange={() => queryClient.invalidateQueries({ queryKey: ['links'] })}
			/>
		{/snippet}
	</LinkDetail>
{:else}
	<FormErrors error={l.error} />
{/if}
