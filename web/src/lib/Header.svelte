<script lang="ts">
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import CircleUserRound from '@lucide/svelte/icons/circle-user-round';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Moon from '@lucide/svelte/icons/moon';
	import RadioTower from '@lucide/svelte/icons/radio-tower';
	import Sun from '@lucide/svelte/icons/sun';
	import User from '@lucide/svelte/icons/user';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { createQuery } from '@tanstack/svelte-query';
	import { api, meQuery } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as DropdownMenu from '#lib/components/ui/dropdown-menu/index.ts';
	import { sections } from '#lib/nav.ts';
	import { theme } from '#lib/theme.svelte.ts';
	import ThemeToggle from '#lib/ThemeToggle.svelte';

	const me = createQuery(meQuery);

	/** Ends the session and goes to the login page. */
	async function logout() {
		await api.DELETE('/api/session');
		location.assign('/login');
	}
</script>

<header class="sticky top-0 z-30 flex h-12 items-center gap-6 border-b bg-card px-4 sm:px-5">
	<a class="flex items-center gap-2 text-sm font-semibold text-foreground" href="/">
		<RadioTower class="size-5 text-primary" />Radiopath
	</a>
	{#if me.data}
		<nav class="hidden items-center gap-5 sm:flex" aria-label="Sections">
			{#each sections as s (s.href)}
				<a
					class={[
						'-mb-px flex h-12 items-center border-b-2 text-[13px] hover:text-foreground',
						page.url.pathname.startsWith(s.href) ? 'border-foreground text-foreground' : 'border-transparent text-muted-foreground'
					]}
					href={s.href}>{s.label}</a
				>
			{/each}
		</nav>
	{/if}
	<div class="ml-auto flex items-center gap-2">
		{#if me.data}
			{@const name = me.data.name}
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button variant="ghost" size="sm" {...props}
							><CircleUserRound /><span class="max-w-32 truncate">{name}</span><ChevronDown class="text-muted-foreground" /></Button
						>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="w-48">
					<DropdownMenu.Item onSelect={() => goto('/account')}><User />Account</DropdownMenu.Item>
					<DropdownMenu.Item onSelect={() => theme.toggle()}>
						{#if theme.dark}<Sun />Light theme{:else}<Moon />Dark theme{/if}
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					<DropdownMenu.Item onSelect={logout}><LogOut />Log out</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		{:else}
			<ThemeToggle />
		{/if}
	</div>
</header>
