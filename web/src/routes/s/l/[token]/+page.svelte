<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import DetailSkeleton from '#lib/DetailSkeleton.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import LinkDetail from '#lib/LinkDetail.svelte';

	const token = page.params.token ?? '';
	const l = createQuery(() => ({
		queryKey: ['share', 'l', token],
		queryFn: () => call(api.GET('/api/share/links/{token}', { params: { path: { token } } }))
	}));
</script>

{#if l.isPending}
	<DetailSkeleton />
{:else if l.data}
	<LinkDetail d={l.data} path="/s/l/{token}" shared />
{:else if l.error}
	<h1 class="mb-4 text-lg font-semibold">Not found</h1>
	<FormErrors error={l.error} />
{/if}
