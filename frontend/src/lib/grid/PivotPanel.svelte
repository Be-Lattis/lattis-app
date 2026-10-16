<script lang="ts">
	// Volet « Champs de tableau croisé », comme Excel : la liste des champs du
	// tableau source, et quatre zones (Filtres, Colonnes, Lignes, Valeurs) où
	// l'on glisse les champs. Chaque changement s'applique tout de suite (Ctrl+Z
	// l'annule).
	import type { PivotConfig, PivotValue } from './Sheet.svelte';
	import Select from '#lib/ui/Select.svelte';

	interface Props {
		config: PivotConfig;
		/** Champs du tableau source, avec leur type. */
		fields: { name: string; numeric: boolean }[];
		error?: string;
		filterValues?: (field: string) => Promise<{ text: string; count: number }[]>;
		onchange: (cfg: PivotConfig) => void;
		ondelete: () => void;
		onclose: () => void;
	}

	let { config, fields, error = '', filterValues, onchange, ondelete, onclose }: Props = $props();

	// Copie profonde (les objets du pont Go ne passent pas structuredClone).
	const clone = <T,>(x: T): T => JSON.parse(JSON.stringify(x));

	type Zone = 'filters' | 'cols' | 'rows' | 'values';
	const ZONES: { zone: Zone; title: string }[] = [
		{ zone: 'filters', title: 'Filtres' },
		{ zone: 'cols', title: 'Colonnes' },
		{ zone: 'rows', title: 'Lignes' },
		{ zone: 'values', title: 'Valeurs' }
	];
	const AGGS: [PivotValue['agg'], string][] = [
		['sum', 'Somme'],
		['count', 'Nombre'],
		['countDistinct', 'Nombre distinct'],
		['avg', 'Moyenne'],
		['min', 'Min'],
		['max', 'Max']
	];
	const SHOWS: [string, string][] = [
		['', 'Valeur'],
		['pctTotal', '% du total général'],
		['pctCol', '% du total de la colonne'],
		['pctRow', '% du total de la ligne']
	];

	const used = $derived(
		new Set([...(config.rowFields ?? []), ...(config.cols ? [config.cols] : []), ...(config.values ?? []).map((v) => v.field ?? ''), ...(config.filters ?? []).map((f) => f.field)])
	);

	function zoneItems(z: Zone): string[] {
		switch (z) {
			case 'rows':
				return config.rowFields ?? [];
			case 'cols':
				return config.cols ? [config.cols] : [];
			case 'filters':
				return (config.filters ?? []).map((f) => f.field);
			case 'values':
				return (config.values ?? []).map((v) => v.field ?? '');
		}
	}

	/** Retire un champ d'une zone (rang i). */
	function without(cfg: PivotConfig, z: Zone, i: number): PivotConfig {
		const c = clone(cfg);
		if (z === 'rows') c.rowFields = (c.rowFields ?? []).filter((_, k) => k !== i);
		else if (z === 'cols') c.cols = undefined;
		else if (z === 'filters') c.filters = (c.filters ?? []).filter((_, k) => k !== i);
		else c.values = (c.values ?? []).filter((_, k) => k !== i);
		return c;
	}

	/** Ajoute un champ à une zone (à la fin, ou au rang at). */
	function adding(cfg: PivotConfig, z: Zone, field: string, at?: number): PivotConfig {
		const c = clone(cfg);
		const put = <T,>(list: T[], x: T) => {
			const k = at === undefined ? list.length : Math.min(at, list.length);
			list.splice(k, 0, x);
			return list;
		};
		if (z === 'rows') c.rowFields = put((c.rowFields ?? []).filter((f) => f !== field), field);
		else if (z === 'cols') c.cols = field;
		else if (z === 'filters') {
			// Un filtre sans valeur choisie garde toutes les lignes.
			if (!(c.filters ?? []).some((f) => f.field === field)) c.filters = put(c.filters ?? [], { field, values: [] as string[] });
		} else {
			const numeric = fields.find((f) => f.name === field)?.numeric;
			c.values = put(c.values ?? [], { field, agg: numeric ? 'sum' : 'count' });
		}
		return c;
	}

	function apply(cfg: PivotConfig) {
		onchange(cfg);
	}
	const shown = $derived(config);

	function toggleField(name: string, on: boolean) {
		if (on) {
			const numeric = fields.find((f) => f.name === name)?.numeric;
			apply(adding(shown, numeric ? 'values' : 'rows', name));
			return;
		}
		let c = shown;
		for (const z of ['rows', 'cols', 'filters', 'values'] as Zone[]) {
			let i: number;
			while ((i = zoneItems2(c, z).indexOf(name)) >= 0) c = without(c, z, i);
		}
		apply(c);
	}
	function zoneItems2(c: PivotConfig, z: Zone) {
		return z === 'rows' ? (c.rowFields ?? []) : z === 'cols' ? (c.cols ? [c.cols] : []) : z === 'filters' ? (c.filters ?? []).map((f) => f.field) : (c.values ?? []).map((v) => v.field ?? '');
	}

	// Glisser-déposer : un champ de la liste ou d'une zone vers une zone.
	let drag: { field: string; from?: { zone: Zone; i: number } } | null = null;
	let over = $state<Zone | null>(null);
	function drop(z: Zone, at?: number) {
		over = null;
		const d = drag;
		drag = null;
		if (!d) return;
		let c = shown;
		if (d.from) {
			if (d.from.zone === z && at !== undefined && d.from.i < at) at--;
			c = without(c, d.from.zone, d.from.i);
		}
		apply(adding(c, z, d.field, at));
	}
	function dropOut() {
		const d = drag;
		drag = null;
		over = null;
		if (d?.from) apply(without(shown, d.from.zone, d.from.i));
	}

	// Filtre : liste des valeurs à cocher.
	let filterOpen = $state<{ field: string; values: { text: string; count: number; on: boolean }[] } | null>(null);
	async function openFilter(field: string) {
		if (!filterValues) return;
		const vals = await filterValues(field);
		const cur = (shown.filters ?? []).find((f) => f.field === field)?.values ?? [];
		filterOpen = { field, values: vals.map((v) => ({ ...v, on: cur.length === 0 || cur.includes(v.text) })) };
	}
	function applyFilter() {
		const f = filterOpen;
		if (!f) return;
		filterOpen = null;
		const all = f.values.every((v) => v.on);
		const c = clone(shown);
		c.filters = (c.filters ?? []).map((x) => (x.field === f.field ? { ...x, values: all ? [] : f.values.filter((v) => v.on).map((v) => v.text) } : x));
		apply(c);
	}

	function setValue(i: number, patch: Partial<PivotValue>) {
		const c = clone(shown);
		c.values = (c.values ?? []).map((v, k) => (k === i ? { ...v, ...patch } : v));
		apply(c);
	}
