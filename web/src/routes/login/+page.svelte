<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError, api, call, configQuery, meQuery, safeNext } from '#lib/api.ts';
	import AuthCard from '#lib/AuthCard.svelte';
	import * as Alert from '#lib/components/ui/alert/index.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import Notice from '#lib/Notice.svelte';
	import { notifyError } from '#lib/notify.ts';

	const notices: Record<string, string> = {
		registered: 'Almost done: check your mail for the next step.',
		verified: 'Address confirmed.',
		reset: 'Password changed. Log in with the new one.'
	};
	const config = createQuery(configQuery);
	const me = createQuery(meQuery);
	const next = safeNext(page.url.searchParams.get('next'));
	let name = $state('');
	let password = $state('');
	// Why the account cannot log in yet, shown next to the resend button.
	let unverified = $state('');
	let busy = $state(false);

	// The page a mail link or the registration led here from says what happened.
	onMount(() => {
		const msg = notices[page.url.searchParams.get('msg') ?? ''];
		if (msg) toast.success(msg);
	});

	// Logged in already: nothing to do here.
	$effect(() => {
		if (me.data) location.replace(next);
	});

	/** Logs in and loads the page the user came from; an unconfirmed account gets the resend button instead. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			await call(api.POST('/api/session', { body: { name, password } }));
			location.assign(next);
		} catch (err) {
			if (err instanceof ApiError && err.status === 403) unverified = err.message;
			else notifyError(err);
			busy = false;
		}
	}

	/** Asks for a new confirmation mail for the name just entered. */
	async function resend() {
		try {
			await call(api.POST('/api/verify/resend', { body: { name } }));
			unverified = '';
			toast.success('If that account is waiting for confirmation, a new link is on its way.');
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<svelte:head>
	<meta
		name="description"
		content="Radiopath plans radio links and area coverage: terrain profiles from Copernicus GLO-30, Longley-Rice/ITM path loss, Fresnel zone, link budget with real antenna patterns, and PDF reports."
	/>
	<link rel="canonical" href="/login" />
</svelte:head>

<AuthCard title="Log in" noindex={false}>
	{#if config.data?.maintenance}
		<Alert.Root class="border-warn/50 text-warn">
			<b>Maintenance</b><br />Radiopath is closed for maintenance. Logins are disabled, please try again later.
		</Alert.Root>
	{:else if unverified}
		<Notice>{unverified}</Notice>
		<div class="flex flex-wrap items-center gap-4">
			<Button onclick={resend}>Resend confirmation mail</Button>
			<button class="text-primary" onclick={() => (unverified = '')}>Back to login</button>
		</div>
	{:else}
		<form class="grid gap-4" onsubmit={submit}>
			<Field label="User name or email">
				<!-- svelte-ignore a11y_autofocus -->
				<Input name="name" autocomplete="username" autofocus required bind:value={name} />
			</Field>
			<Field label="Password"><Input name="password" type="password" autocomplete="current-password" required bind:value={password} /></Field>
			{#if config.data?.password_reset}<a class="-mt-2 justify-self-end text-xs text-primary" href="/forgot">Forgot password?</a>{/if}
			<Button type="submit" class="w-full" disabled={busy}>Log in</Button>
		</form>
	{/if}
	{#snippet below()}
		{#if config.data?.registration}
			<span>New here? <a class="text-primary" href="/register">Create an account</a></span>
		{:else if config.data}
			<span>Registration is closed at the moment.</span>
		{/if}
		<span class="basis-full">In active development: results may change between versions.</span>
	{/snippet}
</AuthCard>
