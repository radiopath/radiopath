<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { onMount } from 'svelte';
	import { api, call } from '#lib/api.ts';
	import { theme } from '#lib/theme.svelte.ts';

	// The description needs a login; call() sends a guest there on 401.
	const me = createQuery(() => ({ queryKey: ['me', 'api-reference'], queryFn: () => call(api.GET('/api/me')) }));
	let el: HTMLDivElement;

	// Scalar in the colours and fonts of the app; the variables follow html[data-theme] by themselves.
	const customCss = `.scalar-app, .light-mode, .dark-mode {
		--scalar-font: 'IBM Plex Sans', ui-sans-serif, system-ui, sans-serif;
		--scalar-font-code: 'JetBrains Mono', ui-monospace, monospace;
		--scalar-background-1: var(--cp-surface);
		--scalar-background-2: var(--cp-bg);
		--scalar-background-3: var(--cp-line-soft);
		--scalar-color-1: var(--cp-fg);
		--scalar-color-2: var(--cp-muted);
		--scalar-color-3: var(--cp-muted);
		--scalar-color-accent: var(--cp-accent);
		--scalar-border-color: var(--cp-line);
		--scalar-custom-header-height: 48px;
	}`;

	// What the reference shows: nothing that calls Scalar's services (fonts, telemetry, AI agent, MCP, proxy).
	const config = (dark: boolean) => ({
		url: '/api/openapi.json',
		servers: [{ url: location.origin }],
		customCss,
		withDefaultFonts: false,
		hideDarkModeToggle: true,
		forceDarkModeState: dark ? ('dark' as const) : ('light' as const),
		documentDownloadType: 'json' as const,
		showDeveloperTools: 'never' as const,
		telemetry: false,
		agent: { disabled: true },
		mcp: { disabled: true },
		hideClientButton: true,
		authentication: { preferredSecurityScheme: 'bearer' }
	});

	let create: typeof import('@scalar/api-reference').createApiReference | undefined = $state();

	/** Loads Scalar only on this page, so the rest of the app does not carry it. */
	onMount(() => {
		let gone = false;
		Promise.all([import('@scalar/api-reference'), import('@scalar/api-reference/style.css')]).then(([m]) => {
			if (!gone) create = m.createApiReference;
		});
		return () => {
			gone = true;
		};
	});

	// Builds the reference and rebuilds it on a theme switch: Scalar reads its colour mode only when it mounts.
	$effect(() => {
		if (!create) return;
		const r = create(el, config(theme.dark));
		return () => r.destroy();
	});
</script>

<svelte:head><title>API reference – Radiopath</title></svelte:head>
{#if me.data}
	<p class="mb-4 max-w-3xl text-sm text-muted-foreground">
		Send an <a class="text-primary hover:underline" href="/account/tokens">API token</a> as
		<span class="font-mono">Authorization: Bearer rp_…</span> to {location.origin}/api. Each operation lists the scope a token needs; "Test request"
		runs it in your browser with your login or a token you enter.
	</p>
{/if}
<div class="api-reference -mx-4 border-y bg-card sm:mx-0 sm:border" bind:this={el}></div>
