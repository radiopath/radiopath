<script lang="ts">
	import AntennaIcon from '@lucide/svelte/icons/antenna';
	import Pencil from '@lucide/svelte/icons/pencil';
	import Trash from '@lucide/svelte/icons/trash';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { toast } from 'svelte-sonner';
	import { api, call, parseNum, type Antenna, type ManualAntennaInput } from '#lib/api.ts';
	import { Button } from '#lib/components/ui/button/index.ts';
	import * as Dialog from '#lib/components/ui/dialog/index.ts';
	import { Input } from '#lib/components/ui/input/index.ts';
	import * as Table from '#lib/components/ui/table/index.ts';
	import Field from '#lib/Field.svelte';
	import { ask } from '#lib/confirm.svelte.ts';
	import EmptyState from '#lib/EmptyState.svelte';
	import FormErrors from '#lib/FormErrors.svelte';
	import ListSkeleton from '#lib/ListSkeleton.svelte';
	import ManualAntennaFields, { type ManualValues } from '#lib/ManualAntennaFields.svelte';
	import { invalid, notifyError } from '#lib/notify.ts';
	import PageHead from '#lib/PageHead.svelte';
	import RowActions from '#lib/RowActions.svelte';

	const queryClient = useQueryClient();
	const antennas = createQuery(() => ({ queryKey: ['antennas'], queryFn: () => call(api.GET('/api/antennas')) }));
	const blank: ManualValues = { name: '', gain_dbi: '0', beamwidth_deg: '65', v_beamwidth_deg: '', front_back_db: '25', freq_mhz: '' };
	let file: FileList | undefined = $state();
	let importName = $state('');
	let manual = $state({ ...blank });
	let editing: Antenna | null = $state(null);
	let edit = $state({ ...blank });

	/** Reloads the list after a change. */
	const refresh = () => queryClient.invalidateQueries({ queryKey: ['antennas'] });

	/** Turns the text fields into the API body; fields that are no number give one message each. */
	function manualBody(v: ManualValues): ManualAntennaInput {
		const labels = {
			gain_dbi: 'Gain',
			beamwidth_deg: 'Beamwidth',
			v_beamwidth_deg: 'Vertical beamwidth',
			front_back_db: 'Front-to-back',
			freq_mhz: 'Frequency'
		} as const;
		const optional = new Set(['v_beamwidth_deg', 'front_back_db', 'freq_mhz']);
		const n: Record<string, number | undefined> = {};
		const nan: string[] = [];
		for (const [k, label] of Object.entries(labels)) {
			const raw = v[k as keyof ManualValues];
			if (optional.has(k) && raw.trim() === '') continue;
			n[k] = parseNum(raw);
			if (isNaN(n[k]!)) nan.push(`${label}: not a number`);
		}
		if (nan.length) throw invalid(...nan);
		return {
			name: v.name,
			gain_dbi: n.gain_dbi!,
			beamwidth_deg: n.beamwidth_deg!,
			front_back_db: n.front_back_db ?? 0,
			v_beamwidth_deg: n.v_beamwidth_deg,
			freq_mhz: n.freq_mhz
		};
	}

	/** Uploads an MSI Planet file. */
	async function importFile(e: SubmitEvent) {
		e.preventDefault();
		try {
			const a = await call(
				api.POST('/api/antennas/import', {
					body: { file: '', name: importName },
					bodySerializer: () => {
						const fd = new FormData();
						fd.append('file', file![0]);
						fd.append('name', importName);
						return fd;
					}
				})
			);
			(e.target as HTMLFormElement).reset();
			importName = '';
			toast.success(`Antenna ${a.name} imported`);
			await refresh();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Creates a sector antenna from the manual form. */
	async function create(e: SubmitEvent) {
		e.preventDefault();
		try {
			const a = await call(api.POST('/api/antennas', { body: manualBody(manual) }));
			manual = { ...blank };
			toast.success(`Antenna ${a.name} created`);
			await refresh();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Opens the edit dialog with the antenna's parameters. */
	function open(a: Antenna) {
		const m = a.manual!;
		edit = {
			name: a.name,
			gain_dbi: String(a.gain_dbi),
			beamwidth_deg: String(m.beamwidth_deg),
			v_beamwidth_deg: m.v_beamwidth_deg == null ? '' : String(m.v_beamwidth_deg),
			front_back_db: String(m.front_back_db),
			freq_mhz: a.freq_mhz == null ? '' : String(a.freq_mhz)
		};
		editing = a;
	}

	/** Saves the edited antenna and closes the dialog. */
	async function save(e: SubmitEvent) {
		e.preventDefault();
		try {
			const a = await call(api.PUT('/api/antennas/{id}', { params: { path: { id: editing!.id } }, body: manualBody(edit) }));
			editing = null;
			toast.success(`Antenna ${a.name} saved`);
			await refresh();
		} catch (err) {
			notifyError(err);
		}
	}

	/** Deletes an antenna after confirmation. */
	async function remove(a: Antenna) {
		const ok = await ask({
			title: `Delete antenna ${a.name}?`,
			description: 'Links and coverages using it become omnidirectional.',
			action: 'Delete',
			destructive: true
		});
		if (!ok) return;
		try {
			await call(api.DELETE('/api/antennas/{id}', { params: { path: { id: a.id } } }));
			toast.success(`Antenna ${a.name} deleted`);
			await refresh();
		} catch (err) {
			notifyError(err);
		}
	}
</script>

<PageHead title="Antennas" />
<FormErrors error={antennas.error} />
<div class="grid grid-cols-1 items-start gap-5 lg:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
	<div>
		{#if antennas.isPending}
			<ListSkeleton />
		{:else if antennas.data?.length}
			<div class="rounded-lg border bg-card">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Azimuth</Table.Head>
							<Table.Head class="hidden sm:table-cell">Elevation</Table.Head>
							<Table.Head>Name</Table.Head>
							<Table.Head class="text-right">Gain</Table.Head>
							<Table.Head class="hidden text-right sm:table-cell">Front/back</Table.Head>
							<Table.Head class="hidden text-right md:table-cell">Frequency</Table.Head>
							<Table.Head class="w-12"></Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each antennas.data as a (a.id)}
							<Table.Row>
								<Table.Cell class="w-20 sm:w-28">{@html a.pattern_svg}</Table.Cell>
								<Table.Cell class="hidden w-28 sm:table-cell">
									{#if a.elevation_svg}{@html a.elevation_svg}{:else}<span class="text-muted-foreground" title="mirrors the azimuth cut">&mdash;</span
										>{/if}
								</Table.Cell>
								<Table.Cell class="break-words whitespace-normal">
									{a.name}{#if a.source}<br /><span class="font-mono text-xs text-muted-foreground">{a.source}</span>{/if}
								</Table.Cell>
								<Table.Cell class="text-right font-mono tabular-nums">{a.gain_dbi.toFixed(1)} dBi</Table.Cell>
								<Table.Cell class="hidden text-right font-mono tabular-nums sm:table-cell">{a.front_db.toFixed(1)} dB</Table.Cell>
								<Table.Cell class="hidden text-right font-mono tabular-nums md:table-cell">
									{#if a.freq_mhz}{a.freq_mhz.toFixed(0)} MHz{:else}<span class="text-muted-foreground">&mdash;</span>{/if}
								</Table.Cell>
								<Table.Cell class="text-right">
									<RowActions
										actions={[
											...(a.manual ? [{ label: 'Edit', icon: Pencil, run: () => open(a) }] : []),
											{ label: 'Delete', icon: Trash, run: () => remove(a), destructive: true }
										]}
									/>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
			<p class="table-note mt-2 text-xs text-muted-foreground">{antennas.data.length} antenna{antennas.data.length === 1 ? '' : 's'}</p>
		{:else if antennas.data}
			<EmptyState icon={AntennaIcon}>No antennas yet. Upload an MSI Planet file to get started.</EmptyState>
		{/if}
	</div>
	<div class="grid gap-5">
		<form class="grid gap-3 rounded-lg border bg-card p-4" aria-label="Import pattern" onsubmit={importFile}>
			<h2 class="border-b pb-1 text-xs font-medium text-muted-foreground">Import pattern</h2>
			<Field label="File"><Input type="file" name="file" accept=".msi,.pln,.ant,.txt" required bind:files={file} /></Field>
			<Field label="Name"><Input name="name" placeholder="from the file" bind:value={importName} /></Field>
			<div><Button type="submit">Import</Button></div>
			<p class="text-xs text-muted-foreground">
				MSI Planet format (<span class="font-mono">.msi</span>, <span class="font-mono">.pln</span>), as shipped by antenna manufacturers. The
				horizontal and, when present, the vertical pattern are used: 360 attenuation values in dB below the main lobe each. Without a vertical block
				the elevation cut mirrors the azimuth cut. A gain without a unit is read as dBd.
			</p>
		</form>
		<form class="grid gap-3 rounded-lg border bg-card p-4" aria-label="Describe an antenna" onsubmit={create}>
			<h2 class="border-b pb-1 text-xs font-medium text-muted-foreground">Or describe one</h2>
			<ManualAntennaFields bind:v={manual} />
			<div><Button type="submit">Create</Button></div>
			<p class="text-xs text-muted-foreground">
				Builds the usual sector shape: 12 (angle / beamwidth)&sup2; dB, capped at the front-to-back ratio. The beamwidth is the 3 dB width, 360 deg
				gives an omni; the vertical beamwidth shapes the elevation cut the same way. The front-to-back ratio is also the floor: nothing outside the
				main lobe, in azimuth or elevation, is attenuated more than that. Good enough for planning, not a substitute for the manufacturer's file.
			</p>
		</form>
	</div>
</div>

<Dialog.Root open={editing !== null} onOpenChange={(o) => !o && (editing = null)}>
	<Dialog.Content showCloseButton={false}>
		<Dialog.Header><Dialog.Title>Edit antenna</Dialog.Title></Dialog.Header>
		<form class="grid gap-3" onsubmit={save}>
			<ManualAntennaFields bind:v={edit} />
			<p class="text-xs text-muted-foreground">
				Links and coverages using this antenna take over the new gain and pattern. A computed coverage keeps its result until you recompute it.
			</p>
			<div class="flex gap-3">
				<Button type="submit">Save</Button>
				<Button variant="outline" onclick={() => (editing = null)}>Close</Button>
			</div>
		</form>
	</Dialog.Content>
</Dialog.Root>
