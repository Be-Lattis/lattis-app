<script lang="ts">
	// Boîte d'une formule par ligne : « Choisir par › Une formule… » d'un
	// onglet (catégorie calculée) et « Champ calculé… » d'un tableau croisé.
	// Un nom, une formule sur les colonnes de chaque ligne (suggestions de
	// fonctions et de [Colonne]), un aperçu des valeurs qu'elle donne (les dix
	// premières), l'erreur de la formule sous le champ.
	import { untrack } from 'svelte';
	import { backdrop } from './backdrop.js';
	import { modal } from './focus.js';
	import { apply as applySuggestion, type Completion } from '#lib/grid/autocomplete.js';
	import { guessCategoryName, previewText, rowSuggest, type RowFormulaHelp } from './rowformula.js';
	import { errorText, fnName, t } from '#lib/i18n/index.js';

	interface Preview {
		values: { text: string; count: number }[];
		count: number;
		tables?: string[];
		others?: string[];
		error?: string;
	}
	interface Props {
		title: string;
		/** Explication sous le titre. */
		intro: string;
		nameLabel?: string;
		name?: string;
		formula?: string;
		/** Le nom se devine d'après la formule tant qu'on ne l'a pas tapé. */
		guess?: boolean;
		help?: RowFormulaHelp;
		/** Mot des tableaux qui suivent (« Suivent », vide : pas de ligne des tableaux). */
		follow?: string;
		okLabel?: string;
		preview: (formula: string) => Promise<Preview>;
		/** Valide ; une exception est affichée sous la formule. */
		onok: (name: string, formula: string) => unknown;
		oncancel: () => void;
	}
	let { title, intro, nameLabel = t('shell.rowFormula.name'), name = '', formula = '', guess = false, help, follow = '', okLabel = t('common.ok'), preview, onok, oncancel }: Props = $props();

	// Valeurs de départ (la boîte est recréée à chaque ouverture).
	let draftName = $state(untrack(() => name));
	let draft = $state(untrack(() => formula || '='));
	let typedName = $state(untrack(() => !!name));
	let shown = $state<Preview | null>(null);
	let failed = $state('');
	let busy = $state(false);
	let completion = $state<Completion | null>(null);
	let pick = $state(0);
	let input = $state<HTMLInputElement | null>(null);

	// Aperçu : relu un instant après la dernière frappe.
	let timer: ReturnType<typeof setTimeout> | undefined;
	let asked = 0;
	$effect(() => {
		const f = draft;
		clearTimeout(timer);
		failed = '';
		if (guess && !typedName) draftName = guessCategoryName(f);
		if (!f.trim() || f.trim() === '=') {
			shown = null;
			return;
		}
		const n = ++asked;
		timer = setTimeout(() => {
			preview(f)
				.then((p) => n === asked && (shown = p))
				.catch((e) => n === asked && (shown = { values: [], count: 0, error: errorText(e) }));
		}, 250);
		return () => clearTimeout(timer);
	});

	function refresh() {
		if (!input) return;
		completion = rowSuggest(input.value, input.selectionStart ?? input.value.length, help);
		pick = 0;
	}
	function choose(i: number) {
		if (!completion || !input) return;
		const r = applySuggestion(draft, input.selectionStart ?? draft.length, completion.from, completion.items[i]);
		draft = r.text;
		completion = null;
		const el = input;
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(r.caret, r.caret);
			refresh();
		});
	}
	function formulaKeys(e: KeyboardEvent) {
		if (!completion) return;
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			e.stopPropagation();
			const n = completion.items.length;
			pick = (pick + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
		} else if (e.key === 'Tab' || (e.key === 'Enter' && completion.items.length)) {
			e.preventDefault();
			e.stopPropagation();
			choose(pick);
		} else if (e.key === 'Escape') {
			e.stopPropagation();
			completion = null;
		}
	}
	const ready = $derived(!!draftName.trim() && !!draft.trim() && draft.trim() !== '=' && !shown?.error && !busy);
	async function ok() {
		if (!ready) return;
		busy = true;
		try {
			await onok(draftName.trim(), draft.trim());
		} catch (e) {
			failed = errorText(e);
		} finally {
			busy = false;
		}
	}
	function keys(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') oncancel();
		else if (e.key === 'Enter') {
			e.preventDefault();
			ok();
		}
	}
