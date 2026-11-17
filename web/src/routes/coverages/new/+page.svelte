<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import CoverageForm, { coverageValues } from '#lib/CoverageForm.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import PageHead from '#lib/PageHead.svelte';

	// ?from=<id> duplicates a coverage with its legend but without its result.
	const from = Number(page.url.searchParams.get('from'));
	const orig = createQuery(() => ({
		queryKey: ['coverages', from],
		queryFn: () => call(api.GET('/api/coverages/{id}', { params: { path: { id: from } } })),
		enabled: from > 0
	}));
</script>

<PageHead title="New coverage"><Button variant="outline" href="/coverages">Back</Button></PageHead>
{#if !from}
	<CoverageForm initial={coverageValues()} />
{:else if orig.data}
	{@const o = orig.data}
	<CoverageForm initial={{ ...coverageValues(o), name: `${o.name} (copy)` }} legend={o.custom_legend ? o : undefined} />
{:else}
	<FormErrors error={orig.error} />
{/if}
