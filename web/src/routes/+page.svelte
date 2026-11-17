<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, call } from '#lib/api.ts';
	import { onMount } from 'svelte';

	// The start page: sites first, links once there are some to connect.
	onMount(async () => {
		const sites = await call(api.GET('/api/sites'));
		await goto(sites.length ? '/links' : '/sites', { replace: true });
	});
</script>
