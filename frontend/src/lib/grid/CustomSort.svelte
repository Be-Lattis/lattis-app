<script lang="ts">
	// Tri personnalisé… (menus ▾ d'une colonne de tableau ou d'un champ de
	// TCD, Données › Trier et filtrer) : les valeurs de la colonne à ranger
	// dans l'ordre voulu, par glisser-déposer, boutons ou clavier, ou d'après
	// une liste personnalisée (Jours, Mois, listes du classeur), comme les
	// listes personnalisées d'Excel. Les valeurs hors de l'ordre et les vides
	// viennent toujours après.
	import { onMount } from 'svelte';
	import { modal } from '#lib/ui/focus.js';
	import { cleanValues, fold, moveItem, orderItems, orderOf, suggestList, type CustomList, type CustomOrder, type OrderItem } from './customsort';

	interface Props {
		/** Nom de la colonne ou du champ. */
		field: string;
		/** Valeurs distinctes des données (le vide compris ou non). */
		loadValues: () => Promise<string[]>;
		/** Ordre enregistré de la colonne ou du champ (null : aucun). */
		loadOrder?: () => Promise<CustomOrder | null>;
		loadLists?: () => Promise<CustomList[]>;
		/** Remplace les listes personnalisées du classeur. */
		saveLists?: (lists: CustomList[]) => Promise<unknown>;
		onapply: (o: CustomOrder) => void;
		onclose: () => void;
	}

	let { field, loadValues, loadOrder, loadLists, saveLists, onapply, onclose }: Props = $props();

	let values = $state<string[]>([]);
	let lists = $state<CustomList[]>([]);
	let items = $state<OrderItem[]>([]);
	/** Liste nommée choisie (son nom), '' : ordre libre. */
	let listName = $state('');
	let desc = $state(false);
	let sel = $state(0);
	let loading = $state(true);
	let error = $state('');
	let saveAs = $state(false);
	let newName = $state('');
	let busy = $state(false);
	let dragFrom = $state<number | null>(null);
	let dragOver = $state<number | null>(null);
	let box = $state<HTMLElement>();

	const chosen = $derived(lists.find((l) => l.name === listName) ?? null);

	onMount(() => {
		Promise.all([loadValues(), loadOrder?.() ?? Promise.resolve(null), loadLists?.() ?? Promise.resolve([])]).then(
			([vals, order, ls]) => {
				values = vals;
				lists = ls;
				if (order) {
					items = orderItems(vals, order.values);
					desc = !!order.desc;
					listName = order.list && ls.some((l) => l.name === order.list) ? order.list : '';
				} else {
					const s = suggestList(vals, ls);
					listName = s?.name ?? '';
					items = orderItems(vals, s?.values);
				}
				loading = false;
			},
			(err) => {
				error = String(err instanceof Error ? err.message : err);
				loading = false;
			}
		);
	});

	/** Reprend l'ordre d'une liste (vide : les valeurs dans leur ordre naturel). */
	function chooseList(name: string) {
		listName = name;
		const l = lists.find((x) => x.name === name);
		items = orderItems(values, l?.values);
		sel = 0;
	}

	function move(from: number, to: number) {
		if (from === to || from < 0 || from >= items.length) return;
		items = moveItem(items, from, to);
		sel = Math.max(0, Math.min(items.length - 1, to));
		// L'ordre ne suit plus la liste : il devient libre.
		if (chosen && orderOf(items, chosen, false).list === undefined) listName = '';
		queueMicrotask(() => box?.querySelector<HTMLElement>(`[data-i="${sel}"]`)?.scrollIntoView({ block: 'nearest' }));
	}

	function onKey(e: KeyboardEvent) {
		const n = items.length;
		if (!n) return;
		const shift = e.altKey || e.ctrlKey;
		let to = -1;
		if (e.key === 'ArrowUp') to = sel - 1;
		else if (e.key === 'ArrowDown') to = sel + 1;
		else if (e.key === 'Home') to = 0;
		else if (e.key === 'End') to = n - 1;
		else if (e.key === 'PageUp') to = sel - 10;
		else if (e.key === 'PageDown') to = sel + 10;
		else return;
		e.preventDefault();
		e.stopPropagation();
		to = Math.max(0, Math.min(n - 1, to));
		if (shift) move(sel, to);
		else {
			sel = to;
			box?.querySelector<HTMLElement>(`[data-i="${sel}"]`)?.scrollIntoView({ block: 'nearest' });
		}
	}

	async function ok() {
		error = '';
		if (!items.length) {
			error = 'Aucune valeur à ranger';
			return;
		}
		let order = orderOf(items, chosen, desc);
		if (saveAs && saveLists) {
			const name = newName.trim();
			if (!name) {
				error = 'Nommez la liste à enregistrer';
				return;
			}
			if (lists.some((l) => fold(l.name) === fold(name))) {
				error = `La liste « ${name} » existe déjà`;
				return;
			}
			const vals = cleanValues(items.map((i) => i.text));
			busy = true;
			try {
				await saveLists([...lists.filter((l) => !l.builtin), { name, values: vals }]);
			} catch (err) {
				error = String(err instanceof Error ? err.message : err);
				busy = false;
				return;
			}
			busy = false;
			order = { list: name, values: vals, ...(desc ? { desc } : {}) };
		}
		onapply(order);
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="back" onmousedown={(e) => e.target === e.currentTarget && onclose()}>
	<div
		class="dlg"
		role="dialog"
		aria-label="Tri personnalisé"
		tabindex="-1"
		aria-modal="true"
		use:modal={onclose}
		onkeydown={(e) => {
			if (e.key === 'Enter' && !(e.target instanceof HTMLButtonElement) && !(e.target instanceof HTMLSelectElement)) {
				e.preventDefault();
				ok();
			}
		}}
	>
		<div class="head">Tri personnalisé – « {field} »</div>
		<div class="body">
			{#if loading}
				<p class="note"><span class="spinner" aria-hidden="true"></span> Chargement des valeurs…</p>
			{:else}
				<label class="row">
					Ordre d'après :
					<select value={listName} onchange={(e) => chooseList((e.currentTarget as HTMLSelectElement).value)}>
						<option value="">les valeurs de la colonne (ordre libre)</option>
						{#each lists as l (l.name)}
							<option value={l.name}>Liste « {l.name} »{l.builtin ? '' : ' (classeur)'} : {l.values.slice(0, 3).join(', ')}…</option>
						{/each}
					</select>
				</label>
				<p class="hint">Glissez les valeurs pour les ranger, ou sélectionnez-en une et utilisez les boutons (au clavier : Alt+↑ / Alt+↓, Alt+Début / Alt+Fin). Les valeurs absentes de l'ordre suivent dans l'ordre croissant, les cellules vides restent à la fin.</p>
				<div class="split">
					<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
					<ul
						class="items"
						role="listbox"
						tabindex="0"
						aria-label="Ordre des valeurs"
						aria-activedescendant={items.length ? `cs-item-${sel}` : undefined}
						bind:this={box}
						onkeydown={onKey}
						data-autofocus
					>
						{#each items as it, i (fold(it.text))}
							<li
								id="cs-item-{i}"
								data-i={i}
								role="option"
								aria-selected={i === sel}
								class:on={i === sel}
								class:absent={it.absent}
								class:over={dragOver === i && dragFrom !== null && dragFrom !== i}
								class:dragging={dragFrom === i}
								draggable="true"
								onmousedown={() => (sel = i)}
								ondragstart={(e) => {
									dragFrom = i;
									sel = i;
									e.dataTransfer?.setData('text/plain', it.text);
									if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move';
								}}
								ondragover={(e) => {
									if (dragFrom === null) return;
									e.preventDefault();
									dragOver = i;
								}}
								ondrop={(e) => {
									e.preventDefault();
									if (dragFrom !== null) move(dragFrom, i);
									dragFrom = dragOver = null;
								}}
								ondragend={() => (dragFrom = dragOver = null)}
							>
								<span class="grip" aria-hidden="true">⠿</span>
								<span class="num">{i + 1}</span>
								<span class="what">{it.text}</span>
								{#if it.absent}<span class="tag" title="Valeur de l'ordre absente des données : gardée pour quand elle apparaîtra">absente</span>{/if}
							</li>
						{/each}
						{#if !items.length}<li class="empty">Aucune valeur (la colonne est vide).</li>{/if}
					</ul>
					<div class="moves">
						<button disabled={sel <= 0} onclick={() => move(sel, 0)} title="Placer en premier (Alt+Début)">En premier</button>
						<button disabled={sel <= 0} onclick={() => move(sel, sel - 1)} title="Monter (Alt+↑)">Monter</button>
						<button disabled={sel >= items.length - 1} onclick={() => move(sel, sel + 1)} title="Descendre (Alt+↓)">Descendre</button>
						<button disabled={sel >= items.length - 1} onclick={() => move(sel, items.length - 1)} title="Placer en dernier (Alt+Fin)">En dernier</button>
					</div>
				</div>
				<fieldset>
					<legend>Sens</legend>
					<label class="row"><input type="radio" name="cs-dir" checked={!desc} onchange={() => (desc = false)} />Dans cet ordre (du premier au dernier)</label>
					<label class="row"><input type="radio" name="cs-dir" checked={desc} onchange={() => (desc = true)} />À rebours (du dernier au premier)</label>
				</fieldset>
				{#if saveLists}
					<label class="row"><input type="checkbox" bind:checked={saveAs} />Enregistrer cet ordre comme liste personnalisée du classeur, pour d'autres colonnes</label>
					{#if saveAs}
						<label class="row indent">Nom : <input type="text" bind:value={newName} placeholder="Tailles, Priorités…" /></label>
					{/if}
				{/if}
			{/if}
			{#if error}<p class="error">{error}</p>{/if}
		</div>
		<div class="foot">
			<span class="sp"></span>
			<button class="primary" disabled={loading || busy || !items.length} onclick={ok}>OK</button>
			<button onclick={onclose}>Annuler</button>
		</div>
	</div>
</div>

<style>
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
		max-height: calc(100vh - 64px);
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		width: 520px;
		max-width: calc(100% - 24px);
		outline: none;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
	}
	.head {
		padding: 8px 14px;
		font-weight: 600;
		border-bottom: 1px solid #d4d7db;
		flex-shrink: 0;
	}
	.body {
		padding: 10px 14px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-height: 0;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.row select {
		flex: 1;
		min-width: 0;
	}
	.indent {
		padding-left: 22px;
	}
	.indent input {
		flex: 1;
	}
	.hint,
	.note {
		margin: 0;
		color: #57606a;
		font-size: 12px;
	}
	.split {
		display: flex;
		gap: 10px;
		align-items: flex-start;
		min-height: 0;
	}
	.items {
		flex: 1;
		list-style: none;
		margin: 0;
		padding: 2px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		height: 260px;
		max-height: 40vh;
		overflow: auto;
		outline: none;
	}
	.items:focus-visible {
		border-color: #217346;
		box-shadow: 0 0 0 1px #217346;
	}
	.items li {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 6px;
		border-radius: 2px;
		cursor: grab;
		user-select: none;
		border-top: 2px solid transparent;
	}
	.items li.on {
		background: #e1efe6;
	}
	.items:focus .on {
		background: #cfe6d7;
	}
	.items li.over {
		border-top-color: #217346;
	}
	.items li.dragging {
		opacity: 0.5;
	}
	.items li.absent .what {
		color: #8c959f;
		font-style: italic;
	}
	.items li.empty {
		color: #8c959f;
		cursor: default;
	}
	.grip {
		color: #8c959f;
	}
	.num {
		color: #8c959f;
		min-width: 2.2em;
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.what {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tag {
		font-size: 11px;
		color: #8c959f;
		border: 1px solid #d0d7de;
		border-radius: 8px;
		padding: 0 6px;
	}
	.moves {
		display: flex;
		flex-direction: column;
		gap: 6px;
		width: 110px;
	}
	fieldset {
		border: 1px solid #e5e7eb;
		margin: 0;
		padding: 6px 12px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.error {
		margin: 0;
		color: #c00000;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 8px 14px;
		border-top: 1px solid #d4d7db;
		flex-shrink: 0;
	}
	.sp {
		flex: 1;
	}
	button {
		padding: 4px 14px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: #fff;
		font: inherit;
		cursor: pointer;
	}
	button:hover:not(:disabled) {
		background: #f3f4f6;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	button.primary:hover:not(:disabled) {
		background: #1b5e39;
	}
	.spinner {
		display: inline-block;
		width: 10px;
		height: 10px;
		border: 2px solid #c9cdd2;
		border-top-color: #217346;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
