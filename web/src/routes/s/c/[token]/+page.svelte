<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import CoverageDetail from '#lib/CoverageDetail.svelte';
	import DetailSkeleton from '#lib/DetailSkeleton.svelte';
	import FormErrors from '#lib/FormErrors.svelte';

	const token = page.params.token ?? '';
	const c = createQuery(() => ({
		queryKey: ['share', 'c', token],
		queryFn: () => call(api.GET('/api/share/coverages/{token}', { params: { path: { token } } }))
	}));
</script>

{#if c.isPending}
	<DetailSkeleton />
{:else if c.data}
	<CoverageDetail d={c.data} path="/s/c/{token}" shared />
{:else if c.error}
	<h1 class="mb-4 text-lg font-semibold">Not found</h1>
	<FormErrors error={c.error} />
{/if}
