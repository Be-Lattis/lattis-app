<script lang="ts">
	// Mise en page… (lanceur des groupes de l'onglet Mise en page), comme
	// Excel : onglets Page, Marges, En-tête/Pied de page et Feuille. La boîte
	// édite une copie ; OK l'applique en une étape d'annulation.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import type { HeaderFooter, Rect } from '#bindings/internal/canvas/models.js';
	import { t } from '#lib/i18n/index.js';
	import { copySetup, fitted, HF_CODES, hfLabel, hfPresets, marginPreset, marginsOf, PAPERS, paperOf, parseRange, parseSpan, rangeA1, renderHF, sameHF, spanA1, type PageSetup, type PageSetupTab } from './pagesetup';

	interface Props {
		setup: PageSetup;
		/** Nom de la feuille et du fichier (aperçu de l'en-tête). */
		sheet: string;
		file: string;
		/** Sélection de la feuille, pour remplir une adresse. */
		selection?: Rect;
		tab?: PageSetupTab;
		onapply: (p: PageSetup) => void;
		/** Imprimer… : appliquer puis imprimer. */
		onprint?: (p: PageSetup) => void;
		onclose: () => void;
	}

	let { setup, sheet, file, selection, tab = 'page', onapply, onprint, onclose }: Props = $props();

	const TABS: { id: PageSetupTab; label: string }[] = $derived([
		{ id: 'page', label: t('shell.pageSetup.dialog.tabs.page') },
		{ id: 'margins', label: t('shell.pageSetup.dialog.tabs.margins') },
		{ id: 'hf', label: t('shell.pageSetup.dialog.tabs.hf') },
		{ id: 'sheet', label: t('shell.pageSetup.dialog.tabs.sheet') }
	]);

	// Copie éditée (prise une fois à l'ouverture).
	const init = (() => copySetup(setup))();
	let current = $state<PageSetupTab>((() => tab)());
	let landscape = $state(!!init.landscape);
	let paper = $state(paperOf(init));
	let fit = $state(fitted(init));
	let scale = $state(init.scale || 100);
	let fitWide = $state<number | ''>(fitted(init) ? init.fitWide || '' : 1);
	let fitTall = $state<number | ''>(fitted(init) ? init.fitTall || '' : '');
	let firstPage = $state(init.firstPage ? String(init.firstPage) : 'Auto');
	const m0 = marginsOf(init);
	const r2 = (v: number) => Math.round(v * 100) / 100;
	let mTop = $state(r2(m0.top));
	let mBottom = $state(r2(m0.bottom));
	let mLeft = $state(r2(m0.left));
	let mRight = $state(r2(m0.right));
	let mHeader = $state(r2(m0.header));
	let mFooter = $state(r2(m0.footer));
	let centerH = $state(!!init.centerH);
	let centerV = $state(!!init.centerV);
	let header = $state<HeaderFooter>({ ...init.header });
	let footer = $state<HeaderFooter>({ ...init.footer });
	let area = $state(init.area ? rangeA1(init.area) : '');
	let titleRows = $state(init.titleRows ? spanA1(init.titleRows, true) : '');
	let titleCols = $state(init.titleCols ? spanA1(init.titleCols, false) : '');
	let gridlines = $state(!!init.gridlines);
	let headings = $state(!!init.headings);
	let blackWhite = $state(!!init.blackWhite);
	let acrossFirst = $state(!!init.acrossFirst);
	let error = $state('');

	const ctx = $derived({ page: 1, pages: 1, sheet, file });
	const presets = $derived(hfPresets());
	const presetIndex = (h: HeaderFooter) => presets.findIndex((p) => sameHF(p, h));

	// Champ d'en-tête où vont les codes (le dernier qui a eu le focus).
	let lastField: { el: HTMLInputElement; hf: 'header' | 'footer'; part: keyof HeaderFooter } | null = null;
	function insertCode(code: string) {
		if (!lastField) return;
		const { el, hf, part } = lastField;
		const v = el.value;
		const a = el.selectionStart ?? v.length;
		const b = el.selectionEnd ?? v.length;
		const next = v.slice(0, a) + code + v.slice(b);
		if (hf === 'header') header = { ...header, [part]: next };
		else footer = { ...footer, [part]: next };
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(a + code.length, a + code.length);
		});
	}

	/** Mise en page saisie, ou un message d'erreur. */
	function build(): PageSetup | string {
		const p = copySetup(init);
		p.landscape = landscape;
		p.paper = paper === 'A4' ? '' : paper;
		if (fit) {
			const w = Number(fitWide) || 0;
			const h = Number(fitTall) || 0;
			if (w < 0 || h < 0 || w > 999 || h > 999) return t('shell.pageSetup.error.fit');
			p.fitWide = w;
			p.fitTall = h;
			if (!w && !h) p.fitWide = 1;
			p.scale = 0;
		} else {
			const s = Math.round(Number(scale));
			if (!(s >= 10 && s <= 400)) return t('shell.pageSetup.error.scale');
			p.scale = s === 100 ? 0 : s;
			p.fitWide = p.fitTall = 0;
		}
		const fp = firstPage.trim();
		if (!fp || /^auto$/i.test(fp)) p.firstPage = 0;
		else if (/^\d+$/.test(fp) && Number(fp) >= 1) p.firstPage = Number(fp);
		else return t('shell.pageSetup.error.firstPage');
		const ms = [mTop, mBottom, mLeft, mRight, mHeader, mFooter].map(Number);
		if (ms.some((v) => !(v >= 0 && v <= 50))) return t('shell.pageSetup.error.margins');
		const [top, bottom, left, right, hd, ft] = ms;
		p.margins = { top, bottom, left, right, header: hd, footer: ft };
		if (marginPreset(p) === 'normal') p.margins = null;
		p.centerH = centerH;
		p.centerV = centerV;
		p.header = { ...header };
		p.footer = { ...footer };
		const a = parseRange(area);
		if (a === undefined) return t('shell.pageSetup.error.area');
		p.area = a;
		const tr = parseSpan(titleRows, true);
		if (tr === undefined) return t('shell.pageSetup.error.titleRows');
		p.titleRows = tr;
		const tc = parseSpan(titleCols, false);
		if (tc === undefined) return t('shell.pageSetup.error.titleCols');
		p.titleCols = tc;
		p.gridlines = gridlines;
		p.headings = headings;
		p.blackWhite = blackWhite;
		p.acrossFirst = acrossFirst;
		return p;
	}

	function apply(print = false) {
		const p = build();
		if (typeof p === 'string') {
			error = p;
			return;
		}
		if (print && onprint) onprint(p);
		else onapply(p);
	}

	function fromSelection(what: 'area' | 'rows' | 'cols') {
		const s = selection;
		if (!s) return;
		if (what === 'area') area = rangeA1(s);
		else if (what === 'rows') titleRows = spanA1({ from: s.r0, to: s.r1 }, true);
		else titleCols = spanA1({ from: s.c0, to: s.c1 }, false);
	}

	function grab(node: HTMLElement) {
		requestAnimationFrame(() => node.focus());
	}

	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') onclose();
		if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement) && !(e.target instanceof HTMLSelectElement)) {
			e.preventDefault();
			apply();
		}
	}
	// Aperçu des marges : la page à l'échelle, contenu grisé.
	const pageW = $derived(landscape ? 120 : 86);
	const pageH = $derived(landscape ? 86 : 120);
	const paperCm = $derived.by(() => {
		const d: Record<string, [number, number]> = { A3: [29.7, 42], A4: [21, 29.7], A5: [14.8, 21], B5: [17.6, 25], Letter: [21.59, 27.94], Legal: [21.59, 35.56], Tabloid: [27.94, 43.18], Executive: [18.42, 26.67] };
		const [w, h] = d[paper] ?? d.A4;
		return landscape ? [h, w] : [w, h];
	});
	const k = $derived(pageW / paperCm[0]);
