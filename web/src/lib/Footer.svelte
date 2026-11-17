<script lang="ts">
	import { createQuery } from '@tanstack/svelte-query';
	import { meQuery } from '#lib/api.ts';

	const me = createQuery(meQuery);
</script>

<footer class="mx-auto mt-auto w-full max-w-[1400px] px-5 py-6 text-center text-xs text-muted-foreground">
	Radiopath by Fabian Berg, HB9HIL &middot; <a class="text-primary" target="_blank" href="https://github.com/radiopath/radiopath">Source</a>
	&middot; ITM/Longley-Rice on Copernicus GLO-30 terrain &middot; map data &copy; OpenStreetMap contributors{#if me.data?.version}
		&middot; <span class="font-mono">{me.data.version}</span>{/if}
</footer>
