<script lang="ts">
	import { toast } from 'svelte-sonner';
	import { call, type Share } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Dialog from '#lib/components/ui/dialog/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import { day } from '#lib/format.ts';
	import { notifyError } from '#lib/notify.ts';

	// create and revoke call the share API of the object; onchange reloads it.
	let {
		what,
		hint = '',
		share,
		create,
		revoke,
		onchange
	}: {
		what: string;
		hint?: string;
		share?: Share;
		create: () => Promise<{ response: Response }>;
		revoke: () => Promise<{ response: Response }>;
		onchange: () => Promise<unknown>;
	} = $props();
	let open = $state(false);

	/** Runs a share call, confirms it with a toast and reloads the page data. */
	async function run(req: () => Promise<{ response: Response }>, done: string) {
		try {
			await call(req());
			toast.success(done);
			await onchange();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Copies the public URL to the clipboard. */
	async function copy() {
		try {
			await navigator.clipboard.writeText(share!.url);
			toast.success('Link copied');
		} catch {
			toast.error('Copying is not allowed here; select the link and copy it by hand');
		}
	}
</script>

<Button variant="outline" class={share ? 'btn-shared' : ''} title={share ? 'Public link active' : undefined} onclick={() => (open = true)}>
	{#if share}<span class="size-1.5 rounded-full bg-ok"></span>{/if}Share
</Button>

<Dialog.Root bind:open>
	<Dialog.Content showCloseButton={false} class="sm:max-w-xl">
		<Dialog.Header><Dialog.Title>Share this {what}</Dialog.Title></Dialog.Header>
		{#if share}
			<div>
				<p class="mb-1 text-xs text-muted-foreground">Public link</p>
				<div class="flex flex-wrap items-center gap-3">
					<Input class="max-w-120 font-mono text-xs" readonly value={share.url} onfocus={(e) => e.currentTarget.select()} />
					<Button variant="outline" onclick={copy}>Copy</Button>
				</div>
				<p class="mt-2 text-xs text-muted-foreground">
					Anyone with this link sees this page without an account, until {day(share.expires_at)}.{hint}
				</p>
			</div>
			<div class="flex flex-wrap gap-3">
				<Button variant="outline" onclick={() => run(create, 'Valid for another 30 days')}>Extend by 30 days</Button>
				<Button variant="destructive" onclick={() => run(revoke, 'Public link revoked')}>Revoke</Button>
				<Button variant="outline" onclick={() => (open = false)}>Close</Button>
			</div>
		{:else}
			<p class="text-xs text-muted-foreground">Creates a link that shows this page to anyone without an account, for 30 days.{hint}</p>
			<div class="flex flex-wrap gap-3">
				<Button onclick={() => run(create, 'Public link created')}>Create public link</Button>
				<Button variant="outline" onclick={() => (open = false)}>Close</Button>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
