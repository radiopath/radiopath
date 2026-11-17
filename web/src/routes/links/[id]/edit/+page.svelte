<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import FormErrors from '#lib/FormErrors.svelte';
	import LinkForm, { linkValues } from '#lib/LinkForm.svelte';
	import PageHead from '#lib/PageHead.svelte';

	const id = Number(page.params.id);
	const l = createQuery(() => ({ queryKey: ['links', id], queryFn: () => call(api.GET('/api/links/{id}', { params: { path: { id } } })) }));
</script>

<PageHead title="Edit link"><Button variant="outline" href="/links">Back</Button></PageHead>
{#if l.data}
	<LinkForm {id} initial={linkValues(l.data)} />
{:else}
	<FormErrors error={l.error} />
{/if}
