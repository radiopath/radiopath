<script lang="ts">
	import Moon from '@lucide/svelte/icons/moon';
	import Sun from '@lucide/svelte/icons/sun';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { theme } from '#lib/theme.svelte.ts';
</script>

<Button variant="ghost" size="icon-sm" title="Light / dark" aria-label="Light / dark" aria-pressed={theme.dark} onclick={() => theme.toggle()}>
	{#if theme.dark}<Sun />{:else}<Moon />{/if}
</Button>
