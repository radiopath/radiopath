<script lang="ts">
	import Info from '@lucide/svelte/icons/info';

	// An info button for the top right corner of a card (which must be `relative`); it opens a short explanation.
	let { label, text }: { label: string; text: string } = $props();
	let open = $state(false);
	let root: HTMLSpanElement;
</script>

<svelte:window
	onclick={(e) => {
		if (open && !root.contains(e.target as Node)) open = false;
	}}
	onkeydown={(e) => {
		if (e.key === 'Escape') open = false;
	}}
/>

<span class="absolute top-2 right-2 z-10" bind:this={root}>
	<button
		type="button"
		class="rounded-full p-0.5 text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
		aria-label={label}
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<Info class="size-4" />
	</button>
	{#if open}
		<div role="note" class="absolute top-7 right-0 w-72 max-w-[80vw] rounded-md border bg-popover p-3 text-xs text-popover-foreground shadow-md">
			{text}
		</div>
	{/if}
</span>
