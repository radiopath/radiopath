<script lang="ts">
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { ask } from '#lib/confirm.svelte.ts';
	import Field from '#lib/Field.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';
	import SettingsSection from '#lib/SettingsSection.svelte';

	const queryClient = useQueryClient();
	// The page needs a login: call() sends a guest there on 401.
	const me = createQuery(() => ({ queryKey: ['me', 'account'], queryFn: () => call(api.GET('/api/me')) }));
	let current = $state('');
	let password = $state('');
	let password2 = $state('');

	/** Changes the password; every other session ends, this one gets a fresh cookie. */
	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		if (password !== password2) return notifyError(invalid('Passwords do not match'));
		try {
			await call(api.PUT('/api/me/password', { body: { current, password } }));
			current = password = password2 = '';
			toast.success('Password changed. Other devices have been logged out.');
			await queryClient.invalidateQueries({ queryKey: ['me'] });
		} catch (err) {
			notifyError(err);
		}
	}

	/** Ends every session of the account except this one, after confirmation. */
	async function logoutOthers() {
		const ok = await ask({
			title: 'Log out other devices?',
			description: 'Every other browser logged in to this account has to log in again. API tokens keep working.',
			action: 'Log out'
		});
		if (!ok) return;
		try {
			const { count } = await call(api.DELETE('/api/me/sessions'));
			toast.success(count ? `${count} other session${count === 1 ? '' : 's'} logged out` : 'No other sessions');
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<svelte:head><title>Security – Radiopath</title></svelte:head>
<FormErrors error={me.error} />
{#if me.data}
	<SettingsSection title="Password">
		{#snippet description()}At least 10 characters. Changing it logs out every other device; API tokens keep working.{/snippet}
		<form class="grid gap-3" aria-label="Change password" onsubmit={changePassword}>
			<Field label="Current password"><Input name="current" type="password" autocomplete="current-password" required bind:value={current} /></Field>
			<Field label="New password">
				<Input name="password" type="password" autocomplete="new-password" minlength={10} required bind:value={password} />
			</Field>
			<Field label="Repeat new password">
				<Input name="password2" type="password" autocomplete="new-password" minlength={10} required bind:value={password2} />
			</Field>
			<div><Button type="submit">Change password</Button></div>
		</form>
	</SettingsSection>
	<SettingsSection title="Sessions">
		{#snippet description()}A login lasts seven days after its last use, at most thirty. Log out the others if you used a shared computer or lost a
			device.{/snippet}
		<div class="flex flex-wrap items-center justify-between gap-3">
			<p class="text-sm text-muted-foreground">This browser stays logged in.</p>
			<Button variant="outline" onclick={logoutOthers}>Log out other devices</Button>
		</div>
	</SettingsSection>
{/if}
