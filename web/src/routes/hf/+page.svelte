<script lang="ts">
	import Waves from '@lucide/svelte/icons/audio-waveform';
	import { tick } from 'svelte';
	import { createQuery, useQueryClient } from '@tanstack/svelte-query';
	import { api, call, parseNum, type HFMap, type HFPoint, type HFPredictionInput, type Site } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { NativeSelect } from '$lib/components/ui/native-select';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import EmptyState from '$lib/EmptyState.svelte';
	import Field from '$lib/Field.svelte';
	import FormErrors from '$lib/FormErrors.svelte';
	import HFWorldMap, { heat } from '$lib/HFWorldMap.svelte';
	import HFGrid from '$lib/HFGrid.svelte';
	import LineChart, { type Pt } from '$lib/LineChart.svelte';
	import { invalid, notifyError } from '$lib/notify';
	import PageHead from '$lib/PageHead.svelte';
	import InfoButton from '$lib/InfoButton.svelte';
	import Notice from '$lib/Notice.svelte';
	import StatTiles, { type Tile } from '$lib/StatTiles.svelte';

	const now = new Date();
	// The model knows a month and the hours of a day, so a range is one prediction per month.
	type Period = { year: number; month: number; hours: number[] };
	type Run = { key: string; label: string; body: HFPredictionInput; points: HFPoint[] };
	const maxMonths = 12;
	const monthNames = Array.from({ length: 12 }, (_, i) => new Date(Date.UTC(2000, i)).toLocaleString('en', { month: 'long', timeZone: 'UTC' }));
	const qc = useQueryClient();
	const sites = createQuery(() => ({ queryKey: ['sites'], queryFn: () => call(api.GET('/api/sites')) }));
	const antennas = createQuery(() => ({ queryKey: ['antennas'], queryFn: () => call(api.GET('/api/antennas')) }));
	const links = createQuery(() => ({ queryKey: ['links'], queryFn: () => call(api.GET('/api/links')) }));

	// The parameters of digital modulation; the defaults are those of the ITU program.
	const digitalFields = [
		['required_sir_db', 'Required SIR (dB)'],
		['f0_hz', 'F0 (Hz)'],
		['t0_ms', 'T0 (ms)'],
		['a_db', 'A (dB)'],
		['tw_ms', 'TW (ms)'],
		['fw_hz', 'FW (Hz)']
	] as const;

	// Rule-of-thumb figures for common modes: the required SNR is in the mode's own bandwidth, from the usual thresholds in 2500 Hz.
	// The digital modes use the defaults of the ITU program for the multipath parameters.
	const modes = [
		{ id: 'am', label: 'AM', modulation: 'analog', bandwidth_hz: '6000', required_snr_db: '20' },
		{ id: 'fm', label: 'FM', modulation: 'analog', bandwidth_hz: '12000', required_snr_db: '15' },
		{ id: 'ssb', label: 'SSB', modulation: 'analog', bandwidth_hz: '3000', required_snr_db: '15' },
		{ id: 'psk31', label: 'PSK31', modulation: 'digital', bandwidth_hz: '31.25', required_snr_db: '10' },
		{ id: 'rtty', label: 'RTTY', modulation: 'digital', bandwidth_hz: '250', required_snr_db: '15' },
		{ id: 'ft8', label: 'FT8', modulation: 'digital', bandwidth_hz: '50', required_snr_db: '-4' },
		{ id: 'ft4', label: 'FT4', modulation: 'digital', bandwidth_hz: '83.3', required_snr_db: '-3' }
	] as const;

	// The form of the last visit; nothing is stored on the server, so the browser keeps it.
	const storageKey = 'hf-form';
	const saved: Record<string, unknown> = (() => {
		try {
			return JSON.parse(localStorage.getItem(storageKey) ?? '{}') ?? {};
		} catch {
			return {};
		}
	})();
	const defaults = {
		site_a_id: '',
		site_b_id: '',
		range_start: now.toISOString().slice(0, 7),
		range_end: now.toISOString().slice(0, 7),
		range_from: '0',
		range_to: '23',
		range_step: '1',
		ssn: '60',
		frequencies_mhz: '7.1, 14.2',
		tx_power_dbm: '50',
		tx_gain_dbi: '0',
		rx_gain_dbi: '0',
		tx_antenna_id: '',
		rx_antenna_id: '',
		tx_azimuth_deg: '',
		rx_azimuth_deg: '',
		bandwidth_hz: '3000',
		required_snr_db: '15',
		snr_percentile: '90',
		noise_environment: 'rural',
		mode: 'ssb',
		modulation: 'analog',
		required_sir_db: '10',
		f0_hz: '0',
		t0_ms: '0',
		a_db: '0',
		tw_ms: '0',
		fw_hz: '0',
		long_path: false
	};
	let v = $state({
		...defaults,
		...Object.fromEntries(Object.entries(saved).filter(([k, x]) => k in defaults && typeof x === typeof defaults[k as keyof typeof defaults]))
	});
	let busy = $state(false);
	// "Predicting 3 of 12…" while the months of a range run one after the other.
	let progress = $state('');
	let error = $state<unknown>(null);
	let runs = $state<Run[] | null>(null);
	let runKey = $state('');
	// The form as it was when the shown runs were predicted; a difference marks them stale.
	let ran = $state('');
	let results: HTMLDivElement | undefined = $state();
	const stale = $derived(runs != null && JSON.stringify(v) !== ran);
	// Whether the shown points were predicted with digital modulation, which decides the reliability to show.
	let digitalRun = $state(false);

	// The reliability map of the last prediction: from site A at one of the frequencies, one frame per hour.
	let heatmap = $state<HFMap | null>(null);
	let heatError = $state<unknown>(null);
	let heatBusy = $state(false);
	let heatFreq = $state(0);
	let heatFrame = $state(0);
	let heatSites = $state<{ tx: Site; rx: Site } | null>(null);
	let heatBody = $state.raw<HFPredictionInput | null>(null);
	let heatSeq = 0;

	const mode = $derived(modes.find((m) => m.id === v.mode));

	// The smoothed sunspot number of the first month from NOAA SWPC; a 404 (no data on the server) just leaves the field alone.
	let ssnEdited = $state(saved.ssnEdited === true);
	const sunWhen = $derived.by(() => {
		const i = monthIndex(v.range_start);
		return { year: Math.floor(i / 12), month: (i % 12) + 1 };
	});
	const sunspot = createQuery(() => ({
		queryKey: ['hf', 'sunspot', sunWhen.year, sunWhen.month],
		queryFn: () => sunspotQuery(sunWhen.year, sunWhen.month),
		enabled:
			Number.isInteger(sunWhen.year) &&
			sunWhen.year >= 1749 &&
			sunWhen.year <= 2100 &&
			Number.isInteger(sunWhen.month) &&
			sunWhen.month >= 1 &&
			sunWhen.month <= 12,
		retry: false,
		staleTime: 3_600_000
	}));

	// Fills the sunspot number until the user typed one.
	$effect(() => {
		if (sunspot.data && !ssnEdited) v.ssn = String(sunspot.data.ssn);
	});

	// A new prediction starts from the first two sites; a remembered site or antenna that is gone falls back too.
	$effect(() => {
		const list = sites.data;
		if (!list?.length) return;
		if (!list.some((s) => String(s.id) === v.site_a_id)) v.site_a_id = String(list[0].id);
		if (!list.some((s) => String(s.id) === v.site_b_id)) v.site_b_id = String(list[Math.min(1, list.length - 1)].id);
	});
	$effect(() => {
		const list = antennas.data;
		if (!list) return;
		for (const k of ['tx_antenna_id', 'rx_antenna_id'] as const) if (v[k] && !list.some((a) => String(a.id) === v[k])) v[k] = '';
	});

	// Remembers the form for the next visit.
	$effect(() => {
		try {
			localStorage.setItem(storageKey, JSON.stringify({ ...v, ssnEdited }));
		} catch {
			// private window or storage blocked: the form just starts fresh next time
		}
	});

	/** Asks the server for the smoothed sunspot number of a month. */
	function sunspotQuery(year: number, month: number) {
		return call(api.GET('/api/hf/sunspots', { params: { query: { year, month } } }));
	}

	/** "October 2026" for a year and a month from 1 to 12. */
	function monthLabel(year: number, month: number): string {
		return `${monthNames[month - 1]} ${year}`;
	}

	/** Reads "2026-10" as a month count to step through; NaN for anything else. */
	function monthIndex(text: string): number {
		const m = /^(\d{4})-(\d\d)$/.exec(text);
		return m ? Number(m[1]) * 12 + Number(m[2]) - 1 : NaN;
	}

	/** The months and hours to predict: every month from the first to the last with the hours from, to and step. Problems go to bad. */
	function periods(bad: string[]): Period[] {
		const a = monthIndex(v.range_start);
		const b = monthIndex(v.range_end);
		const from = parseNum(v.range_from);
		const to = parseNum(v.range_to);
		const step = parseNum(v.range_step);
		if (isNaN(a)) bad.push('Start: not a month');
		if (isNaN(b)) bad.push('End: not a month');
		if (b < a) bad.push('End: before the start');
		if (b - a >= maxMonths) bad.push(`Range: at most ${maxMonths} months`);
		if (!(Number.isInteger(from) && Number.isInteger(to) && from >= 0 && to <= 23)) bad.push('Hours: from and to must be whole hours from 0 to 23');
		else if (to < from) bad.push('Hours: to is before from');
		if (!(Number.isInteger(step) && step >= 1)) bad.push('Step: must be a whole number of hours, 1 or more');
		if (bad.length) return [];
		// 0:00 is hour 24 of the model
		const hours: number[] = [];
		for (let h = from; h <= to; h += step) hours.push(h || 24);
		hours.sort((x, y) => x - y);
		return Array.from({ length: b - a + 1 }, (_, i) => ({ year: Math.floor((a + i) / 12), month: ((a + i) % 12) + 1, hours }));
	}

	/** Splits a list typed as "7.1, 14.2" into numbers; NaN marks an entry that is no number. */
	function list(text: string): number[] {
		return text
			.split(/[\s,;]+/)
			.filter(Boolean)
			.map(parseNum);
	}

	/** Copies the sites, antennas, aiming, gains and power of a link into the form; its frequency only if it is an HF one. */
	function fromLink(id: string) {
		const l = links.data?.find((x) => String(x.id) === id);
		if (!l) return;
		v.site_a_id = String(l.site_a_id);
		v.site_b_id = String(l.site_b_id);
		v.tx_antenna_id = String(l.tx_antenna_id ?? '');
		v.rx_antenna_id = String(l.rx_antenna_id ?? '');
		v.tx_azimuth_deg = String(l.tx_azimuth_deg ?? '');
		v.rx_azimuth_deg = String(l.rx_azimuth_deg ?? '');
		v.tx_gain_dbi = String(l.tx_gain_dbi ?? 0);
		v.rx_gain_dbi = String(l.rx_gain_dbi ?? 0);
		v.tx_power_dbm = String(l.tx_power_dbm);
		if (l.frequency_mhz >= 1 && l.frequency_mhz <= 30) v.frequencies_mhz = String(l.frequency_mhz);
	}

	/** Applies a mode: its modulation, bandwidth and required SNR; Custom keeps the values for editing. */
	function pickMode(id: string) {
		const m = modes.find((x) => x.id === id);
		if (!m) return;
		v.modulation = m.modulation;
		v.bandwidth_hz = m.bandwidth_hz;
		v.required_snr_db = m.required_snr_db;
		v.required_sir_db = '10';
		for (const k of ['f0_hz', 't0_ms', 'a_db', 'tw_ms', 'fw_hz'] as const) v[k] = '0';
	}

	/** Turns the text fields into one API body per month; fields that are no number give one message each. */
	function bodies(): HFPredictionInput[] {
		const nums = {
			ssn: 'Sunspot number',
			tx_power_dbm: 'TX power',
			tx_gain_dbi: 'TX gain',
			rx_gain_dbi: 'RX gain',
			bandwidth_hz: 'Bandwidth',
			required_snr_db: 'Required SNR',
			snr_percentile: 'SNR percentile'
		} as const;
		const n = {} as Record<keyof typeof nums, number>;
		const bad: string[] = [];
		for (const [k, label] of Object.entries(nums) as [keyof typeof nums, string][]) {
			n[k] = parseNum(v[k]);
			if (isNaN(n[k])) bad.push(`${label}: not a number`);
		}
		// An azimuth only counts with an antenna; empty aims at the other end.
		const az: Partial<Record<'tx_azimuth_deg' | 'rx_azimuth_deg', number>> = {};
		for (const [k, label, ant] of [
			['tx_azimuth_deg', 'TX azimuth', 'tx_antenna_id'],
			['rx_azimuth_deg', 'RX azimuth', 'rx_antenna_id']
		] as const) {
			if (!v[ant] || v[k].trim() === '') continue;
			az[k] = parseNum(v[k]);
			if (isNaN(az[k]!)) bad.push(`${label}: not a number`);
		}
		const dg = {} as Record<(typeof digitalFields)[number][0], number>;
		if (v.modulation === 'digital') {
			for (const [k, label] of digitalFields) {
				dg[k] = parseNum(v[k]);
				if (isNaN(dg[k])) bad.push(`${label}: not a number`);
			}
		}
		const when = periods(bad);
		const freqs = list(v.frequencies_mhz);
		if (freqs.some(isNaN)) bad.push('Frequencies: not a list of numbers');
		if (new Set(freqs).size !== freqs.length) bad.push('Frequencies: remove duplicates');
		if (bad.length) throw invalid(...bad);
		return when.map((p) => ({
			site_a_id: Number(v.site_a_id),
			site_b_id: Number(v.site_b_id),
			year: p.year,
			month: p.month,
			ssn: n.ssn,
			hours_utc: p.hours,
			frequencies_mhz: freqs,
			tx_power_dbw: Math.round((n.tx_power_dbm - 30) * 100) / 100,
			tx_gain_dbi: n.tx_gain_dbi,
			rx_gain_dbi: n.rx_gain_dbi,
			tx_antenna_id: v.tx_antenna_id ? Number(v.tx_antenna_id) : undefined,
			rx_antenna_id: v.rx_antenna_id ? Number(v.rx_antenna_id) : undefined,
			tx_azimuth_deg: az.tx_azimuth_deg,
			rx_azimuth_deg: az.rx_azimuth_deg,
			bandwidth_hz: n.bandwidth_hz,
			required_snr_db: n.required_snr_db,
			snr_percentile: n.snr_percentile,
			noise_environment: v.noise_environment as HFPredictionInput['noise_environment'],
			modulation: v.modulation as HFPredictionInput['modulation'],
			digital: v.modulation === 'digital' ? dg : undefined,
			long_path: v.long_path || undefined
		}));
	}

	/** Loads the map of a prediction at one frequency; the antennas count as isotropic with their main lobe gain. Errors show in the map panel. */
	async function loadMap(b: HFPredictionInput, freq: number) {
		const n = ++heatSeq;
		heatBusy = true;
		heatError = null;
		/** The main lobe gain of a chosen antenna, else the typed gain. */
		const gain = (id: number | undefined, fallback: number) => antennas.data?.find((a) => a.id === id)?.gain_dbi ?? fallback;
		try {
			const m = await call(
				api.POST('/api/hf/maps', {
					body: {
						site_id: b.site_a_id,
						year: b.year,
						month: b.month,
						hours_utc: [...new Set(b.hours_utc)].sort((x, y) => x - y),
						ssn: b.ssn,
						frequency_mhz: freq,
						tx_power_dbw: b.tx_power_dbw,
						tx_gain_dbi: gain(b.tx_antenna_id, b.tx_gain_dbi ?? 0),
						rx_gain_dbi: gain(b.rx_antenna_id, b.rx_gain_dbi ?? 0),
						bandwidth_hz: b.bandwidth_hz,
						required_snr_db: b.required_snr_db,
						snr_percentile: b.snr_percentile,
						noise_environment: b.noise_environment,
						modulation: b.modulation,
						digital: b.digital,
						long_path: b.long_path
					}
				})
			);
			if (n !== heatSeq) return;
			heatmap = m;
			heatFreq = freq;
			heatFrame = 0;
		} catch (err) {
			if (n === heatSeq) heatError = err;
		} finally {
			if (n === heatSeq) heatBusy = false;
		}
	}

	/** Shows the map at another of the predicted frequencies. */
	function pickFrequency(freq: number) {
		if (heatBody && freq !== heatFreq) void loadMap(heatBody, freq);
	}

	/** Shows the results and the map of another month of a range. */
	function pickRun(key: string) {
		const r = runs?.find((x) => x.key === key);
		if (!r) return;
		runKey = key;
		heatBody = r.body;
		void loadMap(r.body, r.body.frequencies_mhz.includes(heatFreq) ? heatFreq : r.body.frequencies_mhz[0]);
	}

	/** Until the user typed a sunspot number, each month uses the value of NOAA SWPC. */
	async function withSunspots(bs: HFPredictionInput[]): Promise<HFPredictionInput[]> {
		if (ssnEdited) return bs;
		return Promise.all(
			bs.map(async (b) => {
				const s = await qc
					.fetchQuery({
						queryKey: ['hf', 'sunspot', b.year, b.month],
						queryFn: () => sunspotQuery(b.year, b.month),
						staleTime: 3_600_000
					})
					.catch(() => null);
				return s ? { ...b, ssn: s.ssn } : b;
			})
		);
	}

	/** Runs the prediction of every month and the map of the first; nothing is stored. */
	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = null;
		try {
			const digital = v.modulation === 'digital';
			const form = JSON.stringify(v);
			const bs = await withSunspots(bodies());
			const b = bs[0];
			const tx = sites.data?.find((s) => s.id === b.site_a_id);
			const rx = sites.data?.find((s) => s.id === b.site_b_id);
			heatBody = b;
			heatSites = tx && rx ? { tx, rx } : null;
			if (b.frequencies_mhz.length) void loadMap(b, b.frequencies_mhz[0]);
			const done: Run[] = [];
			for (const r of bs) {
				if (bs.length > 1) progress = `Predicting ${done.length + 1} of ${bs.length}…`;
				const points = (await call(api.POST('/api/hf/predictions', { body: r }))).points;
				done.push({ key: `${r.year}-${r.month}`, label: monthLabel(r.year, r.month), body: r, points });
			}
			runKey = done[0].key;
			runs = done;
			ran = form;
			digitalRun = digital;
			await tick();
			results?.scrollIntoView({ behavior: 'smooth', block: 'start' });
		} catch (err) {
			error = err;
			notifyError(err);
		} finally {
			busy = false;
			progress = '';
		}
	}

	/** A power typed in dBm as "= 100 W" or "= 500 mW"; empty when it is no number. */
	function watts(dbm: string): string {
		const w = 10 ** ((parseNum(dbm) - 30) / 10);
		if (isNaN(w)) return '';
		if (w >= 1000) return `= ${(w / 1000).toFixed(1)} kW`;
		if (w >= 1) return `= ${w.toFixed(w < 10 ? 1 : 0)} W`;
		return `= ${(w * 1000).toFixed(0)} mW`;
	}

	/** The circuit reliability of a point: the overall one exists only for digital modulation up to 9000 km, the basic one otherwise. */
	function reliabilityValue(p: HFPoint): number | null {
		const r = digitalRun && p.distance_km <= 9000 ? p.overall_reliability_pct : p.basic_reliability_pct;
		return r === -307 ? null : r;
	}

	// The texts of the info buttons.
	const infoText = {
		muf: 'The MUF (maximum usable frequency) is the highest frequency the ionosphere still bends back to earth on this path at that hour. The solid line is the median, exceeded on half of the days of the month; the band runs from the value exceeded on 90% of the days to the one exceeded on 10%. The dashed lines are your frequencies: below the MUF the signal is usually reflected, above it most of it escapes into space.',
		reliability:
			'The share of the days of the month on which the circuit works at that hour: the signal to noise ratio reaches the one your mode needs. Analog modes and paths beyond 9000 km use the basic reliability (signal and noise only); digital modes up to 9000 km use the overall reliability, which also counts multipath and the signal to interference ratio. It is a statistic of the month, not a forecast for one day.',
		map: 'The reliability of the circuit from site A (TX) to the centre of each 10° cell, for the frequency and the hour chosen: green works on most days, red almost never. Site B (RX) is marked for reference. Use the slider to step through the hours of the month shown.'
	};

	// One colour per frequency, shared by both charts.
	const palette = ['var(--cp-accent)', 'var(--cp-ok)', 'var(--cp-warn)', 'var(--cp-ray)', 'var(--cp-fresnel)', 'var(--cp-canopy)'];
	const run = $derived(runs?.find((r) => r.key === runKey) ?? runs?.[0]);
	const shown = $derived(run?.points ?? []);
	const hours = $derived([...new Set(shown.map((p) => p.hour_utc))].sort((a, b) => a - b));
	const freqs = $derived([...new Set(shown.map((p) => p.frequency_mhz))].sort((a, b) => a - b));

	/** The key figures of the shown month: path, the most reliable hour and frequency, the span of the median MUF and the sunspot number used. */
	function tiles(): Tile[] {
		let best: HFPoint | null = null;
		let bestR = -1;
		for (const p of shown) {
			const r = reliabilityValue(p);
			if (r != null && r > bestR) [best, bestR] = [p, r];
		}
		const mufs = shown.map((p) => p.muf_50_mhz).filter((m) => m !== -307);
		return [
			{ label: 'Path length', value: `${shown[0].distance_km.toFixed(0)} km`, sub: run?.body.long_path ? 'long path' : 'short path' },
			best
				? {
						label: 'Most reliable',
						value: `${best.frequency_mhz} MHz at ${String(best.hour_utc).padStart(2, '0')}:00`,
						sub: `${bestR.toFixed(0)} % of the days`,
						tone: bestR >= 50 ? 'ok' : 'bad'
					}
				: { label: 'Most reliable', value: '–', sub: 'no reliability on this path' },
			{
				label: 'Median MUF over the day',
				value: mufs.length ? `${Math.min(...mufs).toFixed(1)} to ${Math.max(...mufs).toFixed(1)} MHz` : '–'
			},
			{ label: 'Sunspot number', value: String(run?.body.ssn ?? '–'), sub: 'R12 used by the model' }
		];
	}
	/** The MUF of one kind over the hours; the MUF does not depend on the frequency, so the first point of an hour tells it. */
	function mufOver(key: 'muf_10_mhz' | 'muf_50_mhz' | 'muf_90_mhz'): Pt[] {
		return hours.flatMap((h) => {
			const p = shown.find((q) => q.hour_utc === h);
			return p && p[key] !== -307 ? [{ x: h, y: p[key] }] : [];
		});
	}

	/** The reliability of one frequency over the hours. */
	function reliabilityOver(f: number): Pt[] {
		return hours.flatMap((h) => {
			const p = shown.find((q) => q.hour_utc === h && q.frequency_mhz === f);
			const y = p ? reliabilityValue(p) : null;
			return y == null ? [] : [{ x: h, y }];
		});
	}
