<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import FormErrors from '#lib/FormErrors.svelte';
	import PageHead from '#lib/PageHead.svelte';
	import SiteForm from '#lib/SiteForm.svelte';

	const id = Number(page.params.id);
	const site = createQuery(() => ({ queryKey: ['sites', id], queryFn: () => call(api.GET('/api/sites/{id}', { params: { path: { id } } })) }));
</script>

<PageHead title="Edit site"><Button variant="outline" href="/sites">Back to sites</Button></PageHead>
{#if site.data}
	{@const s = site.data}
	<SiteForm {id} initial={{ name: s.name, lat: String(s.lat), lon: String(s.lon), antenna_height_m: String(s.antenna_height_m) }} />
{:else}
	<FormErrors error={site.error} />
{/if}
