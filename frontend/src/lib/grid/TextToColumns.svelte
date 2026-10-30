<script lang="ts">
	import { errorText, t } from '#lib/i18n/index.js';
	// Assistant « Convertir le texte en colonnes » (Données › Convertir), en
	// trois étapes comme Excel : type des données, séparateurs ou coupures,
	// format et destination de chaque colonne. L'aperçu vient du backend, qui
	// coupe le texte exactement comme la conversion.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import Select from '#lib/ui/Select.svelte';
	import {
		addBreak,
		DATE_ORDERS,
		delimString,
		formatLabel,
		guessDelims,
		guessFixed,
		maxChars,
		suggestBreaks,
		type ColFormat,
		type Delims,
		type TextColumnFormat,
		type TextColumnsOptions,
		type TextColumnsPreview
	} from './textcols';
	import { bold, rich } from './rich';

	interface Props {
		preview: (o: TextColumnsOptions) => Promise<TextColumnsPreview>;
		/** Convertit ; renvoie le nombre de colonnes importées. */
		apply: (o: TextColumnsOptions) => Promise<number>;
		ondone: (n: number, table: string) => void;
		onclose: () => void;
	}

	let { preview, apply, ondone, onclose }: Props = $props();

	let step = $state(1);
	let fixed = $state(false);
	let delims = $state<Delims>({ tab: true, semi: false, comma: false, space: false, other: false, otherChar: '' });
	let merge = $state(false);
	let quote = $state<string>('"');
	let breaks = $state<number[]>([]);
	let formats = $state<TextColumnFormat[]>([]);
	let selCol = $state(0);
	let decimal = $state(',');
	let thousands = $state(' ');
	let dest = $state('');
	let advanced = $state(false);
	let confirm = $state(false);
	let busy = $state(false);
	let error = $state('');
	let pv = $state<TextColumnsPreview | null>(null);
	// Première lecture : propose le type et les séparateurs d'après le texte.
	let guessed = false;

	const opts = $derived<TextColumnsOptions>({
		fixed,
		delims: delimString(delims),
		merge,
		quote,
		breaks: [...breaks],
		columns: formats.map((f) => ({ ...f })),
		decimal,
		thousands,
		dest
	});

	const lines = $derived(pv?.lines ?? []);
	const rows = $derived((pv?.rows ?? []).map((r) => r ?? []));
	const count = $derived(Math.max(1, pv?.count ?? 1));
	const width = $derived(maxChars(lines));
	const inTable = $derived(!!pv?.table);
	const fmt = (j: number): TextColumnFormat => formats[j] ?? { format: '', order: 'DMY' };

	let seq = 0;
	$effect(() => {
		const o = opts;
		const n = ++seq;
		preview(o)
			.then((p) => {
				if (n !== seq) return;
				if (!guessed) {
					guessed = true;
					const ls = p.lines ?? [];
					fixed = guessFixed(ls);
					delims = guessDelims(ls);
					breaks = fixed ? suggestBreaks(ls) : [];
					dest = p.dest;
					return; // les options ont changé : nouvel aperçu
				}
				pv = p;
				error = '';
				if (selCol >= p.count) selCol = 0;
			})
			.catch((e) => {
				if (n === seq) error = errorText(e);
			});
	});

	function setFormat(j: number, patch: Partial<TextColumnFormat>) {
		const next = [...formats];
		while (next.length <= j) next.push({ format: '', order: 'DMY' });
		next[j] = { ...next[j], ...patch };
		formats = next;
	}

	// --- Largeur fixe : coupures dans l'aperçu --------------------------------

	let ruler = $state<HTMLDivElement>();
	let chPx = $state(8);
	const PAD = 6;

	function measure(node: HTMLElement) {
		chPx = node.getBoundingClientRect().width / 10 || 8;
	}

	function posAt(e: MouseEvent): number {
		const r = ruler!.getBoundingClientRect();
		return Math.round((e.clientX - r.left + ruler!.scrollLeft - PAD) / chPx);
	}

	let drag: { i: number; moved: boolean } | null = null;
	let justDragged = false;

	function addAt(e: MouseEvent) {
		if (justDragged) {
			justDragged = false;
			return;
		}
		breaks = addBreak(breaks, posAt(e), Math.max(width, 2));
	}

	function startDrag(e: MouseEvent, i: number) {
		e.preventDefault();
		e.stopPropagation();
		drag = { i, moved: false };
		const box = ruler!.getBoundingClientRect();
		const move = (ev: MouseEvent) => {
			if (!drag) return;
			drag.moved = true;
			const pos = posAt(ev);
			const out = ev.clientY < box.top - 20 || ev.clientY > box.bottom + 20;
			// Pendant le glissement, la coupure suit la souris (elle reste à son rang).
			const next = [...breaks];
			next[drag.i] = out ? -1 : Math.max(1, Math.min(pos, Math.max(width, 2) - 1));
			breaks = next;
		};
		const up = (ev: MouseEvent) => {
			window.removeEventListener('mousemove', move);
			window.removeEventListener('mouseup', up);
			if (drag?.moved) {
				const pos = posAt(ev);
				const out = ev.clientY < box.top - 20 || ev.clientY > box.bottom + 20;
				const rest = breaks.filter((_, k) => k !== drag!.i);
				breaks = out ? rest : addBreak(rest, pos, Math.max(width, 2));
				justDragged = true;
				setTimeout(() => (justDragged = false), 0);
			}
			drag = null;
		};
		window.addEventListener('mousemove', move);
		window.addEventListener('mouseup', up);
	}

	function removeBreak(e: MouseEvent, i: number) {
		e.stopPropagation();
		breaks = breaks.filter((_, k) => k !== i);
	}

	// --- Fin ------------------------------------------------------------------

	async function finish(force = false) {
		if (busy) return;
		error = '';
		busy = true;
		try {
			const p = await preview(opts);
			if (!force && p.overwrite) {
				confirm = true;
				return;
			}
			const n = await apply(opts);
			ondone(n, p.table);
		} catch (e) {
			error = errorText(e);
		} finally {
			busy = false;
		}
	}

	function grab(node: HTMLElement) {
		requestAnimationFrame(() => node.focus());
	}

	function onKey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') {
			if (confirm) confirm = false;
			else onclose();
		}
		if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement) && !(e.target instanceof HTMLInputElement && e.target.type === 'text')) {
			e.preventDefault();
			if (confirm) finish(true);
			else if (step < 3) step++;
			else finish();
		}
	}

	const FORMATS: { value: ColFormat; label: string }[] = $derived([
		{ value: '', label: t('grid.textCols.format.general') },
		{ value: 'text', label: t('grid.textCols.format.text') },
		{ value: 'date', label: t('grid.textCols.format.date') },
		{ value: 'skip', label: t('grid.textCols.format.notImported') }
	]);
	const QUOTES = $derived([
		{ value: '"', label: '"' },
		{ value: "'", label: "'" },
		{ value: '', label: `{${t('grid.textCols.quote.none')}}` }
	]);
	const DECIMALS = $derived([
		{ value: ',', label: t('grid.textCols.sep.comma') },
		{ value: '.', label: t('grid.textCols.sep.period') }
	]);
	const THOUSANDS = $derived([
		{ value: ' ', label: t('grid.textCols.sep.space') },
		{ value: '.', label: t('grid.textCols.sep.period') },
		{ value: ',', label: t('grid.textCols.sep.comma') },
		{ value: "'", label: t('grid.textCols.sep.apostrophe') },
		{ value: '', label: t('grid.textCols.sep.none') }
	]);
	const STEPS = $derived([t('grid.textCols.step.kind'), t('grid.textCols.step.separators'), t('grid.textCols.step.formats')]);
	const shown = (s: string) => s.replaceAll('\t', '→');
