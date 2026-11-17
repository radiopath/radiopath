<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import CoverageForm, { coverageValues } from '#lib/CoverageForm.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import PageHead from '#lib/PageHead.svelte';

	const id = Number(page.params.id);
	const c = createQuery(() => ({ queryKey: ['coverages', id], queryFn: () => call(api.GET('/api/coverages/{id}', { params: { path: { id } } })) }));
</script>

<PageHead title="Edit coverage"><Button variant="outline" href="/coverages">Back</Button></PageHead>
{#if c.data}
	<CoverageForm {id} initial={coverageValues(c.data)} />
{:else}
	<FormErrors error={c.error} />
{/if}