</script>

<PageHead title="HF prediction" />
<div class="grid gap-6">
	<form class="grid gap-6 rounded-lg border bg-card p-4" onsubmit={submit}>
		<fieldset class="grid gap-3">
			<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Circuit</legend>
			{#if links.data?.length}
				<Field wide label="Fill from link">
					<NativeSelect class="w-68" name="from_link" value="" onchange={(e) => fromLink(e.currentTarget.value)}>
						<option value="">choose a link</option>
						{#each links.data as l (l.id)}<option value={String(l.id)}>{l.name}</option>{/each}
					</NativeSelect>
				</Field>
			{/if}
			{#each [['site_a_id', 'Site A (TX)'], ['site_b_id', 'Site B (RX)']] as const as [name, label] (name)}
				<Field wide {label}>
					<NativeSelect class="w-68" {name} bind:value={v[name]}>
						{#each sites.data ?? [] as s (s.id)}<option value={String(s.id)}>{s.name}</option>{/each}
					</NativeSelect>
				</Field>
			{/each}
			<Field wide label="Frequencies (MHz)">
				<Input class="max-w-68 font-mono" name="frequencies_mhz" placeholder="7.1, 14.2" bind:value={v.frequencies_mhz} />
			</Field>
			<Field wide label="Path">
				<span class="flex items-center gap-2">
					<input type="checkbox" name="long_path" bind:checked={v.long_path} />
					<span class="text-xs text-muted-foreground">long path (the long way round the globe)</span>
				</span>
			</Field>
		</fieldset>
		<fieldset class="grid gap-3">
			<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Time and ionosphere</legend>
			<Field wide label="First month">
				<Input class="max-w-68 font-mono" type="month" name="range_start" bind:value={v.range_start} />
			</Field>
			<Field wide label="Last month">
				<Input class="max-w-68 font-mono" type="month" name="range_end" bind:value={v.range_end} />
			</Field>
			<Field wide label="Hours (UTC, 0 to 23)">
				<span class="flex items-center gap-2">
					<Input class="w-20 font-mono" name="range_from" inputmode="numeric" aria-label="From hour" bind:value={v.range_from} />
					<span class="text-xs text-muted-foreground">to</span>
					<Input class="w-20 font-mono" name="range_to" inputmode="numeric" aria-label="To hour" bind:value={v.range_to} />
					<span class="text-xs text-muted-foreground">every</span>
					<Input class="w-20 font-mono" name="range_step" inputmode="numeric" aria-label="Step in hours" bind:value={v.range_step} />
					<span class="text-xs text-muted-foreground">h</span>
				</span>
			</Field>
			<p class="text-xs text-muted-foreground">
				The model gives the median day of each month at each hour, so there is no day to pick: each month from the first to the last (at most
				{maxMonths}) is one prediction.
			</p>
			<Field wide label="Sunspot number (R12)">
				<Input
					class="max-w-68 font-mono"
					name="ssn"
					inputmode="numeric"
					bind:value={v.ssn}
					oninput={(e) => (ssnEdited = e.currentTarget.value.trim() !== '')}
				/>
			</Field>
			{#if sunspot.data}
				<p class="text-xs text-muted-foreground" data-testid="ssn-source">
					{sunspot.data.ssn}, {sunspot.data.source} smoothed value of {sunspot.data.reference} for {v.range_start === v.range_end
						? 'this month'
						: 'the first month, the others get their own'}
				</p>
			{/if}
		</fieldset>
		<fieldset class="grid gap-3">
			<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Stations</legend>
			<Field wide label="TX power (dBm)">
				<span class="flex items-center gap-2">
					<Input class="max-w-68 font-mono" name="tx_power_dbm" inputmode="decimal" bind:value={v.tx_power_dbm} />
					<span class="text-xs text-muted-foreground">{watts(v.tx_power_dbm)}</span>
				</span>
			</Field>
			{#each [['tx_antenna_id', 'tx_gain_dbi', 'tx_azimuth_deg', 'TX'], ['rx_antenna_id', 'rx_gain_dbi', 'rx_azimuth_deg', 'RX']] as const as [ant, gain, az, label] (ant)}
				{@const sel = antennas.data?.find((a) => String(a.id) === v[ant])}
				<Field wide label="{label} antenna">
					<NativeSelect class="w-68" name={ant} bind:value={v[ant]}>
						<option value="">isotropic</option>
						{#each antennas.data ?? [] as a (a.id)}<option value={String(a.id)}>{a.name}</option>{/each}
					</NativeSelect>
				</Field>
				<Field wide label="{label} gain (dBi)">
					{#if sel}
						<Input class="max-w-68 font-mono" name={gain} readonly value={sel.gain_dbi.toFixed(1)} title="from the selected pattern" />
					{:else}
						<Input class="max-w-68 font-mono" name={gain} inputmode="decimal" bind:value={v[gain]} />
					{/if}
				</Field>
				{#if sel}
					<Field wide label="{label} main lobe azimuth (deg)">
						<Input
							class="max-w-68 font-mono"
							name={az}
							inputmode="decimal"
							placeholder={label === 'TX' ? 'towards B' : 'towards A'}
							bind:value={v[az]}
						/>
					</Field>
				{/if}
			{/each}
			<p class="text-xs text-muted-foreground">
				An antenna measured at VHF is only a rough model at HF. An empty azimuth aims the antenna at the other site.
			</p>
		</fieldset>
		<fieldset class="grid gap-3">
			<legend class="mb-2 w-full border-b pb-1 text-xs font-medium text-muted-foreground">Signal</legend>
			<Field wide label="Mode">
				<NativeSelect class="w-68" name="mode" bind:value={v.mode} onchange={(e) => pickMode(e.currentTarget.value)}>
					{#each modes as m (m.id)}<option value={m.id}>{m.label}</option>{/each}
					<option value="custom">Custom</option>
				</NativeSelect>
			</Field>
			{#if mode}
				<p class="text-xs text-muted-foreground">
					{mode.modulation}, {mode.bandwidth_hz} Hz bandwidth, {mode.required_snr_db} dB required SNR in that bandwidth. These are rule-of-thumb figures;
					choose Custom to set your own.
				</p>
			{:else}
				<Field wide label="Modulation">
					<NativeSelect class="w-68" name="modulation" bind:value={v.modulation}>
						<option value="analog">analog</option>
						<option value="digital">digital</option>
					</NativeSelect>
				</Field>
				{#if v.modulation === 'digital'}
					{#each digitalFields as [name, label] (name)}
						<Field wide {label}><Input class="max-w-68 font-mono" {name} inputmode="decimal" bind:value={v[name]} /></Field>
					{/each}
				{/if}
				<Field wide label="Bandwidth (Hz)"
					><Input class="max-w-68 font-mono" name="bandwidth_hz" inputmode="decimal" bind:value={v.bandwidth_hz} /></Field
				>
				<Field wide label="Required SNR (dB)"
					><Input class="max-w-68 font-mono" name="required_snr_db" inputmode="decimal" bind:value={v.required_snr_db} /></Field
				>
			{/if}
			<Field wide label="SNR percentile (1 to 99)">
				<Input class="max-w-68 font-mono" name="snr_percentile" inputmode="numeric" bind:value={v.snr_percentile} />
			</Field>
			<Field wide label="Noise environment">
				<NativeSelect class="w-68" name="noise_environment" bind:value={v.noise_environment}>
					{#each ['city', 'residential', 'rural', 'quiet_rural', 'noisy', 'quiet'] as n (n)}
						<option value={n}>{n.replace('_', ' ')}</option>
					{/each}
				</NativeSelect>
			</Field>
		</fieldset>
		<div class="flex items-center gap-3">
			<Button type="submit" disabled={busy || (sites.data?.length ?? 0) < 2}>{progress || 'Predict'}</Button>
			{#if sites.data && sites.data.length < 2}
				<span class="text-xs text-muted-foreground">Create at least two <a class="text-primary" href="/sites/new">sites</a> first.</span>
			{/if}
		</div>
	</form>

	<FormErrors {error} />
	{#if runs || heatSites}
		<div class="scroll-mt-16 grid gap-6" bind:this={results}>
			{#if stale}
				<Notice>The form changed since this prediction; predict again to see the effect.</Notice>
			{/if}
			{#if runs && runs.length > 1}
				<label class="flex items-center gap-3 text-sm">
					<span class="text-muted-foreground">Month shown</span>
					<NativeSelect class="w-48" name="period" value={run?.key} onchange={(e) => pickRun(e.currentTarget.value)}>
						{#each runs as r (r.key)}<option value={r.key}>{r.label}</option>{/each}
					</NativeSelect>
					<span class="text-xs text-muted-foreground">for the map and the charts</span>
				</label>
			{/if}
			{#if shown.length}
				<StatTiles tiles={tiles()} />
			{/if}
			{#if heatSites && (heatmap || heatBusy || heatError)}
				<section class="relative grid gap-3 rounded-lg border bg-card p-3" aria-label="Reliability map">
					<InfoButton label="About the reliability map" text={infoText.map} />
					<div class="flex flex-wrap items-center justify-between gap-2 pr-6">
						<h2 class="text-sm font-medium">Reliability from {heatSites.tx.name} at {heatFreq || heatBody?.frequencies_mhz[0]} MHz</h2>
						{#if freqs.length > 1}
							<NativeSelect
								class="w-32"
								name="map_frequency"
								aria-label="Frequency of the map"
								value={String(heatFreq)}
								disabled={heatBusy}
								onchange={(e) => pickFrequency(Number(e.currentTarget.value))}
							>
								{#each freqs as f (f)}<option value={String(f)}>{f} MHz</option>{/each}
							</NativeSelect>
						{/if}
					</div>
					<FormErrors error={heatError} />
					{#if heatmap}
						<div class={heatBusy ? 'opacity-60' : undefined}>
							<HFWorldMap tx={heatSites.tx} rx={heatSites.rx} map={heatmap} frame={heatFrame} longPath={heatBody?.long_path ?? false} />
						</div>
						{@const hour = String(heatmap.frames[heatFrame]?.hour_utc ?? '').padStart(2, '0')}
						{#if heatmap.frames.length > 1}
							<label class="flex items-center gap-3 text-sm">
								<span class="w-24 shrink-0 font-mono tabular-nums">{hour}:00 UTC</span>
								<input
									type="range"
									name="map_hour"
									class="w-full accent-primary"
									min="0"
									max={heatmap.frames.length - 1}
									step="1"
									aria-label="Hour of the map"
									aria-valuetext="{hour}:00 UTC"
									bind:value={heatFrame}
								/>
							</label>
						{:else}
							<p class="font-mono text-sm tabular-nums">{hour}:00 UTC</p>
						{/if}
						<div class="flex items-center gap-2 text-xs text-muted-foreground">
							<span>0 %</span>
							<span class="h-2 flex-1 rounded" style:background="linear-gradient(to right, {[0, 25, 50, 75, 100].map(heat).join(', ')})"></span>
							<span>100 %</span>
						</div>
						<p class="text-xs text-muted-foreground">
							Circuit reliability from A (TX) to the centre of each {heatmap.step_deg}° cell, B (RX) is marked for reference. Antennas count as
							isotropic with their main lobe gain. Cells without colour could not be calculated.
						</p>
					{:else if heatBusy}
						<Skeleton class="h-64 w-full" />
					{/if}
				</section>
			{/if}
			{#if shown.length}
				<div class="grid gap-4 md:grid-cols-2">
					<LineChart
						title="MUF"
						unit="MHz"
						band={{ label: 'MUF exceeded on 90% to 10% of the days', color: 'var(--cp-fg)', lo: mufOver('muf_90_mhz'), hi: mufOver('muf_10_mhz') }}
						series={[{ label: 'MUF, median', color: 'var(--cp-fg)', points: mufOver('muf_50_mhz') }]}
						refs={freqs.map((f, i) => ({ label: `${f} MHz`, color: palette[i % palette.length], y: f }))}
						info={infoText.muf}
					/>
					<LineChart
						title="Circuit reliability"
						unit="%"
						yMax={100}
						info={infoText.reliability}
						series={freqs.map((f, i) => ({ label: `${f} MHz`, color: palette[i % palette.length], points: reliabilityOver(f) }))}
					/>
				</div>
				<HFGrid runs={runs ?? []} value={reliabilityValue} selected={run?.key ?? ''} onpick={pickRun} />
			{:else if runs}
				<EmptyState icon={Waves}>The model returned no points.</EmptyState>
			{/if}
		</div>
	{/if}
</div>
