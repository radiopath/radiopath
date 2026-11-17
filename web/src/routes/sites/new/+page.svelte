<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import FormErrors from '#lib/FormErrors.svelte';
	import PageHead from '#lib/PageHead.svelte';
	import SiteForm from '#lib/SiteForm.svelte';

	// ?from=<id> duplicates a site.
	const from = Number(page.url.searchParams.get('from'));
	const orig = createQuery(() => ({
		queryKey: ['sites', from],
		queryFn: () => call(api.GET('/api/sites/{id}', { params: { path: { id: from } } })),
		enabled: from > 0
	}));
</script>

<PageHead title="New site"><Button variant="outline" href="/sites">Back to sites</Button></PageHead>
{#if !from}
	<SiteForm initial={{ name: '', lat: '', lon: '', antenna_height_m: '10' }} />
{:else if orig.data}
	{@const o = orig.data}
	<SiteForm initial={{ name: `${o.name} (copy)`, lat: String(o.lat), lon: String(o.lon), antenna_height_m: String(o.antenna_height_m) }} />
{:else}
	<FormErrors error={orig.error} />
{/if}
