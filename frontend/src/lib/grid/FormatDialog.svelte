<script lang="ts" module>
	export type FormatTab = 'number' | 'align' | 'font' | 'border' | 'fill';
</script>

<script lang="ts">
	// Format de cellule (Ctrl+1), comme Excel : onglets Nombre, Alignement,
	// Police, Bordure et Remplissage. Seules les propriétés modifiées sont
	// envoyées : une sélection aux styles différents garde le reste.
	import { modal } from '#lib/ui/focus.js';
	import Select from '#lib/ui/Select.svelte';
	import { categoryOf, DATE_FORMATS, formatCell, formatPatch, TIME_FORMATS, type Category } from './numfmt';
	import { cellFont, DEFAULT_PT, FONTS, SIZES } from './textwrap';
	import type { CellStyle } from './style';

	interface Props {
		style: CellStyle;
		/** Contenu de la cellule active, pour l'exemple du format. */
		sample: { text: string; value: unknown };
		tab?: FormatTab;
		/** Sélection de cellules libres (la fusion est possible) et fusion en cours. */
		canMerge?: boolean;
		merged?: boolean;
		/** merge : true fusionne, false annule la fusion, null n'y touche pas. */
		onapply: (patch: Record<string, unknown>, merge: boolean | null) => void;
		onclose: () => void;
	}

	let { style, sample, tab = 'number', canMerge = false, merged = false, onapply, onclose }: Props = $props();

	const TABS: { id: FormatTab; label: string }[] = [
		{ id: 'number', label: 'Nombre' },
		{ id: 'align', label: 'Alignement' },
		{ id: 'font', label: 'Police' },
		{ id: 'border', label: 'Bordure' },
		{ id: 'fill', label: 'Remplissage' }
	];
	const CATEGORIES: { id: Category; label: string; note: string }[] = [
		{ id: 'standard', label: 'Standard', note: 'Pas de format particulier : la valeur telle qu’elle est saisie ou calculée.' },
		{ id: 'number', label: 'Nombre', note: 'Nombre de décimales et séparateur de milliers.' },
		{ id: 'currency', label: 'Monétaire', note: 'Montant en euros.' },
		{ id: 'percent', label: 'Pourcentage', note: 'La valeur multipliée par 100, suivie de %.' },
		{ id: 'date', label: 'Date', note: 'Une date (ou un numéro de série d’Excel) affichée selon le modèle choisi.' },
		{ id: 'time', label: 'Heure', note: 'Une heure (ou une date et une heure) affichée selon le modèle choisi.' },
		{ id: 'text', label: 'Texte', note: 'Le contenu est affiché tel qu’il est saisi.' }
	];
	const COLORS = [
		'#000000', '#404040', '#7f7f7f', '#bfbfbf', '#ffffff',
		'#c00000', '#ff0000', '#ffc000', '#ffff00', '#92d050',
		'#00b050', '#00b0f0', '#0070c0', '#002060', '#7030a0',
		'#ffc7ce', '#fff2cc', '#c6efce', '#ddebf7', '#e4dfec'
	];
	const SAMPLE_DATE = '2026-10-16T08:30:00';

	// Valeurs initiales (copiées une fois : le dialogue édite une copie).
	const init = (() => ({ ...style }))();
	let current = $state<FormatTab>((() => tab)());
	// Nombre.
	let cat = $state<Category>(categoryOf(init));
	let dp = $state(init.dp ?? (init.nf === '0' ? 0 : init.nf === '%' ? 0 : 2));
	let sep = $state(!init.ng);
	let df = $state(init.df ?? '');
	// Alignement.
	let al = $state<string>(init.al ?? '');
	let va = $state<string>(init.va ?? 'middle');
	let wr = $state(!!init.wr);
	let merge = $state((() => merged)());
	// Police.
	let ff = $state(init.ff ?? FONTS[0]);
	let fs = $state(init.fs ?? DEFAULT_PT);
	let weight = $state<'n' | 'i' | 'b' | 'bi'>(init.b && init.i ? 'bi' : init.b ? 'b' : init.i ? 'i' : 'n');
	let u = $state(!!init.u);
	let x = $state(!!init.x);
	let fg = $state(init.fg ?? '');
	// Bordure et remplissage.
	let bt = $state(!!init.bt);
	let bb = $state(!!init.bb);
	let bl = $state(!!init.bl);
	let br = $state(!!init.br);
	let bc = $state(init.bc ?? '');
	let preset = $state<'' | 'none' | 'outline' | 'all'>('');
	let bg = $state(init.bg ?? '');
	// Groupes de propriétés touchés : seuls ceux-là sont envoyés.
	const touched = new Set<string>();
	const touch = (k: string) => touched.add(k);

	const dfShown = $derived(df || (cat === 'time' ? TIME_FORMATS[0] : DATE_FORMATS[0]));
	const preview = $derived.by(() => {
		const st = formatPatch(cat, { dp, sep, df: dfShown }) as CellStyle;
		const clean = Object.fromEntries(Object.entries(st).filter(([, v]) => v !== null)) as CellStyle;
		if (cat === 'date' || cat === 'time') {
			// La cellule active si elle contient une date, sinon une date d'exemple.
			const isDate = !!sample.text && formatCell(sample.text, sample.value, { nf: 'date', df: 'aaaa' }) !== sample.text;
			return isDate ? formatCell(sample.text, sample.value, clean) : formatCell(SAMPLE_DATE, SAMPLE_DATE, clean);
		}
		if (cat === 'standard') return sample.text;
		const n = typeof sample.value === 'number' ? sample.value : 1234.5;
		const text = typeof sample.value === 'number' ? sample.text : '1234,5';
		return cat === 'text' ? sample.text || 'Texte' : formatCell(text, n, clean);
	});

	function apply() {
		const p: Record<string, unknown> = {};
		if (touched.has('number')) Object.assign(p, formatPatch(cat, { dp, sep, df: dfShown }));
		if (touched.has('al')) p.al = al || null;
		if (touched.has('va')) p.va = va === 'middle' ? null : va;
		if (touched.has('wr')) p.wr = wr || null;
		if (touched.has('ff')) p.ff = ff === FONTS[0] ? null : ff;
		if (touched.has('fs')) p.fs = fs === DEFAULT_PT ? null : fs;
		if (touched.has('weight')) {
			p.b = weight === 'b' || weight === 'bi' ? true : null;
			p.i = weight === 'i' || weight === 'bi' ? true : null;
		}
		if (touched.has('u')) p.u = u || null;
		if (touched.has('x')) p.x = x || null;
		if (touched.has('fg')) p.fg = fg || null;
		if (touched.has('bg')) p.bg = bg || null;
		if (touched.has('bc')) p.bc = bc || null;
		if (preset) p.border = preset;
		if (touched.has('edges')) Object.assign(p, { bt: bt || null, bb: bb || null, bl: bl || null, br: br || null });
		onapply(p, touched.has('merge') && merge !== merged ? merge : null);
	}

	function choosePreset(k: 'none' | 'outline' | 'all') {
		preset = k;
		touched.delete('edges');
		bt = bb = bl = br = k !== 'none';
	}

	/** Le dialogue prend le focus à l'ouverture (Échap, Entrée). */
	function grab(node: HTMLElement) {
		requestAnimationFrame(() => node.focus());
	}

	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') onclose();
		if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement)) {
			e.preventDefault();
			apply();
		}
	}
