<script lang="ts">
	import { goto } from '$app/navigation';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, call, configQuery } from '#lib/api.ts';
	import AuthCard from '#lib/AuthCard.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';

	const config = createQuery(configQuery);
	let name = $state('');
	let email = $state('');
	let password = $state('');
	let password2 = $state('');
	let busy = $state(false);

	/** Creates the account; the confirmation mail is on its way when the login page shows. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (password !== password2) return notifyError(invalid('Passwords do not match'));
		busy = true;
		try {
			await call(api.POST('/api/register', { body: { name, email, password } }));
			await goto('/login?msg=registered');
		} catch (err) {
			notifyError(err);
			busy = false;
		}
	}
</script>

<AuthCard title="Register">
	{#if config.data && !config.data.registration}
		<p>Registration is closed at the moment.</p>
	{:else}
		<form class="grid gap-4" onsubmit={submit}>
			<Field label="User name">
				<!-- svelte-ignore a11y_autofocus -->
				<Input name="name" autocomplete="username" autofocus required bind:value={name} />
			</Field>
			<Field label="Email"><Input name="email" type="email" autocomplete="email" required bind:value={email} /></Field>
			<Field label="Password"
				><Input name="password" type="password" autocomplete="new-password" minlength={10} required bind:value={password} /></Field
			>
			<Field label="Repeat password">
				<Input name="password2" type="password" autocomplete="new-password" minlength={10} required bind:value={password2} />
			</Field>
			<Button type="submit" class="w-full" disabled={busy}>Create account</Button>
		</form>
		<p class="text-xs text-muted-foreground">
			3 to 32 characters for the name (letters, digits, . _ -), at least 10 for the password. We mail a confirmation link to the address; the account
			opens once you follow it.
		</p>
	{/if}
	{#snippet below()}
		<span>Have an account? <a class="text-primary" href="/login">Log in</a></span>
	{/snippet}
</AuthCard>
