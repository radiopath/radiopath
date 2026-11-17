<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import FormErrors from '#lib/FormErrors.svelte';
	import LinkForm, { linkValues } from '#lib/LinkForm.svelte';
	import PageHead from '#lib/PageHead.svelte';

	// ?from=<id> duplicates a link.
	const from = Number(page.url.searchParams.get('from'));
	const orig = createQuery(() => ({
		queryKey: ['links', from],
		queryFn: () => call(api.GET('/api/links/{id}', { params: { path: { id: from } } })),
		enabled: from > 0
	}));
</script>

<PageHead title="New link"><Button variant="outline" href="/links">Back</Button></PageHead>
{#if !from}
	<LinkForm initial={linkValues()} />
{:else if orig.data}
	<LinkForm initial={{ ...linkValues(orig.data), name: `${orig.data.name} (copy)` }} />
{:else}
	<FormErrors error={orig.error} />
{/if}