</script>

{#snippet table(withHead: boolean)}
	<div class="pv">
		<table>
			{#if withHead}
				<thead>
					<tr>
						{#each { length: count } as _, j (j)}
							<th class:on={selCol === j} class:skip={fmt(j).format === 'skip'} onclick={() => (selCol = j)}>{formatLabel(fmt(j))}</th>
						{/each}
					</tr>
				</thead>
			{/if}
			<tbody>
				{#each rows as r, i (i)}
					<tr class:head={pv?.header && i === 0}>
						{#each { length: count } as _, j (j)}
							<td class:on={withHead && selCol === j} class:skip={withHead && fmt(j).format === 'skip'} onclick={() => withHead && (selCol = j)}>{r[j] ?? ''}</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/snippet}

<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label={t('grid.textCols.title')} tabindex="-1" aria-modal="true" use:grab use:modal onkeydown={onKey}>
		<div class="head">{t('grid.textCols.heading', { step, name: STEPS[step - 1] })}</div>
		<div class="body">
			{#if pv}
				<p class="note">
					{#if inTable}
						{#each rich(t(pv.header ? 'grid.textCols.note.tableWithHeader' : 'grid.textCols.note.table', { table: bold(pv.table), column: bold(pv.column) })) as p, k (k)}{#if p.b}<b>{p.text}</b>{:else}{p.text}{/if}{/each}
					{:else}
						{t('grid.textCols.note.cells', { n: pv.total })}
					{/if}
				</p>
			{/if}
			{#if step === 1}
				<fieldset>
					<legend>{t('grid.textCols.kind.legend')}</legend>
					<label class="row"><input type="radio" name="ttc-kind" checked={!fixed} onchange={() => (fixed = false)} /><b>{t('grid.textCols.kind.delimited')}</b><span class="muted">{t('grid.textCols.kind.delimited.help')}</span></label>
					<label class="row"><input type="radio" name="ttc-kind" checked={fixed} onchange={() => {
						fixed = true;
						if (!breaks.length) breaks = suggestBreaks(lines);
					}} /><b>{t('grid.textCols.kind.fixed')}</b><span class="muted">{t('grid.textCols.kind.fixed.help')}</span></label>
				</fieldset>
				<div class="lbl">{t('grid.textCols.preview.selection')}</div>
				<div class="pv mono">
					{#each lines as l, i (i)}
						<div class="line" class:head={pv?.header && i === 0}><span class="num">{i + 1}</span>{shown(l)}</div>
					{/each}
				</div>
			{:else if step === 2 && !fixed}
				<div class="split">
					<fieldset>
						<legend>{t('grid.textCols.step.separators')}</legend>
						<label class="row"><input type="checkbox" bind:checked={delims.tab} />{t('grid.textCols.delim.tab')}</label>
						<label class="row"><input type="checkbox" bind:checked={delims.semi} />{t('grid.textCols.delim.semicolon')}</label>
						<label class="row"><input type="checkbox" bind:checked={delims.comma} />{t('grid.textCols.delim.comma')}</label>
						<label class="row"><input type="checkbox" bind:checked={delims.space} />{t('grid.textCols.delim.space')}</label>
						<label class="row"><input type="checkbox" bind:checked={delims.other} />{t('grid.textCols.delim.other')}
							<input class="other" type="text" maxlength="1" bind:value={delims.otherChar} oninput={() => (delims.other = delims.otherChar !== '')} aria-label={t('grid.textCols.delim.other.aria')} /></label>
					</fieldset>
					<div class="col">
						<label class="row"><input type="checkbox" bind:checked={merge} />{t('grid.textCols.merge')}</label>
						<div class="row">{t('grid.textCols.quote')} <Select bind:value={quote} options={QUOTES} width="90px" /></div>
					</div>
				</div>
				<div class="lbl">{t('grid.textCols.preview')}</div>
				{@render table(false)}
			{:else if step === 2}
				<p class="hint">
					{t('grid.textCols.fixed.hint')}
				</p>
				<div class="lbl">{t('grid.textCols.preview')}</div>
				<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (coupures à la souris seulement ; équivalent au clavier à faire, voir l'audit d'ergonomie) -->
				<div class="pv mono ruler" bind:this={ruler} onclick={addAt} style:--ch="{chPx}px">
					<span class="meter" use:measure>0000000000</span>
					<div class="scale" style:width="{(width + 2) * chPx + PAD * 2}px">
						{#each { length: Math.ceil((width + 2) / 10) + 1 } as _, k (k)}
							<span class="tick" style:left="{PAD + k * 10 * chPx}px">{k * 10}</span>
						{/each}
					</div>
					<div class="text" style:width="{(width + 2) * chPx + PAD * 2}px">
						{#each lines as l, i (i)}
							<div class="fline" class:head={pv?.header && i === 0}>{shown(l) || ' '}</div>
						{/each}
						{#each breaks as b, i (i)}
							{#if b > 0}
								<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions (coupures à la souris seulement ; équivalent au clavier à faire, voir l'audit d'ergonomie) -->
								<div class="brk" style:left="{PAD + b * chPx}px" onmousedown={(e) => startDrag(e, i)} ondblclick={(e) => removeBreak(e, i)} onclick={(e) => e.stopPropagation()} title={t('grid.textCols.break.tooltip')}></div>
							{/if}
						{/each}
					</div>
				</div>
			{:else}
				<div class="split">
					<fieldset>
						<legend>{t('grid.textCols.format.legend', { n: selCol + 1 })}</legend>
						{#each FORMATS as f (f.value)}
							<label class="row">
								<input type="radio" name="ttc-fmt" checked={fmt(selCol).format === f.value} onchange={() => setFormat(selCol, { format: f.value })} />{f.label}
								{#if f.value === 'date'}
									<Select value={fmt(selCol).order || 'DMY'} options={DATE_ORDERS} width="70px" disabled={fmt(selCol).format !== 'date'} onchange={(v) => setFormat(selCol, { format: 'date', order: v })} />
								{/if}
							</label>
						{/each}
					</fieldset>
					<div class="col">
						<p class="hint">{t('grid.textCols.format.hint')}</p>
						<label class="row dest">{t('grid.textCols.dest')}
							<input type="text" bind:value={dest} disabled={inTable} title={inTable ? t('grid.textCols.dest.table.tooltip') : t('grid.textCols.dest.tooltip')} />
						</label>
						<button class="plain" onclick={() => (advanced = !advanced)}>{advanced ? t('grid.textCols.advanced.open') : t('grid.textCols.advanced')}</button>
						{#if advanced}
							<div class="adv">
								<div class="row">{t('grid.textCols.decimal')} <Select bind:value={decimal} options={DECIMALS} width="120px" /></div>
								<div class="row">{t('grid.textCols.thousands')} <Select bind:value={thousands} options={THOUSANDS} width="120px" /></div>
							</div>
						{/if}
					</div>
				</div>
				<div class="lbl">{t('grid.textCols.preview')}</div>
				{@render table(true)}
			{/if}
			{#if error}<p class="error">{error}</p>{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button onclick={onclose}>{t('common.cancel')}</button>
			<button disabled={step === 1} onclick={() => step--}>{t('grid.textCols.back')}</button>
			<button disabled={step === 3} onclick={() => step++}>{t('grid.textCols.next')}</button>
			<button class="primary" disabled={busy || !pv} onclick={() => finish()}>{t('grid.textCols.finish')}</button>
		</div>
		{#if confirm}
			<div class="confirm" role="alertdialog" aria-label={t('grid.textCols.confirm.title')}>
				<div class="box">
					<p>{t('grid.textCols.confirm')}</p>
					<div class="foot">
						<span class="sp"></span>
						<button class="primary" onclick={() => {
							confirm = false;
							finish(true);
						}}>{t('common.ok')}</button>
						<button onclick={() => (confirm = false)}>{t('common.cancel')}</button>
					</div>
				</div>
			</div>
		{/if}
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
		position: relative;
		display: flex;
		flex-direction: column;
		/* Fenêtre basse : le corps défile, OK et Annuler restent visibles. */
		max-height: calc(100vh - 64px);
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 640px;
		max-width: calc(100% - 24px);
		outline: none;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.head {
		padding: 8px 14px;
		font-weight: 600;
		border-bottom: 1px solid #d4d7db;
	}
	.head,
	.foot {
		flex-shrink: 0;
	}
	.body {
		padding: 10px 14px;
		min-height: min(330px, calc(100vh - 200px));
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.split {
		display: flex;
		gap: 16px;
		align-items: flex-start;
	}
	.split fieldset {
		min-width: 190px;
	}
	.col {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		margin: 0;
		padding: 8px 12px;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	legend,
	.lbl {
		font-size: 12px;
		color: #4b5563;
	}
	.lbl {
		margin-top: 4px;
	}
	.muted {
		color: #6b7280;
	}
	.note,
	.hint {
		margin: 0;
		font-size: 12px;
		color: #4b5563;
	}
	.error {
		margin: 0;
		color: #b91c1c;
		font-size: 12px;
	}
	input[type='text'] {
		font: inherit;
		padding: 2px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}
	input.other {
		width: 28px;
		text-align: center;
	}
	.dest input {
		width: 120px;
	}
	.adv {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 6px 8px;
		border: 1px solid #e5e7eb;
	}
	button.plain {
		align-self: flex-start;
		font: inherit;
		padding: 3px 10px;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		cursor: pointer;
	}
	.pv {
		border: 1px solid #d4d7db;
		height: 150px;
		overflow: auto;
		background: #fff;
	}
	.mono {
		font: 12px Consolas, 'Cascadia Mono', 'Courier New', monospace;
		white-space: pre;
	}
	.line {
		padding: 0 6px;
		line-height: 18px;
	}
	.num {
		display: inline-block;
		width: 3ch;
		color: #9ca3af;
	}
	.head {
		font-weight: 600;
	}
	table {
		border-collapse: collapse;
		font-size: 12px;
	}
	th,
	td {
		border: 1px solid #e5e7eb;
		padding: 1px 8px;
		white-space: pre;
		text-align: left;
		max-width: 220px;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	th {
		background: #f3f4f6;
		font-weight: 600;
		cursor: pointer;
		position: sticky;
		top: 0;
	}
	td.on,
	th.on {
		background: #d3e7dc;
	}
	td.skip,
	th.skip {
		color: #9ca3af;
	}
	tr.head td {
		font-weight: 600;
	}
	.ruler {
		position: relative;
		cursor: crosshair;
		height: 170px;
	}
	.meter {
		position: absolute;
		visibility: hidden;
		pointer-events: none;
	}
	.scale {
		height: 16px;
		border-bottom: 1px solid #d4d7db;
		font-size: 10px;
		color: #6b7280;
		position: sticky;
		top: 0;
		background: #fafafa;
		z-index: 1;
	}
	.tick {
		position: absolute;
		top: 1px;
		transform: translateX(-1px);
		border-left: 1px solid #c9cdd2;
		padding-left: 2px;
	}
	.text {
		position: relative;
		padding: 2px 0;
		min-height: calc(100% - 17px);
	}
	.fline {
		padding: 0 6px;
		line-height: 18px;
	}
	.brk {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 9px;
		margin-left: -4px;
		cursor: ew-resize;
		z-index: 2;
	}
	.brk::before {
		content: '';
		position: absolute;
		left: 4px;
		top: 0;
		bottom: 0;
		border-left: 1px solid #1f2328;
	}
	.brk::after {
		content: '';
		position: absolute;
		left: 0;
		top: 0;
		border: 4px solid transparent;
		border-top: 6px solid #1f2328;
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
	.foot button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.foot button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.confirm {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: rgba(255, 255, 255, 0.6);
		border-radius: var(--float-radius);
	}
	.box {
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		max-width: 360px;
	}
	.box p {
		margin: 0;
		padding: 14px 14px 4px;
	}
</style>
