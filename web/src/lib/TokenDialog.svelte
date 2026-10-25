<script lang="ts" module>
	type Level = 'none' | 'read' | 'write';

	// What a token can be given access to; account and hf have no write scope.
	export const resources = [
		{ key: 'sites', label: 'Sites', hint: 'Locations and antenna heights', write: true },
		{ key: 'antennas', label: 'Antennas', hint: 'Patterns and manual antennas', write: true },
		{ key: 'links', label: 'Links', hint: 'Links, path analyses and PDF reports', write: true },
		{ key: 'coverages', label: 'Coverages', hint: 'Coverages, jobs, rasters and PDF reports', write: true },
		{ key: 'hf', label: 'HF prediction', hint: 'HF circuit predictions, maps and sunspot numbers', write: false },
		{ key: 'account', label: 'Account', hint: 'User name and address', write: false }
	] as const;

	/** The scopes of a token, one per resource: "links:write" stands for read and write. */
	export const scopeSummary = (scopes: string[]) =>
		resources.flatMap((r) => (scopes.includes(`${r.key}:write`) ? [`${r.key}:write`] : scopes.includes(`${r.key}:read`) ? [`${r.key}:read`] : []));
</script>

<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import Copy from '@lucide/svelte/icons/copy';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import { toast } from 'svelte-sonner';
	import { api, call, type Schemas } from '$lib/api';
	import * as Alert from '$lib/components/ui/alert';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { NativeSelect } from '$lib/components/ui/native-select';
	import Field from '$lib/Field.svelte';
	import { notifyError } from '$lib/notify';

	// oncreated reloads the list; the secret stays in this dialog until it is closed.
	let { open = $bindable(false), oncreated }: { open?: boolean; oncreated: () => Promise<unknown> } = $props();

	/** Every resource without access, the start of a new token. */
	const none = () => Object.fromEntries(resources.map((r) => [r.key, 'none'])) as Record<string, Level>;
	let name = $state('');
	let expiry = $state('30');
	let customDay = $state('');
	let password = $state('');
	let levels = $state(none());
	let secret = $state('');
	let sample = $state('');
	let copied = $state(false);

	const chosen = $derived(resources.filter((r) => levels[r.key] !== 'none').length);
	const tomorrow = new Date(Date.now() + 86400000).toLocaleDateString('sv-SE');

	/** The end of the token's life from the expiry choice, or null for never; undefined while a custom day is missing. */
	const expiresAt = $derived.by((): Date | null | undefined => {
		if (expiry === 'never') return null;
		if (expiry === 'custom') {
			if (!customDay) return undefined;
			const [y, m, d] = customDay.split('-').map(Number);
			return new Date(y, m - 1, d, 23, 59, 59);
		}
		return new Date(Date.now() + Number(expiry) * 86400000);
	});

	/** Sets every resource to the same level; account cannot be written. */
	function preset(l: Level) {
		for (const r of resources) levels[r.key] = l === 'write' && !r.write ? 'read' : l;
	}

	/** Creates the token and switches the dialog to show it once. */
	async function create(e: SubmitEvent) {
		e.preventDefault();
		const scopes = resources.flatMap((r) =>
			levels[r.key] === 'write' ? [`${r.key}:read`, `${r.key}:write`] : levels[r.key] === 'read' ? [`${r.key}:read`] : []
		);
		try {
			const t = await call(
				api.POST('/api/me/tokens', {
					body: { name, scopes: scopes as Schemas['NewTokenInput']['scopes'], password, expires_at: expiresAt ? expiresAt.toISOString() : undefined }
				})
			);
			secret = t.token;
			// a GET the new token may call
			const first = resources.find((r) => levels[r.key] !== 'none')!;
			sample = first.key === 'account' ? '/api/me' : first.key === 'hf' ? '/api/hf/sunspots?year=2026&month=1' : `/api/${first.key}`;
			password = '';
			await oncreated();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Copies the token to the clipboard. */
	async function copy() {
		try {
			await navigator.clipboard.writeText(secret);
			copied = true;
			toast.success('Token copied');
		} catch {
			toast.error('Copying is not allowed here; select the token and copy it by hand');
		}
	}

	/** Closes the dialog and forgets the secret and the form. */
	function close() {
		open = false;
		name = password = secret = customDay = '';
		expiry = '30';
		levels = none();
		copied = false;
	}
</script>

<Dialog.Root bind:open onOpenChange={(o) => !o && close()}>
	<Dialog.Content
		showCloseButton={false}
		class="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
		interactOutsideBehavior={secret ? 'ignore' : 'close'}
		escapeKeydownBehavior={secret ? 'ignore' : 'close'}
	>
		{#if secret}
			<Dialog.Header>
				<Dialog.Title>Token created</Dialog.Title>
				<Dialog.Description>Use it as a Bearer token in the Authorization header.</Dialog.Description>
			</Dialog.Header>
			<Alert.Root class="border-warn/50 text-warn">
				<TriangleAlert />
				<Alert.Title>Copy it now</Alert.Title>
				<Alert.Description class="text-warn/90">It is stored only as a hash and will not be shown again.</Alert.Description>
			</Alert.Root>
			<div class="flex min-w-0 gap-2">
				<Input class="min-w-0 font-mono text-xs" readonly value={secret} aria-label="Token" onfocus={(e) => e.currentTarget.select()} />
				<Button variant="outline" onclick={copy}
					>{#if copied}<Check />Copied{:else}<Copy />Copy{/if}</Button
				>
			</div>
			<div class="min-w-0">
				<p class="mb-1 text-xs text-muted-foreground">Try it</p>
				<pre class="overflow-x-auto rounded-md border bg-muted px-3 py-2 font-mono text-xs">curl -H "Authorization: Bearer {secret}" \
  {location.origin}{sample}</pre>
			</div>
			<Dialog.Footer><Button onclick={close}>Done</Button></Dialog.Footer>
		{:else}
			<Dialog.Header>
				<Dialog.Title>Create API token</Dialog.Title>
				<Dialog.Description>A token lets another program use the API on your behalf, with only the access you give it here.</Dialog.Description>
			</Dialog.Header>
			<form class="grid gap-4" aria-label="Create token" onsubmit={create}>
				<Field label="Name"><Input name="name" placeholder="e.g. logbook sync" maxlength={100} required bind:value={name} /></Field>
				<div class="grid gap-1.5">
					<Field label="Expiration">
						<NativeSelect name="expiry" bind:value={expiry}>
							<option value="7">7 days</option>
							<option value="30">30 days</option>
							<option value="90">90 days</option>
							<option value="365">1 year</option>
							<option value="custom">Custom date</option>
							<option value="never">No expiration</option>
						</NativeSelect>
					</Field>
					{#if expiry === 'custom'}
						<Input type="date" name="expires_on" aria-label="Expires on" min={tomorrow} required bind:value={customDay} />
					{/if}
					{#if expiresAt}
						<p class="text-xs text-muted-foreground">
							Expires on {expiresAt.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' })}.
						</p>
					{:else if expiresAt === null}
						<p class="flex items-center gap-1.5 text-xs text-warn">
							<TriangleAlert class="size-3.5" />The token works until you revoke it. Prefer an expiration date.
						</p>
					{/if}
				</div>
				<div class="grid gap-2" role="group" aria-labelledby="token-access">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<span id="token-access" class="text-sm text-muted-foreground">Access</span>
						<div class="flex gap-3 text-xs">
							<button type="button" class="text-primary hover:underline" onclick={() => preset('read')}>All read only</button>
							<button type="button" class="text-primary hover:underline" onclick={() => preset('write')}>Full access</button>
							<button type="button" class="text-muted-foreground hover:underline" onclick={() => preset('none')}>Clear</button>
						</div>
					</div>
					<div class="divide-y rounded-md border">
						{#each resources as r (r.key)}
							<div class="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-3 py-2.5">
								<div class="min-w-0">
									<div class="text-sm font-medium">{r.label}</div>
									<div class="text-xs text-muted-foreground">{r.hint}</div>
								</div>
								<div role="radiogroup" aria-label={r.label} class="inline-flex rounded-md border bg-muted p-0.5">
									{#each [['none', 'No access'], ['read', 'Read'], ...(r.write ? [['write', 'Read & write']] : [])] as [value, label] (value)}
										<label
											class={[
												'cursor-pointer rounded px-2.5 py-1 text-xs text-muted-foreground select-none hover:text-foreground has-checked:font-medium has-checked:shadow-sm has-focus-visible:ring-2 has-focus-visible:ring-ring',
												value === 'none'
													? 'has-checked:bg-card has-checked:text-foreground'
													: 'has-checked:bg-primary has-checked:text-primary-foreground'
											]}
										>
											<input class="sr-only" type="radio" name={`scope-${r.key}`} {value} bind:group={levels[r.key]} />{label}
										</label>
									{/each}
								</div>
							</div>
						{/each}
					</div>
					<p class="text-xs text-muted-foreground">
						Account settings, passwords and tokens always need a login in the web app; no token can reach them.
					</p>
				</div>
				<Field label="Your password">
					<Input name="password" type="password" autocomplete="current-password" required bind:value={password} />
				</Field>
				<Dialog.Footer>
					<Button variant="outline" onclick={close}>Cancel</Button>
					<Button type="submit" disabled={chosen === 0 || expiresAt === undefined}>Create token</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
