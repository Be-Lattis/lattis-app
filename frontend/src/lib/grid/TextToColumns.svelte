<script lang="ts">
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
				if (n === seq) error = String(e instanceof Error ? e.message : e);
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
			error = String(e instanceof Error ? e.message : e);
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

	const FORMATS: { value: ColFormat; label: string }[] = [
		{ value: '', label: 'Standard' },
		{ value: 'text', label: 'Texte' },
		{ value: 'date', label: 'Date' },
		{ value: 'skip', label: 'Colonne non distribuée' }
	];
	const QUOTES = [
		{ value: '"', label: '"' },
		{ value: "'", label: "'" },
		{ value: '', label: '{aucun}' }
	];
	const DECIMALS = [
		{ value: ',', label: ', (virgule)' },
		{ value: '.', label: '. (point)' }
	];
	const THOUSANDS = [
		{ value: ' ', label: 'espace' },
		{ value: '.', label: '. (point)' },
		{ value: ',', label: ', (virgule)' },
		{ value: "'", label: "' (apostrophe)" },
		{ value: '', label: 'aucun' }
	];
	const STEPS = ['Type des données', 'Séparateurs', 'Format des colonnes'];
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

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="back" use:backdrop={() => onclose()}>
	<div class="dlg" role="dialog" aria-label="Convertir le texte en colonnes" tabindex="-1" aria-modal="true" use:grab use:modal onkeydown={onKey}>
		<div class="head">Assistant Conversion – Étape {step} sur 3 : {STEPS[step - 1]}</div>
		<div class="body">
			{#if pv}
				<p class="note">
					{#if inTable}
						Tableau <b>{pv.table}</b>, colonne <b>{pv.column}</b>{pv.header ? ' (en-tête compris)' : ''} : la première partie reste dans la colonne, les suivantes deviennent de nouvelles colonnes insérées à droite.
					{:else}
						{pv.total} cellule{pv.total > 1 ? 's' : ''} de texte à convertir.
					{/if}
				</p>
			{/if}
			{#if step === 1}
				<fieldset>
					<legend>Choisissez le type de fichier qui décrit le mieux vos données</legend>
					<label class="row"><input type="radio" name="ttc-kind" checked={!fixed} onchange={() => (fixed = false)} /><b>Délimité</b><span class="muted">– Des caractères tels que des virgules ou des tabulations séparent chaque champ.</span></label>
					<label class="row"><input type="radio" name="ttc-kind" checked={fixed} onchange={() => {
						fixed = true;
						if (!breaks.length) breaks = suggestBreaks(lines);
					}} /><b>Largeur fixe</b><span class="muted">– Les champs sont alignés en colonnes, séparés par des espaces.</span></label>
				</fieldset>
				<div class="lbl">Aperçu des données sélectionnées</div>
				<div class="pv mono">
					{#each lines as l, i (i)}
						<div class="line" class:head={pv?.header && i === 0}><span class="num">{i + 1}</span>{shown(l)}</div>
					{/each}
				</div>
			{:else if step === 2 && !fixed}
				<div class="split">
					<fieldset>
						<legend>Séparateurs</legend>
						<label class="row"><input type="checkbox" bind:checked={delims.tab} />Tabulation</label>
						<label class="row"><input type="checkbox" bind:checked={delims.semi} />Point-virgule</label>
						<label class="row"><input type="checkbox" bind:checked={delims.comma} />Virgule</label>
						<label class="row"><input type="checkbox" bind:checked={delims.space} />Espace</label>
						<label class="row"><input type="checkbox" bind:checked={delims.other} />Autre :
							<input class="other" type="text" maxlength="1" bind:value={delims.otherChar} oninput={() => (delims.other = delims.otherChar !== '')} aria-label="Autre séparateur" /></label>
					</fieldset>
					<div class="col">
						<label class="row"><input type="checkbox" bind:checked={merge} />Interpréter des séparateurs identiques consécutifs comme uniques</label>
						<div class="row">Identificateur de texte : <Select bind:value={quote} options={QUOTES} width="90px" /></div>
					</div>
				</div>
				<div class="lbl">Aperçu de données</div>
				{@render table(false)}
			{:else if step === 2}
				<p class="hint">
					Les lignes avec flèche signalent un saut de colonne. Pour créer un saut de colonne, cliquez à la position désirée.
					Pour effacer un saut de colonne, double-cliquez sur la ligne. Pour déplacer un saut de colonne, faites-le glisser.
				</p>
				<div class="lbl">Aperçu de données</div>
				<!-- svelte-ignore a11y_click_events_have_key_events -->
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
								<!-- svelte-ignore a11y_no_static_element_interactions -->
								<div class="brk" style:left="{PAD + b * chPx}px" onmousedown={(e) => startDrag(e, i)} ondblclick={(e) => removeBreak(e, i)} onclick={(e) => e.stopPropagation()} title="Glisser pour déplacer, double-clic pour supprimer"></div>
							{/if}
						{/each}
					</div>
				</div>
			{:else}
				<div class="split">
					<fieldset>
						<legend>Format des données en colonne {selCol + 1}</legend>
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
						<p class="hint">« Standard » convertit les nombres et les dates, le reste devient du texte. Cliquez sur une colonne de l'aperçu pour choisir son format.</p>
						<label class="row dest">Destination :
							<input type="text" bind:value={dest} disabled={inTable} title={inTable ? 'Dans un tableau, la première partie reste dans la colonne et les suivantes sont insérées à droite' : 'Première cellule où écrire les colonnes, par exemple $B$2'} />
						</label>
						<button class="plain" onclick={() => (advanced = !advanced)}>{advanced ? 'Avancé ▴' : 'Avancé…'}</button>
						{#if advanced}
							<div class="adv">
								<div class="row">Séparateur décimal : <Select bind:value={decimal} options={DECIMALS} width="120px" /></div>
								<div class="row">Séparateur des milliers : <Select bind:value={thousands} options={THOUSANDS} width="120px" /></div>
							</div>
						{/if}
					</div>
				</div>
				<div class="lbl">Aperçu de données</div>
				{@render table(true)}
			{/if}
			{#if error}<p class="error">{error}</p>{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button onclick={onclose}>Annuler</button>
			<button disabled={step === 1} onclick={() => step--}>&lt; Précédent</button>
			<button disabled={step === 3} onclick={() => step++}>Suivant &gt;</button>
			<button class="primary" disabled={busy || !pv} onclick={() => finish()}>Terminer</button>
		</div>
		{#if confirm}
			<div class="confirm" role="alertdialog" aria-label="Remplacer le contenu">
				<div class="box">
					<p>Voulez-vous remplacer le contenu des cellules de destination ?</p>
					<div class="foot">
						<span class="sp"></span>
						<button class="primary" onclick={() => {
							confirm = false;
							finish(true);
						}}>OK</button>
						<button onclick={() => (confirm = false)}>Annuler</button>
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
