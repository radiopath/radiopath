<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as Alert from '#lib/components/ui/alert/index.ts';

	let { children }: { children: Snippet } = $props();
</script>

<Alert.Root class="mb-4 border-primary/40">{@render children()}</Alert.Root>
