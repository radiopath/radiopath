<script lang="ts">
	import { Input } from '#lib/components/ui/input/index.ts';
	import Field from '#lib/Field.svelte';

	export type ManualValues = {
		name: string;
		gain_dbi: string;
		beamwidth_deg: string;
		v_beamwidth_deg: string;
		front_back_db: string;
		freq_mhz: string;
	};
	let { v = $bindable() }: { v: ManualValues } = $props();
</script>

<Field label="Name"><Input name="name" required bind:value={v.name} /></Field>
<Field label="Gain (dBi)"><Input class="font-mono" name="gain_dbi" inputmode="decimal" required bind:value={v.gain_dbi} /></Field>
<Field label="Beamwidth (deg)"><Input class="font-mono" name="beamwidth_deg" inputmode="decimal" required bind:value={v.beamwidth_deg} /></Field>
<Field label="Vertical beamwidth (deg)">
	<Input class="font-mono" name="v_beamwidth_deg" inputmode="decimal" placeholder="same as horizontal" bind:value={v.v_beamwidth_deg} />
</Field>
<Field label="Front-to-back (dB)"><Input class="font-mono" name="front_back_db" inputmode="decimal" bind:value={v.front_back_db} /></Field>
<Field label="Frequency (MHz)"><Input class="font-mono" name="freq_mhz" inputmode="decimal" placeholder="optional" bind:value={v.freq_mhz} /></Field>
