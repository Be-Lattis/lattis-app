<script lang="ts" module>
	export type FormatTab = 'number' | 'align' | 'font' | 'border' | 'fill';
</script>

<script lang="ts">
	// Format de cellule (Ctrl+1), comme Excel : onglets Nombre, Alignement,
	// Police, Bordure et Remplissage. Seules les propriétés modifiées sont
	// envoyées : une sélection aux styles différents garde le reste.
	import { t } from '#lib/i18n/index.js';
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Select from '#lib/ui/Select.svelte';
	import { categoryOf, DATE_FORMATS, formatCell, formatPatch, TIME_FORMATS, type Category } from './numfmt';
	import { cellFont, DEFAULT_PT, FONTS, SIZES } from './textwrap';
	import type { CellStyle } from './style';
	import ColorPalette from './ColorPalette.svelte';
	import { resolveColor } from './colors';
	import { tableTheme } from './theme.svelte.js';

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

	const TABS: { id: FormatTab; label: string }[] = $derived([
		{ id: 'number', label: t('grid.format.tab.number') },
		{ id: 'align', label: t('grid.format.tab.align') },
		{ id: 'font', label: t('grid.format.tab.font') },
		{ id: 'border', label: t('grid.format.tab.border') },
		{ id: 'fill', label: t('grid.format.tab.fill') }
	]);
	const CATEGORIES: { id: Category; label: string; note: string }[] = $derived([
		{ id: 'standard', label: t('grid.format.category.standard'), note: t('grid.format.category.standard.help') },
		{ id: 'number', label: t('grid.format.category.number'), note: t('grid.format.category.number.help') },
		{ id: 'currency', label: t('grid.format.category.currency'), note: t('grid.format.category.currency.help') },
		{ id: 'percent', label: t('grid.format.category.percent'), note: t('grid.format.category.percent.help') },
		{ id: 'date', label: t('grid.format.category.date'), note: t('grid.format.category.date.help') },
		{ id: 'time', label: t('grid.format.category.time'), note: t('grid.format.category.time.help') },
		{ id: 'text', label: t('grid.format.category.text'), note: t('grid.format.category.text.help') }
	]);
	/** Couleur affichée (référence au thème résolue). */
	const shown = (c: string) => String(resolveColor(c, tableTheme.current.colors));
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
		return cat === 'text' ? sample.text || t('grid.format.sampleText') : formatCell(text, n, clean);
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
	<div class="colors"><ColorPalette {value} {none} onpick={(c) => set(c ?? '')} /></div>
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label={t('grid.format.title')} tabindex="-1" aria-modal="true" use:grab use:modal onkeydown={onKey}>
		<div class="head">{t('grid.format.title')}</div>
		<div class="tabs">
			{#each TABS as tb (tb.id)}
				<button class="tab" class:on={current === tb.id} onclick={() => (current = tb.id)}>{tb.label}</button>
			{/each}
		</div>
		<div class="body">
			{#if current === 'number'}
				<div class="split">
					<div class="cats">
						<div class="lbl">{t('grid.format.category')}</div>
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
						<div class="lbl">{t('grid.format.sample')}</div>
						<div class="sample">{preview || ' '}</div>
						{#if cat === 'number' || cat === 'currency' || cat === 'percent'}
							<label class="row"
								>{t('grid.format.decimals')}
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
							<label class="row"><input type="checkbox" bind:checked={sep} onchange={() => touch('number')} /> {t('grid.format.thousands')}</label>
						{/if}
						{#if cat === 'date' || cat === 'time'}
							<div class="lbl">{t('grid.format.dateType')}</div>
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
					<legend>{t('grid.format.align.legend')}</legend>
					<label class="row"
						>{t('grid.format.align.horizontal')}
						<Select
							value={al}
							width="160px"
							options={[
								{ value: '', label: t('grid.format.align.general') },
								{ value: 'left', label: t('grid.format.align.left') },
								{ value: 'center', label: t('grid.format.align.center') },
								{ value: 'right', label: t('grid.format.align.right') }
							]}
							onchange={(v) => {
								al = v;
								touch('al');
							}}
						/></label
					>
					<label class="row"
						>{t('grid.format.align.vertical')}
						<Select
							value={va}
							width="160px"
							options={[
								{ value: 'top', label: t('grid.format.align.top') },
								{ value: 'middle', label: t('grid.format.align.center') },
								{ value: 'bottom', label: t('grid.format.align.bottom') }
							]}
							onchange={(v) => {
								va = v;
								touch('va');
							}}
						/></label
					>
				</fieldset>
				<fieldset>
					<legend>{t('grid.format.textControl')}</legend>
					<label class="row"><input type="checkbox" bind:checked={wr} onchange={() => touch('wr')} /> {t('grid.format.wrap')}</label>
					<label class="row" title={canMerge || merged ? '' : t('grid.format.merge.tooltip')}
						><input type="checkbox" bind:checked={merge} disabled={!canMerge && !merged} onchange={() => touch('merge')} /> {t('grid.format.merge')}</label
					>
				</fieldset>
			{:else if current === 'font'}
				<div class="split">
					<div class="opts">
						<label class="col"
							>{t('grid.format.font')}
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
							>{t('grid.format.font.style')}
							<Select
								value={weight}
								width="200px"
								options={[
									{ value: 'n', label: t('grid.format.font.regular') },
									{ value: 'i', label: t('grid.format.font.italic') },
									{ value: 'b', label: t('grid.format.font.bold') },
									{ value: 'bi', label: t('grid.format.font.boldItalic') }
								]}
								onchange={(v) => {
									weight = v;
									touch('weight');
								}}
							/></label
						>
						<label class="col"
							>{t('grid.format.font.size')}
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
						<label class="row"><input type="checkbox" bind:checked={u} onchange={() => touch('u')} /> {t('grid.format.font.underline')}</label>
						<label class="row"><input type="checkbox" bind:checked={x} onchange={() => touch('x')} /> {t('grid.format.font.strike')}</label>
					</div>
					<div class="opts">
						<div class="lbl">{t('grid.format.color')}</div>
						{@render palette(
							fg,
							(c) => {
								fg = c;
								touch('fg');
							},
							t('grid.format.color.auto')
						)}
						<div class="lbl">{t('grid.format.preview')}</div>
						<div
							class="sample font"
							style:font={cellFont({ b: weight === 'b' || weight === 'bi', i: weight === 'i' || weight === 'bi', ff: ff === FONTS[0] ? undefined : ff, fs })}
							style:color={fg ? shown(fg) : '#1f2328'}
							style:text-decoration={[u ? 'underline' : '', x ? 'line-through' : ''].join(' ').trim() || 'none'}
						>
							AaBbCcYyZz
						</div>
					</div>
				</div>
			{:else if current === 'border'}
				<div class="split">
					<div class="opts">
						<div class="lbl">{t('grid.format.border.presets')}</div>
						<div class="row">
							<button class="plain" class:on={preset === 'none'} onclick={() => choosePreset('none')}>{t('grid.format.border.none')}</button>
							<button class="plain" class:on={preset === 'outline'} onclick={() => choosePreset('outline')}>{t('grid.format.border.outline')}</button>
							<button class="plain" class:on={preset === 'all'} onclick={() => choosePreset('all')}>{t('grid.format.border.all')}</button>
						</div>
						<div class="lbl">{t('grid.format.border.edges')}</div>
						<div class="edges">
							{#each [['bt', t('grid.format.border.top')], ['bb', t('grid.format.border.bottom')], ['bl', t('grid.format.border.left')], ['br', t('grid.format.border.right')]] as [k, label] (k)}
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
							<span class="txt">{t('grid.format.sampleText')}</span>
						</div>
						<p class="note">{t('grid.format.border.help')}</p>
					</div>
					<div class="opts">
						<div class="lbl">{t('grid.format.color')}</div>
						{@render palette(
							bc,
							(c) => {
								bc = c;
								touch('bc');
							},
							t('grid.format.color.auto')
						)}
					</div>
				</div>
			{:else}
				<div class="lbl">{t('grid.format.fill')}</div>
				{@render palette(
					bg,
					(c) => {
						bg = c;
						touch('bg');
					},
					t('grid.format.fill.none')
				)}
				<div class="lbl">{t('grid.format.preview')}</div>
				<div class="sample" style:background={bg ? shown(bg) : '#fff'}>&nbsp;</div>
			{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" onclick={apply}>{t('common.ok')}</button>
			<button onclick={onclose}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	/* Toute la fenêtre : le ruban et les onglets ne restent pas cliquables sous une boîte modale. */
	.back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 40px;
		background: var(--backdrop);
	}
	.dlg {
		display: flex;
		flex-direction: column;
		/* Fenêtre basse : le corps défile, OK et Annuler restent visibles. */
		max-height: calc(100vh - 64px);
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
		min-height: min(300px, calc(100vh - 220px));
		overflow: auto;
	}
	.head,
	.tabs,
	.foot {
		flex-shrink: 0;
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
	.colors {
		width: max-content;
		padding: 6px;
		border: 1px solid #e5e7eb;
		border-radius: 3px;
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