</script>

<div class="rf-back" role="presentation" use:backdrop={oncancel}>
	<div class="rf" role="dialog" aria-modal="true" aria-label={title} tabindex="-1" use:modal onkeydown={keys}>
		<h2>{title}</h2>
		<p>{intro}</p>
		<label for="rf-formula">{t('shell.rowFormula.formula')}</label>
		<div class="field">
			<input
				id="rf-formula"
				bind:this={input}
				bind:value={draft}
				spellcheck="false"
				autocomplete="off"
				data-autofocus
				placeholder={`=${fnName('YEAR', 'ANNEE')}([Date])`}
				aria-invalid={!!(shown?.error || failed)}
				oninput={refresh}
				onclick={refresh}
				onkeydown={formulaKeys}
				onblur={() => setTimeout(() => (completion = null), 150)}
			/>
			{#if completion}
				<div class="rf-suggest" role="listbox" aria-label={t('shell.rowFormula.suggestions')}>
					{#each completion.items as s, i (s.kind + s.label)}
						<button role="option" aria-selected={i === pick} class:on={i === pick} tabindex="-1" onmousedown={(e) => e.preventDefault()} onclick={() => choose(i)}
							><span class="kind {s.kind}">{s.kind === 'column' ? '[ ]' : 'ƒ'}</span>{s.label}<span class="detail">{s.detail}</span></button
						>
					{/each}
				</div>
			{/if}
		</div>
		<div class="hint">{t('shell.rowFormula.hint')}</div>
		<label for="rf-name">{nameLabel}</label>
		<input id="rf-name" bind:value={draftName} oninput={() => (typedName = true)} />
		<div class="preview" aria-live="polite">
			{#if failed || shown?.error}
				<div class="error">{failed || shown?.error}</div>
			{:else if shown}
				<div><b>{t('shell.rowFormula.count', { n: shown.count })}</b> : {previewText(shown.values, shown.count)}</div>
				{#if follow && shown.tables?.length}
					<div class="tables">{t('shell.rowFormula.follow', { label: follow, list: shown.tables.join(', ') })}{#if shown.others?.length}{t('shell.rowFormula.others', { list: shown.others.join(', ') })}{/if}</div>
				{/if}
			{:else}
				<div class="muted">{t('shell.rowFormula.previewEmpty')}</div>
			{/if}
		</div>
		<div class="actions">
			<button class="primary" disabled={!ready} onclick={ok}>{okLabel}</button>
			<button onclick={oncancel}>{t('common.cancel')}</button>
		</div>
	</div>
</div>

<style>
	.rf-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.rf {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		width: min(460px, calc(100vw - 32px));
		display: flex;
		flex-direction: column;
		gap: 6px;
		outline: none;
	}
	h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	p {
		margin: 0 0 4px;
		line-height: 1.4;
	}
	input {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
		width: 100%;
		box-sizing: border-box;
	}
	#rf-formula {
		font-family: Consolas, 'Cascadia Mono', monospace;
	}
	input[aria-invalid='true'] {
		border-color: #c42b1c;
	}
	.field {
		position: relative;
	}
	.rf-suggest {
		position: absolute;
		left: 0;
		top: 100%;
		z-index: 1;
		min-width: 240px;
		max-width: 100%;
		background: #fff;
		border: 1px solid #d4d7db;
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		display: flex;
		flex-direction: column;
	}
	.rf-suggest button {
		font: inherit;
		text-align: left;
		border: 0;
		background: none;
		padding: 3px 8px;
		display: flex;
		gap: 6px;
		align-items: baseline;
		cursor: pointer;
		white-space: nowrap;
	}
	.rf-suggest button.on {
		background: #e1efe7;
	}
	.kind {
		width: 18px;
		color: #217346;
		font-size: 11px;
	}
	.detail {
		margin-left: auto;
		color: #6b7280;
		font-size: 11px;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 180px;
	}
	.hint,
	.muted,
	.tables {
		color: #6b7280;
		font-size: 12px;
	}
	.preview {
		min-height: 36px;
		margin-top: 4px;
		padding: 6px 8px;
		background: #f6f8fa;
		border-radius: 3px;
		line-height: 1.4;
		overflow-wrap: anywhere;
	}
	.error {
		color: #c42b1c;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 6px;
	}
	.actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	.actions button:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
