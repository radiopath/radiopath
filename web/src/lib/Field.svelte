<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Label } from '#lib/components/ui/label/index.ts';

	// wide puts the label left of the field from sm up, for the long forms; otherwise it sits above.
	let { label, wide = false, children }: { label: string; wide?: boolean; children: Snippet } = $props();
</script>

<Label class={['grid gap-1.5 font-normal', wide && 'sm:grid-cols-[17rem_minmax(0,1fr)] sm:items-center sm:gap-x-4']}
	><span class="text-muted-foreground">{label}</span>{@render children()}</Label
>
