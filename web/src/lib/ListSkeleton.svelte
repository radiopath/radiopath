<script lang="ts">
	import { Skeleton } from '#lib/components/ui/skeleton/index.ts';
</script>

<div class="grid gap-2 rounded-lg border bg-card p-3" aria-busy="true" aria-label="Loading">
	{#each [0, 1, 2] as i (i)}<Skeleton class="h-9 w-full" />{/each}
</div>
