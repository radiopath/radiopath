<script lang="ts" module>
	import type { Component } from 'svelte';

	export type RowAction = { label: string; icon?: Component; href?: string; run?: () => void; destructive?: boolean };
</script>

<script lang="ts">
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import { goto } from '$app/navigation';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as DropdownMenu from '#lib/components/ui/dropdown-menu/index.ts';

	// The actions of one row; destructive ones come last, after a separator.
	let { actions }: { actions: RowAction[] } = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button variant="ghost" size="icon" aria-label="Actions" {...props}><Ellipsis /></Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="w-44">
		{#each actions as a (a.label)}
			{#if a.destructive}<DropdownMenu.Separator />{/if}
			<DropdownMenu.Item variant={a.destructive ? 'destructive' : 'default'} onSelect={() => (a.href ? goto(a.href) : a.run?.())}>
				{#if a.icon}<a.icon />{/if}{a.label}
			</DropdownMenu.Item>
		{/each}
	</DropdownMenu.Content>
</DropdownMenu.Root>
