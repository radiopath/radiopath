<script lang="ts">
	import type { Snippet } from 'svelte';
	import { createQuery } from '@tanstack/svelte-query';
	import { meQuery } from '#lib/api.ts';
	import BottomNav from '#lib/BottomNav.svelte';
	import Footer from '#lib/Footer.svelte';
	import Header from '#lib/Header.svelte';

	let { children }: { children: Snippet } = $props();
	// Inside the query provider, so it can tell whether the tab bar is there and needs room.
	const me = createQuery(meQuery);
</script>

<div class={['flex min-h-screen flex-col', me.data && 'pb-[calc(3.5rem+env(safe-area-inset-bottom))] sm:pb-0']}>
	<Header />
	<main class="mx-auto w-full max-w-[1400px] px-4 py-4 sm:px-5 sm:py-5">
		{@render children()}
	</main>
	<Footer />
</div>
{#if me.data}<BottomNav />{/if}
