<script lang="ts">
	import type { Coverage } from '#lib/api.ts';

	let { c }: { c: Pick<Coverage, 'legend' | 'legend_floor'> } = $props();
</script>

<table class="kv">
	<tbody>
		{#each c.legend as b (b.min_db)}
			<tr><td>{b.label}</td><td><span class="swatch inline-block h-4 w-8 border align-middle" style="background:{b.color}"></span></td></tr>
		{/each}
		<tr>
			<td>{c.legend_floor}</td>
			<td><span class="swatch inline-block h-4 w-8 border align-middle"></span> <span class="text-muted-foreground">transparent</span></td>
		</tr>
	</tbody>
</table>
