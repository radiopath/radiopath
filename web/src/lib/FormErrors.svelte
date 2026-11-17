<script lang="ts">
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { ApiError } from '#lib/api.ts';

	let { error }: { error: unknown } = $props();
	const messages = $derived(error instanceof ApiError && error.errors.length ? error.errors : error ? [(error as Error).message] : []);
</script>

{#if messages.length}
	<Alert.Root variant="destructive" class="mb-4">
		{#if messages.length === 1}{messages[0]}{:else}<ul class="list-disc pl-4">
				{#each messages as m (m)}<li>{m}</li>{/each}
			</ul>{/if}
	</Alert.Root>
{/if}
