<!--
	Champ d'une formule dans une boîte de dialogue (validation des données) :
	la même aide que l'éditeur de cellule et la barre de formule
	(autocomplete.ts) : fonctions, tableaux, colonnes, noms, signature de la
	fonction en cours. Flèches, Tab ou Entrée reprennent une suggestion,
	Échap la ferme. Rien n'est proposé tant que le texte ne commence pas par =.
-->
<script lang="ts">
	import { apply as applySuggestion, signature, signatureParts, suggest, type Catalog, type Completion, type FunctionDoc } from './autocomplete';
	import { t as tr } from '#lib/i18n/index.js';

	interface Props {
		id: string;
		value: string;
		/** Catalogue des fonctions, tableaux et noms ; absent : simple champ. */
		catalog?: Catalog;
		/** Tableau de la cellule : [Colonne] y est la valeur de la ligne. */
		table?: string;
		placeholder?: string;
		oninput?: () => void;
	}
	let { id, value = $bindable(), catalog, table, placeholder, oninput }: Props = $props();

	let input = $state<HTMLInputElement | null>(null);
	let completion = $state<(Completion & { sel: number }) | null>(null);
	let sig = $state<{ fn: FunctionDoc; index: number } | null>(null);

	function refresh() {
		if (!input || !catalog) return;
		const caret = input.selectionStart ?? input.value.length;
		const c = suggest(input.value, caret, catalog, table);
		completion = c ? { ...c, sel: 0 } : null;
		sig = signature(input.value, caret, catalog);
	}

	function choose(i: number) {
		const c = completion;
		const el = input;
		if (!c || !el) return;
		const r = applySuggestion(value, el.selectionStart ?? value.length, c.from, c.items[i]);
		value = r.text;
		completion = null;
		oninput?.();
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(r.caret, r.caret);
			refresh();
		});
	}

	function keys(e: KeyboardEvent) {
		const c = completion;
		if (!c) {
			if (e.key === 'Escape' && sig) {
				e.stopPropagation();
				sig = null;
			}
			return;
		}
		const n = c.items.length;
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') completion = { ...c, sel: (c.sel + (e.key === 'ArrowDown' ? 1 : n - 1)) % n };
		else if (e.key === 'Tab' || e.key === 'Enter') choose(c.sel);
		else if (e.key === 'Escape') completion = null;
		else return;
		e.preventDefault();
		e.stopPropagation();
	}

	const sp = $derived(sig ? signatureParts(sig.fn, sig.index) : null);
</script>

<div class="ff">
	<input
		{id}
		type="text"
		spellcheck="false"
		autocomplete="off"
		{placeholder}
		bind:this={input}
		bind:value
		aria-autocomplete={catalog ? 'list' : undefined}
		oninput={() => {
			oninput?.();
			refresh();
		}}
		onclick={refresh}
		onkeydown={keys}
		onkeyup={(e) => ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key) && refresh()}
		onblur={() =>
			setTimeout(() => {
				completion = null;
				sig = null;
			}, 150)}
	/>
	{#if (sig && sp) || completion}
		<div class="ff-pop">
			{#if sig && sp}
				<div class="sig">
					<div class="sig-line">{#each sp.parts as p, i (i)}{#if p.on}<b>{p.text}</b>{:else}{p.text}{/if}{/each}</div>
					{#if sig.fn.description}<div class="sig-desc">{sig.fn.description}</div>{/if}
					{#if sp.arg}<div class="sig-arg"><b>{sp.arg.name}</b>{sp.arg.optional ? ' ' + tr('grid.sig.optional') : ''} : {sp.arg.desc}</div>{/if}
				</div>
			{/if}
			{#if completion}
				<div role="listbox" aria-label={tr('shell.rowFormula.suggestions')}>
					{#each completion.items as s, i (s.kind + s.label)}
						<button role="option" aria-selected={i === completion.sel} class:on={i === completion.sel} tabindex="-1" onmousedown={(e) => e.preventDefault()} onclick={() => choose(i)}>
							<span class="kind">{s.kind === 'function' ? 'fx' : s.kind === 'table' ? '▦' : s.kind === 'name' ? 'N' : '[ ]'}</span>
							<span class="lab">{s.label}</span>
							<span class="det" title={s.detail}>{s.detail}</span>
						</button>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.ff {
		position: relative;
		flex: 1;
		min-width: 0;
		display: flex;
	}

	input {
		flex: 1;
		min-width: 0;
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
	}

	.ff-pop {
		position: absolute;
		left: 0;
		top: 100%;
		z-index: 2;
		margin-top: 2px;
		min-width: 100%;
		max-width: 420px;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		font-size: 12px;
	}

	.sig {
		padding: 5px 10px;
		background: #f7f8f9;
		color: #374151;
		border-bottom: 1px solid #e3e5e8;
		line-height: 1.4;
	}

	.sig:last-child {
		border-bottom: none;
	}

	.sig-line {
		font-family: Consolas, 'Courier New', monospace;
		color: #111827;
	}

	.sig-line b {
		color: #14532d;
	}

	.sig-desc,
	.sig-arg {
		margin-top: 2px;
		color: #4b5563;
	}

	button {
		display: flex;
		gap: 8px;
		align-items: center;
		width: 100%;
		font: inherit;
		text-align: left;
		border: 0;
		border-radius: 0;
		background: none;
		padding: 3px 10px;
		cursor: pointer;
	}

	button.on {
		background: #e3efe7;
	}

	.kind {
		width: 22px;
		flex: none;
		color: #217346;
		font-weight: 600;
		text-align: center;
	}

	.lab {
		flex: 1;
		white-space: nowrap;
	}

	.det {
		color: #6b7280;
		max-width: 200px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