</script>

{#snippet palette(value: string, set: (c: string) => void, none: string)}
	<div class="chips">
		{#each COLORS as c (c)}
			<button class="chip" class:on={value === c} style:background={c} title={c} aria-label={c} onclick={() => set(c)}></button>
		{/each}
	</div>
	<div class="row">
		<button class="plain" class:on={!value} onclick={() => set('')}>{none}</button>
		<label class="row">Autre couleur <input type="color" value={value || '#000000'} onchange={(e) => set((e.currentTarget as HTMLInputElement).value)} /></label>
	</div>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="back" onmousedown={(e) => e.target === e.currentTarget && onclose()}>
	<div class="dlg" role="dialog" aria-label="Format de cellule" tabindex="-1" aria-modal="true" use:grab use:modal onkeydown={onKey}>
		<div class="head">Format de cellule</div>
		<div class="tabs">
			{#each TABS as t (t.id)}
				<button class="tab" class:on={current === t.id} onclick={() => (current = t.id)}>{t.label}</button>
			{/each}
		</div>
		<div class="body">
			{#if current === 'number'}
				<div class="split">
					<div class="cats">
						<div class="lbl">Catégorie</div>
						{#each CATEGORIES as c (c.id)}
							<button
								class="cat"
								class:on={cat === c.id}
								onclick={() => {
									cat = c.id;
									if (c.id === 'date' && !DATE_FORMATS.includes(df)) df = DATE_FORMATS[0];
									if (c.id === 'time' && !TIME_FORMATS.includes(df)) df = TIME_FORMATS[0];
									touch('number');
								}}>{c.label}</button
							>
						{/each}
					</div>
					<div class="opts">
						<div class="lbl">Exemple</div>
						<div class="sample">{preview || ' '}</div>
						{#if cat === 'number' || cat === 'currency' || cat === 'percent'}
							<label class="row"
								>Nombre de décimales
								<input
									type="number"
									min="0"
									max="10"
									bind:value={dp}
									oninput={() => {
										dp = Math.max(0, Math.min(10, Math.round(Number(dp) || 0)));
										touch('number');
									}}
								/></label
							>
						{/if}
						{#if cat === 'number'}
							<label class="row"><input type="checkbox" bind:checked={sep} onchange={() => touch('number')} /> Utiliser le séparateur de milliers ( )</label>
						{/if}
						{#if cat === 'date' || cat === 'time'}
							<div class="lbl">Type</div>
							<div class="list">
								{#each cat === 'date' ? DATE_FORMATS : TIME_FORMATS as f (f)}
									<button
										class="cat"
										class:on={dfShown === f}
										onclick={() => {
											df = f;
											touch('number');
										}}
										><span>{formatCell(SAMPLE_DATE, SAMPLE_DATE, { nf: cat, df: f })}</span><span class="muted">{f}</span></button
									>
								{/each}
							</div>
						{/if}
						<p class="note">{CATEGORIES.find((c) => c.id === cat)?.note}</p>
					</div>
				</div>
			{:else if current === 'align'}
				<fieldset>
					<legend>Alignement du texte</legend>
					<label class="row"
						>Horizontal
						<Select
							value={al}
							width="160px"
							options={[
								{ value: '', label: 'Standard' },
								{ value: 'left', label: 'Gauche' },
								{ value: 'center', label: 'Centré' },
								{ value: 'right', label: 'Droite' }
							]}
							onchange={(v) => {
								al = v;
								touch('al');
							}}
						/></label
					>
					<label class="row"
						>Vertical
						<Select
							value={va}
							width="160px"
							options={[
								{ value: 'top', label: 'Haut' },
								{ value: 'middle', label: 'Centré' },
								{ value: 'bottom', label: 'Bas' }
							]}
							onchange={(v) => {
								va = v;
								touch('va');
							}}
						/></label
					>
				</fieldset>
				<fieldset>
					<legend>Contrôle du texte</legend>
					<label class="row"><input type="checkbox" bind:checked={wr} onchange={() => touch('wr')} /> Renvoyer à la ligne automatiquement</label>
					<label class="row" title={canMerge || merged ? '' : 'Sélectionnez plusieurs cellules libres (hors tableau) pour les fusionner'}
						><input type="checkbox" bind:checked={merge} disabled={!canMerge && !merged} onchange={() => touch('merge')} /> Fusionner les cellules</label
					>
				</fieldset>
			{:else if current === 'font'}
				<div class="split">
					<div class="opts">
						<label class="col"
							>Police
							<Select
								value={ff}
								width="200px"
								options={FONTS.map((f) => ({ value: f, label: f }))}
								onchange={(v) => {
									ff = v;
									touch('ff');
								}}
							/></label
						>
						<label class="col"
							>Style
							<Select
								value={weight}
								width="200px"
								options={[
									{ value: 'n', label: 'Normal' },
									{ value: 'i', label: 'Italique' },
									{ value: 'b', label: 'Gras' },
									{ value: 'bi', label: 'Gras italique' }
								]}
								onchange={(v) => {
									weight = v;
									touch('weight');
								}}
							/></label
						>
						<label class="col"
							>Taille
							<Select
								value={fs}
								width="200px"
								options={SIZES.map((s) => ({ value: s, label: String(s) }))}
								onchange={(v) => {
									fs = v;
									touch('fs');
								}}
							/></label
						>
						<label class="row"><input type="checkbox" bind:checked={u} onchange={() => touch('u')} /> Souligné</label>
						<label class="row"><input type="checkbox" bind:checked={x} onchange={() => touch('x')} /> Barré</label>
					</div>
					<div class="opts">
						<div class="lbl">Couleur</div>
						{@render palette(
							fg,
							(c) => {
								fg = c;
								touch('fg');
							},
							'Automatique'
						)}
						<div class="lbl">Aperçu</div>
						<div
							class="sample font"
							style:font={cellFont({ b: weight === 'b' || weight === 'bi', i: weight === 'i' || weight === 'bi', ff: ff === FONTS[0] ? undefined : ff, fs })}
							style:color={fg || '#1f2328'}
							style:text-decoration={[u ? 'underline' : '', x ? 'line-through' : ''].join(' ').trim() || 'none'}
						>
							AaBbCcYyZz
						</div>
					</div>
				</div>
			{:else if current === 'border'}
				<div class="split">
					<div class="opts">
						<div class="lbl">Présélections</div>
						<div class="row">
							<button class="plain" class:on={preset === 'none'} onclick={() => choosePreset('none')}>Aucune</button>
							<button class="plain" class:on={preset === 'outline'} onclick={() => choosePreset('outline')}>Contour</button>
							<button class="plain" class:on={preset === 'all'} onclick={() => choosePreset('all')}>Toutes</button>
						</div>
						<div class="lbl">Bordure (cliquez un bord)</div>
						<div class="edges">
							{#each [['bt', 'Haut'], ['bb', 'Bas'], ['bl', 'Gauche'], ['br', 'Droite']] as [k, label] (k)}
								<button
									class="edge {k}"
									class:on={k === 'bt' ? bt : k === 'bb' ? bb : k === 'bl' ? bl : br}
									title={label}
									aria-label={label}
									onclick={() => {
										if (k === 'bt') bt = !bt;
										else if (k === 'bb') bb = !bb;
										else if (k === 'bl') bl = !bl;
										else br = !br;
										preset = '';
										touch('edges');
									}}
								></button>
							{/each}
							<span class="txt">Texte</span>
						</div>
						<p class="note">Les bords choisis valent pour chaque cellule de la sélection ; « Contour » trace le tour de la sélection.</p>
					</div>
					<div class="opts">
						<div class="lbl">Couleur</div>
						{@render palette(
							bc,
							(c) => {
								bc = c;
								touch('bc');
							},
							'Automatique'
						)}
					</div>
				</div>
			{:else}
				<div class="lbl">Couleur de fond</div>
				{@render palette(
					bg,
					(c) => {
						bg = c;
						touch('bg');
					},
					'Aucun remplissage'
				)}
				<div class="lbl">Aperçu</div>
				<div class="sample" style:background={bg || '#fff'}>&nbsp;</div>
			{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={apply}>OK</button>
			<button onclick={onclose}>Annuler</button>
		</div>
	</div>
</div>

<style>
	.back {
		position: absolute;
		inset: 0;
		z-index: 20;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 40px;
		background: rgba(0, 0, 0, 0.08);
	}
	.dlg {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 560px;
		max-width: calc(100% - 24px);
		outline: none;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.head {
		padding: 8px 14px;
		font-weight: 600;
	}
	.tabs {
		display: flex;
		gap: 2px;
		padding: 0 10px;
		border-bottom: 1px solid #d4d7db;
	}
	.tab {
		font: inherit;
		padding: 5px 12px;
		border: 1px solid transparent;
		border-bottom: none;
		background: none;
		cursor: pointer;
		margin-bottom: -1px;
	}
	.tab.on {
		background: #fff;
		border-color: #d4d7db;
		border-bottom: 1px solid #fff;
		color: #14532d;
		font-weight: 600;
	}
	.body {
		padding: 12px 14px;
		min-height: 300px;
	}
	.split {
		display: flex;
		gap: 16px;
	}
	.cats {
		width: 150px;
		display: flex;
		flex-direction: column;
	}
	.opts {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}
	.lbl {
		font-size: 12px;
		color: #4b5563;
		margin: 4px 0 2px;
	}
	.cat {
		font: inherit;
		text-align: left;
		padding: 3px 8px;
		border: none;
		background: none;
		cursor: pointer;
		display: flex;
		justify-content: space-between;
		gap: 12px;
	}
	.cat:hover {
		background: #eef6f0;
	}
	.cat.on {
		background: #d3e7dc;
	}
	.list {
		border: 1px solid #e5e7eb;
		max-height: 170px;
		overflow: auto;
		display: flex;
		flex-direction: column;
	}
	.muted {
		color: #6b7280;
	}
	.sample {
		border: 1px solid #d4d7db;
		padding: 6px 8px;
		min-height: 20px;
		background: #fff;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.sample.font {
		height: 48px;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.note {
		margin: 6px 0 0;
		font-size: 12px;
		color: #4b5563;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.col {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		margin: 0 0 10px;
		padding: 8px 12px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	legend {
		font-size: 12px;
		color: #4b5563;
	}
	input[type='number'] {
		width: 60px;
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.chips {
		display: grid;
		grid-template-columns: repeat(5, 22px);
		gap: 4px;
	}
	.chip {
		width: 22px;
		height: 22px;
		padding: 0;
		border: 1px solid #c9cdd2;
		cursor: pointer;
	}
	.chip.on,
	.chip:hover {
		outline: 2px solid #217346;
	}
	button.plain {
		font: inherit;
		padding: 3px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	button.plain.on {
		background: #d3e7dc;
		border-color: #8fbc9f;
	}
	input[type='color'] {
		width: 28px;
		height: 20px;
		padding: 0;
		border: none;
	}
	.edges {
		position: relative;
		width: 150px;
		height: 90px;
		margin: 6px 0;
		background: #fafafa;
	}
	.edges .txt {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #9aa0a6;
		pointer-events: none;
	}
	.edge {
		position: absolute;
		padding: 0;
		border: none;
		background: transparent;
		cursor: pointer;
	}
	.edge::after {
		content: '';
		position: absolute;
		background: #d4d7db;
	}
	.edge.on::after {
		background: #1f2328;
	}
	.edge.bt,
	.edge.bb {
		left: 8px;
		right: 8px;
		height: 12px;
	}
	.edge.bt {
		top: 2px;
	}
	.edge.bb {
		bottom: 2px;
	}
	.edge.bt::after,
	.edge.bb::after {
		left: 0;
		right: 0;
		top: 5px;
		height: 2px;
	}
	.edge.bl,
	.edge.br {
		top: 8px;
		bottom: 8px;
		width: 12px;
	}
	.edge.bl {
		left: 2px;
	}
	.edge.br {
		right: 2px;
	}
	.edge.bl::after,
	.edge.br::after {
		top: 0;
		bottom: 0;
		left: 5px;
		width: 2px;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 8px 14px 12px;
	}
	.sp {
		flex: 1;
	}
	.foot button {
		font: inherit;
		padding: 4px 14px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.foot button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
