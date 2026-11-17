<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { api, call } from '#lib/api.ts';
	import AuthCard from '#lib/AuthCard.svelte';
	import { Button } from '#lib/components/ui/button/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';
	import { notifyError } from '#lib/notify.ts';

	let email = $state('');
	let busy = $state(false);

	/** Requests the reset link; the answer is the same whether the address has an account or not. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			await call(api.POST('/api/password/forgot', { body: { email } }));
			toast.success('If that address belongs to an account, a reset link is on its way.');
		} catch (err) {
			notifyError(err);
		}
		busy = false;
	}
</script>

<AuthCard title="Forgot password">
	<form class="grid gap-4" onsubmit={submit}>
		<Field label="Email">
			<!-- svelte-ignore a11y_autofocus -->
			<Input name="email" type="email" autocomplete="email" autofocus required bind:value={email} />
		</Field>
		<Button type="submit" class="w-full" disabled={busy}>Send reset link</Button>
	</form>
	<p class="text-xs text-muted-foreground">We mail a link to the address of your account. It is valid for one hour.</p>
	{#snippet below()}
		<a class="text-primary" href="/login">Back to login</a>
	{/snippet}
</AuthCard>
