<script lang="ts">
	import { page } from '$app/state';
	import { sections } from '$lib/nav';
</script>

<!-- Phones only: the sections within reach of the thumb. -->
<nav class="fixed inset-x-0 bottom-0 z-30 grid grid-cols-5 border-t bg-card pb-[env(safe-area-inset-bottom)] sm:hidden" aria-label="Sections">
	{#each sections as s (s.href)}
		{@const active = page.url.pathname.startsWith(s.href)}
		<a
			class={['flex h-14 flex-col items-center justify-center gap-1 text-[11px]', active ? 'font-medium text-primary' : 'text-muted-foreground']}
			href={s.href}
			aria-current={active ? 'page' : undefined}
		>
			<s.icon class="size-5" />{s.label}
		</a>
	{/each}
</nav>
