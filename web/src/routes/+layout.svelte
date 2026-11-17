<script lang="ts">
	import '../app.css';
	import { QueryClient, QueryClientProvider } from '@tanstack/svelte-query';
	import ConfirmHost from '#lib/ConfirmHost.svelte';
	import { Toaster } from '#lib/components/ui/sonner/index.ts';
	import Shell from '#lib/Shell.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
	// Link analysis runs on every GET, so no refetch on window focus.
	const queryClient = new QueryClient({ defaultOptions: { queries: { refetchOnWindowFocus: false, retry: false } } });
</script>

<svelte:head>
	<title>Radiopath</title>
</svelte:head>

<Toaster position="top-center" richColors closeButton />
<ConfirmHost />
<QueryClientProvider client={queryClient}>
	<Shell>
		{@render children()}
	</Shell>
</QueryClientProvider>