</script>

{#snippet hfEditor(kind: 'header' | 'footer')}
	{@const h = kind === 'header' ? header : footer}
	<label class="col">
		<span class="lbl">{kind === 'header' ? t('shell.pageSetup.dialog.header') : t('shell.pageSetup.dialog.footer')}</span>
		<select
			value={String(presetIndex(h))}
			onchange={(e) => {
				const i = Number((e.currentTarget as HTMLSelectElement).value);
				if (i < 0) return;
				if (kind === 'header') header = { ...presets[i] };
				else footer = { ...presets[i] };
			}}
		>
			{#if presetIndex(h) < 0}<option value="-1">{t('shell.pageSetup.dialog.customHF')}</option>{/if}
			{#each presets as p, i (i)}
				<option value={String(i)}>{hfLabel(p, ctx)}</option>
			{/each}
		</select>
	</label>
	<div class="hf">
		{#each [['left', t('shell.pageSetup.dialog.left')], ['center', t('shell.pageSetup.dialog.center')], ['right', t('shell.pageSetup.dialog.right')]] as [part, label] (part)}
			<label class="col"
				><span class="lbl">{label}</span>
				<input
					type="text"
					value={h[part as keyof HeaderFooter] ?? ''}
					onfocus={(e) => (lastField = { el: e.currentTarget as HTMLInputElement, hf: kind, part: part as keyof HeaderFooter })}
					oninput={(e) => {
						const v = (e.currentTarget as HTMLInputElement).value;
						if (kind === 'header') header = { ...header, [part]: v };
						else footer = { ...footer, [part]: v };
					}}
				/></label
			>
		{/each}
	</div>
	<div class="hf preview" aria-label={t('shell.pageSetup.dialog.preview')}>
		<span>{renderHF(h.left ?? '', ctx)}</span><span class="c">{renderHF(h.center ?? '', ctx)}</span><span class="r">{renderHF(h.right ?? '', ctx)}</span>
	</div>
{/snippet}

{#snippet pick(what: 'area' | 'rows' | 'cols')}
	{#if selection}<button class="plain small" title={t('shell.pageSetup.dialog.fromSelectionTooltip')} onclick={() => fromSelection(what)}>{t('shell.pageSetup.dialog.fromSelection')}</button>{/if}
{/snippet}

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label={t('shell.pageSetup.dialog.title')} tabindex="-1" aria-modal="true" use:grab use:modal onkeydown={onKey}>
		<div class="head">{t('shell.pageSetup.dialog.title')}</div>
		<div class="tabs" role="tablist">
			{#each TABS as tb (tb.id)}
				<button class="tab" role="tab" aria-selected={current === tb.id} class:on={current === tb.id} onclick={() => (current = tb.id)}>{tb.label}</button>
			{/each}
		</div>
		<div class="body">
			{#if current === 'page'}
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.orientation')}</legend>
					<div class="row">
						<label class="row"><input type="radio" name="orient" checked={!landscape} onchange={() => (landscape = false)} /> {t('shell.pageSetup.dialog.portrait')}</label>
						<label class="row"><input type="radio" name="orient" checked={landscape} onchange={() => (landscape = true)} /> {t('shell.pageSetup.dialog.landscape')}</label>
					</div>
				</fieldset>
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.scale')}</legend>
					<label class="row"
						><input type="radio" name="fit" checked={!fit} onchange={() => (fit = false)} /> {t('shell.pageSetup.dialog.scaleBefore')}
						<input type="number" min="10" max="400" bind:value={scale} disabled={fit} /> {t('shell.pageSetup.dialog.scaleAfter')}</label
					>
					<label class="row"
						><input type="radio" name="fit" checked={fit} onchange={() => (fit = true)} /> {t('shell.pageSetup.dialog.fitBefore')}
						<input type="number" min="1" max="999" bind:value={fitWide} disabled={!fit} placeholder={t('shell.pageSetup.scale.auto')} /> {t('shell.pageSetup.dialog.fitWide')}
						<input type="number" min="1" max="999" bind:value={fitTall} disabled={!fit} placeholder={t('shell.pageSetup.scale.auto')} /> {t('shell.pageSetup.dialog.fitTall')}</label
					>
					<p class="note">{t('shell.pageSetup.dialog.fitNote')}</p>
				</fieldset>
				<label class="row"
					>{t('shell.pageSetup.dialog.paper')}
					<select bind:value={paper}>
						{#each PAPERS as p (p.id)}<option value={p.id}>{t('ribbon.cmd.size.item', { name: p.label, size: p.size })}</option>{/each}
					</select></label
				>
				<label class="row">{t('shell.pageSetup.dialog.firstPage')} <input type="text" class="short" bind:value={firstPage} /></label>
			{:else if current === 'margins'}
				<div class="margins">
					<div class="grid3">
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.top')}</span><input type="number" step="0.1" min="0" bind:value={mTop} /></label>
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.header')}</span><input type="number" step="0.1" min="0" bind:value={mHeader} /></label>
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.left')}</span><input type="number" step="0.1" min="0" bind:value={mLeft} /></label>
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.right')}</span><input type="number" step="0.1" min="0" bind:value={mRight} /></label>
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.bottom')}</span><input type="number" step="0.1" min="0" bind:value={mBottom} /></label>
						<label class="col"><span class="lbl">{t('shell.pageSetup.dialog.footer')}</span><input type="number" step="0.1" min="0" bind:value={mFooter} /></label>
					</div>
					<svg class="paper" width={pageW + 2} height={pageH + 2} aria-hidden="true">
						<rect x="1" y="1" width={pageW} height={pageH} fill="#fff" stroke="#9aa0a6" />
						<rect x={1 + mLeft * k} y={1 + mTop * k} width={Math.max(pageW - (mLeft + mRight) * k, 2)} height={Math.max(pageH - (mTop + mBottom) * k, 2)} fill="#e5efe9" stroke="#8fbc9f" stroke-dasharray="2 2" />
						<line x1="1" x2={pageW + 1} y1={1 + mHeader * k} y2={1 + mHeader * k} stroke="#c9cdd2" stroke-dasharray="1 2" />
						<line x1="1" x2={pageW + 1} y1={1 + pageH - mFooter * k} y2={1 + pageH - mFooter * k} stroke="#c9cdd2" stroke-dasharray="1 2" />
					</svg>
				</div>
				<p class="note">{t('shell.pageSetup.dialog.marginsNote')}</p>
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.centerOnPage')}</legend>
					<div class="row">
						<label class="row"><input type="checkbox" bind:checked={centerH} /> {t('shell.pageSetup.dialog.horizontally')}</label>
						<label class="row"><input type="checkbox" bind:checked={centerV} /> {t('shell.pageSetup.dialog.vertically')}</label>
					</div>
				</fieldset>
			{:else if current === 'hf'}
				{@render hfEditor('header')}
				<div class="codes">
					<span class="lbl">{t('shell.pageSetup.dialog.insertCode')}</span>
					{#each HF_CODES as c (c.code)}
						<button class="plain small" title={c.code} onmousedown={(e) => e.preventDefault()} onclick={() => insertCode(c.code)}>{c.label}</button>
					{/each}
				</div>
				{@render hfEditor('footer')}
			{:else}
				<label class="row wide"
					>{t('shell.pageSetup.dialog.printArea')} <input type="text" bind:value={area} placeholder={t('shell.pageSetup.dialog.printAreaPlaceholder')} />{@render pick('area')}</label
				>
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.printTitles')}</legend>
					<label class="row wide">{t('shell.pageSetup.dialog.titleRows')} <input type="text" bind:value={titleRows} placeholder="$1:$1" />{@render pick('rows')}</label>
					<label class="row wide">{t('shell.pageSetup.dialog.titleCols')} <input type="text" bind:value={titleCols} placeholder="$A:$A" />{@render pick('cols')}</label>
				</fieldset>
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.print')}</legend>
					<label class="row"><input type="checkbox" bind:checked={gridlines} /> {t('shell.pageSetup.dialog.gridlines')}</label>
					<label class="row"><input type="checkbox" bind:checked={headings} /> {t('shell.pageSetup.dialog.headings')}</label>
					<label class="row"><input type="checkbox" bind:checked={blackWhite} /> {t('shell.pageSetup.dialog.blackWhite')}</label>
				</fieldset>
				<fieldset>
					<legend>{t('shell.pageSetup.dialog.pageOrder')}</legend>
					<label class="row"><input type="radio" name="order" checked={!acrossFirst} onchange={() => (acrossFirst = false)} /> {t('shell.pageSetup.dialog.downThenOver')}</label>
					<label class="row"><input type="radio" name="order" checked={acrossFirst} onchange={() => (acrossFirst = true)} /> {t('shell.pageSetup.dialog.overThenDown')}</label>
				</fieldset>
			{/if}
		</div>
		<div class="foot">
			{#if error}<span class="error" role="alert">{error}</span>{/if}
			<span class="sp"></span>
			{#if onprint}<button onclick={() => apply(true)}>{t('shell.pageSetup.dialog.printButton')}</button>{/if}
			<button class="primary" onclick={() => apply()}>{t('common.ok')}</button>
			<button onclick={onclose}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 50;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 60px;
		background: rgba(0, 0, 0, 0.08);
	}
	.dlg {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 600px;
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
		min-height: 330px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		margin: 0;
		padding: 8px 12px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	legend {
		font-size: 12px;
		color: #4b5563;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}
	.row.wide input[type='text'] {
		flex: 1;
		min-width: 120px;
	}
	.col {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.lbl {
		font-size: 12px;
		color: #4b5563;
	}
	.note {
		margin: 0;
		font-size: 12px;
		color: #6b7280;
	}
	input[type='number'] {
		width: 64px;
	}
	input[type='text'].short {
		width: 70px;
	}
	input[type='number'],
	input[type='text'],
	select {
		font: inherit;
		padding: 2px 4px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	.margins {
		display: flex;
		gap: 24px;
		align-items: center;
	}
	.grid3 {
		display: grid;
		grid-template-columns: repeat(2, 110px);
		gap: 8px 16px;
	}
	.paper {
		flex: none;
	}
	.hf {
		display: grid;
		grid-template-columns: 1fr 1fr 1fr;
		gap: 8px;
	}
	.hf input {
		width: 100%;
		box-sizing: border-box;
	}
	.hf.preview {
		border: 1px solid #e5e7eb;
		background: #fafafa;
		padding: 4px 8px;
		min-height: 18px;
		font-size: 12px;
	}
	.hf.preview .c {
		text-align: center;
	}
	.hf.preview .r {
		text-align: right;
	}
	.codes {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
		margin: 4px 0;
	}
	button.plain {
		font: inherit;
		padding: 3px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	button.small {
		padding: 1px 8px;
		font-size: 12px;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 8px 14px 12px;
		align-items: center;
	}
	.error {
		color: #b42318;
		font-size: 12px;
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
	}
</style>
