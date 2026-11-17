<script lang="ts">
	import { Skeleton } from '#lib/components/ui/skeleton/index.ts';
</script>

<div class="grid gap-5" aria-busy="true" aria-label="Loading">
	<Skeleton class="h-8 w-60" />
	<div class="grid grid-cols-2 gap-3 md:grid-cols-4">
		{#each [0, 1, 2, 3] as i (i)}<Skeleton class="h-20" />{/each}
	</div>
	<div class="grid gap-5 lg:grid-cols-2">
		<Skeleton class="h-72" />
		<Skeleton class="h-72" />
	</div>
</div>
