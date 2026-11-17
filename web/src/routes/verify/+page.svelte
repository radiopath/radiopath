<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api, call } from '#lib/api.ts';
	import AuthCard from '#lib/AuthCard.svelte';
	import FormErrors from '#lib/FormErrors.svelte';

	// The mail link carries the token in the fragment, which never reaches a server or its logs.
	const token = location.hash.slice(1);
	let error: unknown = $state(null);

	// Confirmed on mount, not by the GET of the link, so mail scanners that prefetch it do not use up the token.
	onMount(async () => {
		// drop the token from the address bar and history; SvelteKit's replaceState throws before its router has started
		history.replaceState(history.state, '', location.pathname);
		try {
			await call(api.POST('/api/verify', { body: { token } }));
			const { response } = await api.GET('/api/me');
			if (response.ok) await goto('/account?msg=verified');
			else await goto('/login?msg=verified');
		} catch (err) {
			error = err;
		}
	});
</script>

<AuthCard title="Confirm address">
	{#if error}
		<FormErrors {error} />
	{:else}
		<p class="text-muted-foreground">Confirming …</p>
	{/if}
	{#snippet below()}
		<a class="text-primary" href="/login">Back to login</a>
	{/snippet}
</AuthCard>
