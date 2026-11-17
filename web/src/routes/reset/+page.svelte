<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { ApiError, api, call } from '#lib/api.ts';
	import AuthCard from '#lib/AuthCard.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';

	// The mail link carries the token in the fragment, which never reaches a server or its logs.
	const token = location.hash.slice(1);
	let password = $state('');
	let password2 = $state('');
	// An expired or used link: the page has nothing left to offer but a new one.
	let stale: unknown = $state(null);
	let busy = $state(false);

	/** Shows an expired or used link on the page, any other error as a toast. */
	function check(err: unknown) {
		if (err instanceof ApiError && err.status === 410) stale = err;
		else notifyError(err);
	}

	onMount(() => {
		// drop the token from the address bar and history; SvelteKit's replaceState throws before its router has started
		history.replaceState(history.state, '', location.pathname);
		call(api.POST('/api/password/reset/check', { body: { token } })).catch(check);
	});

	/** Sets the new password, which logs out every session of the account. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (password !== password2) return notifyError(invalid('Passwords do not match'));
		busy = true;
		try {
			await call(api.POST('/api/password/reset', { body: { token, password } }));
			await goto('/login?msg=reset');
		} catch (err) {
			check(err);
			busy = false;
		}
	}
</script>

<AuthCard title="Choose a new password">
	{#if stale}
		<FormErrors error={stale} />
		<a class="text-xs text-primary" href="/forgot">Request a new link</a>
	{:else}
		<form class="grid gap-4" onsubmit={submit}>
			<Field label="New password">
				<!-- svelte-ignore a11y_autofocus -->
				<Input name="password" type="password" autocomplete="new-password" minlength={10} autofocus required bind:value={password} />
			</Field>
			<Field label="Repeat"
				><Input name="password2" type="password" autocomplete="new-password" minlength={10} required bind:value={password2} /></Field
			>
			<Button type="submit" class="w-full" disabled={busy}>Set password</Button>
		</form>
		<p class="text-xs text-muted-foreground">At least 10 characters. Every open session of the account is logged out.</p>
	{/if}
	{#snippet below()}
		<a class="text-primary" href="/login">Back to login</a>
	{/snippet}
</AuthCard>