</script>

<aside class="panel">
	<header>
		<strong>Champs de tableau croisé{config.name ? ` · ${config.name}` : ''}</strong>
		<button class="x" title="Fermer le volet" onclick={onclose}>✕</button>
	</header>
	<p class="hint">Cochez les champs, ou glissez-les dans les zones ci-dessous. Le TCD s'interroge dans l'onglet SQL sous le nom {config.name}, et sert de source aux graphiques.</p>
	<div class="fields" role="list" ondragover={(e) => e.preventDefault()} ondrop={dropOut}>
		{#each fields as f (f.name)}
			<label
				class="field"
				role="listitem"
				draggable="true"
				ondragstart={(e) => {
					drag = { field: f.name };
					e.dataTransfer?.setData('text/plain', f.name);
				}}
			>
				<input type="checkbox" checked={used.has(f.name)} onchange={(e) => toggleField(f.name, (e.currentTarget as HTMLInputElement).checked)} />
				<span>{f.name}</span>
				<span class="kind">{f.numeric ? '123' : 'Abc'}</span>
			</label>
		{/each}
	</div>
	<div class="zones">
		{#each ZONES as z (z.zone)}
			<section
				class="zone"
				class:over={over === z.zone}
				role="list"
				ondragover={(e) => {
					e.preventDefault();
					over = z.zone;
				}}
				ondragleave={() => (over = null)}
				ondrop={(e) => {
					e.preventDefault();
					e.stopPropagation();
					drop(z.zone);
				}}
			>
				<h3>{z.title}</h3>
				{#each zoneItems2(shown, z.zone) as name, i (z.zone + i + name)}
					<div
						class="chip"
						role="listitem"
						draggable="true"
						ondragstart={(e) => {
							drag = { field: name, from: { zone: z.zone, i } };
							e.dataTransfer?.setData('text/plain', name);
						}}
						ondrop={(e) => {
							e.preventDefault();
							e.stopPropagation();
							drop(z.zone, i);
						}}
					>
						{#if z.zone === 'values'}
							{@const v = (shown.values ?? [])[i]}
							<Select value={v.agg} options={AGGS.map(([a, label]) => ({ value: a, label, disabled: !v.field && a !== 'count' }))} onchange={(agg) => setValue(i, { agg })} />
							<span class="name">{name || 'lignes'}</span>
							<span class="show"><Select value={v.show ?? ''} width="100%" title="Afficher les valeurs" options={SHOWS.map(([s, label]) => ({ value: s, label }))} onchange={(s) => setValue(i, { show: s || undefined })} /></span>
						{:else}
							<span class="name">{name}</span>
							{#if z.zone === 'filters'}
								{@const f = (shown.filters ?? [])[i]}
								<button class="link" onclick={() => openFilter(name)}>{f.values.length === 0 ? '(Tous)' : `${f.values.length} valeur${f.values.length > 1 ? 's' : ''}`} ▾</button>
							{/if}
						{/if}
						<button class="x" title="Retirer" onclick={() => apply(without(shown, z.zone, i))}>✕</button>
					</div>
				{/each}
				{#if z.zone === 'filters' && filterOpen}
					{@const fo = filterOpen}
					<div class="filter-pop">
						<strong>{fo.field}</strong>
						<div class="vals">
							{#each fo.values as v (v.text)}
								<label><input type="checkbox" bind:checked={v.on} /> {v.text === '' ? '(Vides)' : v.text} <span class="kind">{v.count}</span></label>
							{/each}
						</div>
						<div class="row">
							<button class="primary" onclick={applyFilter} disabled={!fo.values.some((v) => v.on)}>OK</button>
							<button onclick={() => (filterOpen = null)}>Annuler</button>
						</div>
					</div>
				{/if}
			</section>
		{/each}
	</div>
	<label class="opt"><input type="checkbox" checked={!shown.noGrandTotal} onchange={(e) => apply({ ...shown, noGrandTotal: !(e.currentTarget as HTMLInputElement).checked || undefined })} /> Totaux généraux</label>
	{#if error}<p class="error">{error}</p>{/if}
	<footer>
		<button onclick={ondelete}>Supprimer le tableau croisé</button>
	</footer>
</aside>

<style>
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
	button.primary:disabled {
		opacity: 0.5;
	}
	.panel {
		width: 300px;
		flex: none;
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 8px 10px;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		overflow: auto;
		font-size: 12px;
	}
	header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 13px;
	}
	.hint {
		margin: 0;
		color: #6b7280;
	}
	.fields {
		border: 1px solid #e5e7eb;
		background: #fff;
		max-height: 180px;
		overflow: auto;
		padding: 2px 0;
	}
	.field {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 2px 8px;
		cursor: grab;
	}
	.field span:nth-child(2) {
		flex: 1;
	}
	.kind {
		color: #9aa0a6;
		font-size: 11px;
	}
	.zones {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 6px;
	}
	.zone {
		position: relative;
		min-height: 70px;
		border: 1px solid #e5e7eb;
		background: #fff;
		padding: 4px;
	}
	.zone.over {
		border-color: #217346;
		background: #eef6f0;
	}
	h3 {
		margin: 0 0 4px;
		font-size: 11px;
		font-weight: 600;
		color: #4b5563;
	}
	.chip {
		display: flex;
		align-items: center;
		gap: 4px;
		flex-wrap: wrap;
		padding: 2px 4px;
		margin-bottom: 3px;
		background: #e3efe7;
		border: 1px solid #c5d6cb;
		border-radius: 3px;
		cursor: grab;
	}
	.chip .name {
		flex: 1;
		min-width: 40px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.chip .show {
		flex-basis: 100%;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.x,
	.link {
		border: none;
		background: none;
		padding: 0 2px;
		color: #4b5563;
	}
	.link {
		color: #217346;
	}
	.filter-pop {
		position: absolute;
		z-index: 5;
		top: 100%;
		left: 0;
		width: 220px;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 6px;
	}
	.filter-pop .vals {
		max-height: 180px;
		overflow: auto;
		margin: 4px 0;
	}
	.filter-pop label {
		display: block;
	}
	.row {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
	}
	.opt {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.error {
		color: #c0392b;
		margin: 0;
	}
	footer {
		margin-top: auto;
		padding-top: 6px;
	}
</style>
