<script lang="ts">
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Plus from '@lucide/svelte/icons/plus';
	import Trash from '@lucide/svelte/icons/trash';
	import TriangleAlert from '@lucide/svelte/icons/triangle-alert';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, type Token } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Table from '#lib/components/ui/table/index.ts';
	import { ask } from '#lib/confirm.svelte.ts';
	import EmptyState from '#lib/EmptyState.svelte';
	import { day, fromNow } from '#lib/format.ts';
	import FormErrors from '#lib/FormErrors.svelte';
	import ListSkeleton from '#lib/ListSkeleton.svelte';
	import { notifyError } from '#lib/notify.ts';
	import RowActions from '#lib/RowActions.svelte';
	import TokenDialog, { scopeSummary } from '#lib/TokenDialog.svelte';

	const queryClient = useQueryClient();
	const tokens = createQuery(() => ({ queryKey: ['tokens'], queryFn: () => call(api.GET('/api/me/tokens')) }));
	let creating = $state(false);

	/** Reloads the list after a change. */
	const refresh = () => queryClient.invalidateQueries({ queryKey: ['tokens'] });

	/** Revokes a token after confirmation. */
	async function revoke(t: Token) {
		const ok = await ask({
			title: `Revoke token ${t.name}?`,
			description: 'Programs using it lose access at once. This cannot be undone.',
			action: 'Revoke',
			destructive: true
		});
		if (!ok) return;
		try {
			await call(api.DELETE('/api/me/tokens/{id}', { params: { path: { id: t.id } } }));
			toast.success(`Token ${t.name} revoked`);
			await refresh();
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<svelte:head><title>API tokens – Radiopath</title></svelte:head>
<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
	<p class="max-w-3xl text-sm text-muted-foreground">
		Tokens let other programs and scripts use the Radiopath API on your behalf. Each one reaches only what you allowed when you created it; revoke a
		token as soon as a program no longer needs it. The <a class="text-primary hover:underline" href="/account/api-reference">API reference</a> lists what
		they can do.
	</p>
	<Button onclick={() => (creating = true)}><Plus />Create token</Button>
</div>
<FormErrors error={tokens.error} />
{#if tokens.isPending}
	<ListSkeleton />
{:else if tokens.data?.length}
	<div class="rounded-lg border bg-card">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>Name</Table.Head>
					<Table.Head class="hidden md:table-cell">Access</Table.Head>
					<Table.Head>Expires</Table.Head>
					<Table.Head class="hidden sm:table-cell">Last used</Table.Head>
					<Table.Head class="hidden lg:table-cell">Created</Table.Head>
					<Table.Head class="w-12"></Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each tokens.data as t (t.id)}
					<Table.Row class={t.expired ? 'text-muted-foreground' : ''}>
						<Table.Cell class="break-words whitespace-normal">
							{t.name}<br /><span class="font-mono text-xs text-muted-foreground">{t.prefix}…</span>
						</Table.Cell>
						<Table.Cell class="hidden whitespace-normal md:table-cell">
							<div class="flex flex-wrap gap-1">
								{#each scopeSummary(t.scopes) as s (s)}
									<span class="border px-1.5 py-px font-mono text-xs">{s}</span>
								{/each}
							</div>
						</Table.Cell>
						<Table.Cell>
							{#if t.expired}
								<span class="border border-destructive px-1.5 py-px text-xs font-medium text-destructive">Expired</span>
							{:else if t.expires_at}
								<span title={new Date(t.expires_at).toLocaleString()}>{day(t.expires_at)}</span>
								<span class="block text-xs text-muted-foreground">{fromNow(t.expires_at)}</span>
							{:else}
								<span class="inline-flex items-center gap-1 text-warn"><TriangleAlert class="size-3.5" />Never</span>
							{/if}
						</Table.Cell>
						<Table.Cell class="hidden sm:table-cell">
							{#if t.last_used_at}<span title={new Date(t.last_used_at).toLocaleString()}>{fromNow(t.last_used_at)}</span>{:else}<span
									class="text-muted-foreground">Never used</span
								>{/if}
						</Table.Cell>
						<Table.Cell class="hidden lg:table-cell">{day(t.created_at)}</Table.Cell>
						<Table.Cell class="text-right">
							<RowActions actions={[{ label: 'Revoke', icon: Trash, run: () => revoke(t), destructive: true }]} />
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
	<p class="table-note mt-2 text-xs text-muted-foreground">
		{tokens.data.length} token{tokens.data.length === 1 ? '' : 's'}. Expired tokens stay listed for 30 days.
	</p>
{:else if tokens.data}
	<EmptyState icon={KeyRound}>No tokens yet. Create one to connect a program, then paste the token into its settings.</EmptyState>
{/if}

<TokenDialog bind:open={creating} oncreated={refresh} />
