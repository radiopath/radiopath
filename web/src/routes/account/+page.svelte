<script lang="ts">
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { api, call } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Dialog from '#lib/components/ui/dialog/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import { notifyError } from '#lib/notify.ts';
	import SettingsSection from '#lib/SettingsSection.svelte';

	// The page needs a login: call() sends a guest there on 401.
	const me = createQuery(() => ({ queryKey: ['me', 'account'], queryFn: () => call(api.GET('/api/me')) }));
	let changing = $state(false);
	let email = $state('');
	let password = $state('');

	// The confirmation link of a new address leads here.
	onMount(() => {
		if (page.url.searchParams.get('msg') === 'verified') toast.success('Address confirmed.');
	});

	/** Opens the dialog with an empty form. */
	function open() {
		email = password = '';
		changing = true;
	}

	/** Asks for the new address to be confirmed by mail; the password guards against a stolen session. */
	async function changeEmail(e: SubmitEvent) {
		e.preventDefault();
		try {
			const r = await api.PUT('/api/me/email', { body: { email, password } });
			await call(Promise.resolve(r));
			changing = false;
			// 204: the address did not change
			if (r.response.status === 202) toast.success('Confirmation link sent to the new address. It takes effect once you open it.');
			else toast.info('That is the address on file already.');
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<svelte:head><title>Profile – Radiopath</title></svelte:head>
<FormErrors error={me.error} />
{#if me.data}
	<SettingsSection title="Profile">
		{#snippet description()}Your user name and the address for password resets and notices, such as a new API token.{/snippet}
		<dl class="divide-y text-sm">
			<div class="grid gap-1 pb-3">
				<dt class="text-muted-foreground">User name</dt>
				<dd class="font-medium">{me.data.name}</dd>
				<dd class="text-xs text-muted-foreground">The user name cannot be changed.</dd>
			</div>
			<div class="grid gap-1 pt-3">
				<dt class="text-muted-foreground">Email</dt>
				<dd class="flex flex-wrap items-center justify-between gap-2">
					<span class="flex min-w-0 items-center gap-2">
						{#if me.data.email}
							<span class="truncate font-medium" id="email">{me.data.email}</span>
							<span class="shrink-0 border border-ok px-1.5 py-px text-xs font-medium text-ok">Confirmed</span>
						{:else}
							<span class="shrink-0 border border-warn px-1.5 py-px text-xs font-medium text-warn">No address</span>
						{/if}
					</span>
					<Button variant="outline" size="sm" onclick={open}>{me.data.email ? 'Change' : 'Add address'}</Button>
				</dd>
				<dd class="text-xs text-muted-foreground">
					{#if me.data.email}
						Password reset mails go to this address.
					{:else}
						No address on file: without one there is no password reset.
					{/if}
				</dd>
			</div>
		</dl>
	</SettingsSection>

	<Dialog.Root bind:open={changing}>
		<Dialog.Content showCloseButton={false} class="sm:max-w-md">
			<Dialog.Header>
				<Dialog.Title>{me.data.email ? 'Change email address' : 'Add an email address'}</Dialog.Title>
				<Dialog.Description>
					We send a confirmation link to the new address; it takes effect once you open it.{#if me.data.email}
						The current address gets a notice.{/if}
				</Dialog.Description>
			</Dialog.Header>
			<form class="grid gap-3" aria-label="Change address" onsubmit={changeEmail}>
				<Field label="New address"><Input name="email" type="email" autocomplete="email" required bind:value={email} /></Field>
				<Field label="Your password">
					<Input name="email_password" type="password" autocomplete="current-password" required bind:value={password} />
				</Field>
				<p class="-mt-1 text-xs text-muted-foreground">Needed so that nobody with access to this browser can take over the account.</p>
				<Dialog.Footer>
					<Button variant="outline" onclick={() => (changing = false)}>Cancel</Button>
					<Button type="submit">Send confirmation link</Button>
				</Dialog.Footer>
			</form>
		</Dialog.Content>
	</Dialog.Root>
{/if}
