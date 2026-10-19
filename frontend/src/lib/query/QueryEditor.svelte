<script lang="ts">
	// Éditeur de requête, comme l'éditeur Power Query d'Excel : un ruban
	// d'étapes en haut, l'aperçu au centre (sur les 1 000 premières lignes de
	// chaque source), les étapes appliquées à droite. Cliquer une étape montre
	// l'aperçu à cette étape ; une nouvelle étape s'insère après celle qui est
	// choisie. « Fermer et charger » enregistre la requête et charge son
	// résultat dans un tableau de feuille.
	import { modal } from '#lib/ui/focus.js';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Column, PasswordNeed, Preview, Query, Source, Step } from '#bindings/internal/query/models.js';
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';
	import { dismiss } from '#lib/grid/dismiss.js';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { typeIcon, typeIconName, type ColumnType } from '#lib/grid/format.js';
	import StepForm from './StepForm.svelte';
	import SourceForm from './SourceForm.svelte';
	import type { ConnKind } from './sources';
	import { OPS, TYPES, clickColumns, clone, defaultStep, insertStep, moveStep, newQuery, otherSource, quickSteps, sourceCancel, typeName, withColumns, type Op, type QuickAction } from './steps';

	interface Props {
		query: Query;
		/** Ouvrir d'abord la boîte de la source (nouvelle requête à préciser). */
		editSource?: boolean;
		queries: string[];
		tables: string[];
		askPassword: (need: PasswordNeed) => Promise<boolean>;
		newConnection: (kind: ConnKind) => Promise<string | null>;
		/** Enregistrer (et charger si load) ; renvoie un message d'erreur ou "". */
		onfinish: (q: Query, load: boolean) => Promise<string>;
		onclose: () => void;
	}
	let { query, editSource = false, queries, tables, askPassword, newConnection, onfinish, onclose }: Props = $props();

	const initial = (): Query => ({ ...clone(query), steps: clone(query.steps ?? []) });
	let q = $state<Query>(initial());
	// Étape affichée : -1 pour la source, sinon le rang de l'étape.
	let sel = $state(initial().steps!.length - 1);
	let preview = $state<Preview | null>(null);
	let loading = $state(false);
	// Colonnes choisies dans l'aperçu (clic, Ctrl, Maj sur les en-têtes).
	let selCols = $state<string[]>([]);
	let anchor = '';
	const shown = $derived(preview?.columns ?? []);
	const chosen = $derived(shown.map((c) => c.name).filter((n) => selCols.includes(n)));
	const onCol = $derived(shown.map((c) => selCols.includes(c.name)));
	// Menu d'une colonne (▾ de l'en-tête ou clic droit).
	let menu = $state<{ x: number; y: number; col: string } | null>(null);
	let form = $state<{ step: Step; cols: Column[]; edit: number | null } | null>(null);
	const initialSource = () => editSource;
	let sourceOpen = $state(initialSource());
	// Nouvelle requête dont la source n'est pas encore validée : « Annuler »
	// dans la boîte de la source annule toute la création (rien n'est enregistré).
	let creating = $state(initialSource());
	let error = $state('');
	let busy = $state(false);
	let showSQL = $state(false);
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	const otherQueries = $derived(queries.filter((n) => n.toLowerCase() !== q.name.toLowerCase()));
	const others = $derived([...otherQueries.map((n) => ({ key: 'query:' + n, label: 'Requête ' + n })), ...tables.map((n) => ({ key: 'table:' + n, label: 'Tableau ' + n }))]);

	let seq = 0;
	async function refresh(fresh = false) {
		const n = ++seq;
		loading = true;
		try {
			let pv = await DocumentService.PreviewQuery(q, sel + 1, fresh);
			if (pv.needPassword && (await askPassword(pv.needPassword))) pv = await DocumentService.PreviewQuery(q, sel + 1, fresh);
			if (n === seq) preview = pv;
		} catch (e) {
			if (n === seq) error = err(e);
		} finally {
			if (n === seq) loading = false;
		}
	}
	onMount(() => {
		if (!editSource) refresh();
	});

	function select(i: number) {
		sel = i;
		refresh();
	}

	async function columnsAt(i: number): Promise<Column[]> {
		if (i === sel + 1 && preview && preview.errorStep < 0) return preview.columns ?? [];
		const pv = await DocumentService.PreviewQuery(q, i, false);
		return pv.columns ?? [];
	}

	// Annuler / rétablir les changements d'étapes (Ctrl+Z, Ctrl+Y), comme
	// l'éditeur de Power Query : chaque état des étapes et l'étape affichée.
	type Snap = { steps: Step[]; sel: number };
	let undoStack = $state<Snap[]>([]);
	let redoStack = $state<Snap[]>([]);
	const canUndo = $derived(undoStack.length > 0);
	const canRedo = $derived(redoStack.length > 0);
	const snap = (): Snap => ({ steps: clone(q.steps ?? []), sel });
	/** À appeler avant tout changement des étapes. */
	function remember() {
		undoStack.push(snap());
		if (undoStack.length > 100) undoStack.shift();
		redoStack = [];
	}
	function restore(from: Snap[], to: Snap[]) {
		const s = from.pop();
		if (!s) return;
		to.push(snap());
		q.steps = s.steps;
		sel = Math.min(s.sel, s.steps.length - 1);
		form = null;
		menu = null;
		refresh();
	}
	const undoStep = () => restore(undoStack, redoStack);
	const redoStep = () => restore(redoStack, undoStack);

	function onKey(ev: KeyboardEvent) {
		const t = ev.target as HTMLElement | null;
		// Dans un champ de saisie, Ctrl+Z reste celui du champ.
		if (t?.closest('input, textarea, select, [contenteditable="true"]') || form || sourceOpen) return;
		if (!(ev.ctrlKey || ev.metaKey) || ev.altKey) return;
		const k = ev.key.toLowerCase();
		if (k === 'z' && !ev.shiftKey) undoStep();
		else if (k === 'y' || (k === 'z' && ev.shiftKey)) redoStep();
		else return;
		ev.preventDefault();
	}

	async function addStep(op: Op) {
		error = '';
		menu = null;
		const cols = await columnsAt(sel + 1);
		const picked = chosen.filter((n) => cols.some((c) => c.name === n));
		form = { step: withColumns(defaultStep(op, cols, picked[0]), picked), cols, edit: null };
	}

	/** Étapes ajoutées sans boîte par le menu d'une colonne ou le ruban (colonnes choisies). */
	function quick(a: QuickAction, arg = '') {
		menu = null;
		error = '';
		const add = quickSteps(a, shown, chosen, arg);
		if (!add.length) return;
		remember();
		let steps = q.steps ?? [];
		for (const s of add) steps = insertStep(steps, sel++, s);
		q.steps = steps;
		refresh();
	}

	/** Clic sur un en-tête de l'aperçu : choisir la colonne (Ctrl, Maj : plusieurs). */
	function clickHeader(ev: MouseEvent, name: string) {
		const r = clickColumns(
			shown.map((c) => c.name),
			chosen,
			anchor,
			name,
			{ ctrl: ev.ctrlKey || ev.metaKey, shift: ev.shiftKey }
		);
		selCols = r.selected;
		anchor = r.anchor;
	}

	/** Clic dans une cellule de l'aperçu : choisit sa colonne (Ctrl, Maj : plusieurs), sans la retirer d'un second clic. */
	function clickCell(ev: MouseEvent, name: string) {
		if (!ev.ctrlKey && !ev.metaKey && !ev.shiftKey) {
			selCols = [name];
			anchor = name;
			return;
		}
		clickHeader(ev, name);
	}

	/** Menu de la colonne : la colonne rejoint le choix si elle n'y est pas. */
	function openMenu(name: string, x: number, y: number) {
		if (!chosen.includes(name)) {
			selCols = [name];
			anchor = name;
		}
		menu = { x, y, col: name };
	}

	function onHeaderMenu(ev: MouseEvent, name: string) {
		ev.preventDefault();
		openMenu(name, ev.clientX, ev.clientY);
	}

	// Le clic sur le ▾ d'un menu ouvert le ferme d'abord (clic ailleurs) :
	// il ne doit pas le rouvrir aussitôt (le ▾ ouvre et ferme, comme Excel).
	let closed = { col: '', t: 0 };
	function closeMenu() {
		if (menu) closed = { col: menu.col, t: performance.now() };
		menu = null;
	}

	function onDrop(ev: MouseEvent, name: string) {
		ev.stopPropagation();
		if (menu?.col === name) return void (menu = null);
		if (closed.col === name && performance.now() - closed.t < 400) return;
		const th = (ev.currentTarget as HTMLElement).closest('th')!.getBoundingClientRect();
		openMenu(name, th.left, th.bottom);
	}

	/** Garde un menu flottant dans la fenêtre. */
	function inView(node: HTMLElement) {
		const r = node.getBoundingClientRect();
		if (r.right > window.innerWidth - 4) node.style.left = `${Math.max(4, window.innerWidth - r.width - 4)}px`;
		if (r.bottom > window.innerHeight - 4) node.style.top = `${Math.max(4, window.innerHeight - r.height - 4)}px`;
	}

	const typeOf = (t: string) => (t || 'text') as ColumnType;

	async function editStep(i: number) {
		const cols = await columnsAt(i);
		form = { step: clone(q.steps![i]), cols, edit: i };
	}

	function applyForm(s: Step) {
		const f = form!;
		form = null;
		remember();
		if (f.edit !== null) {
			q.steps = q.steps!.map((x, k) => (k === f.edit ? s : x));
			sel = f.edit;
		} else {
			q.steps = insertStep(q.steps ?? [], sel, s);
			sel = sel + 1;
		}
		refresh();
	}

	function removeStep(i: number) {
		remember();
		q.steps = (q.steps ?? []).filter((_, k) => k !== i);
		if (sel >= i) sel = Math.max(-1, sel - 1);
		refresh();
	}

	function move(i: number, d: number) {
		remember();
		q.steps = moveStep(q.steps ?? [], i, d);
		if (sel === i) sel = i + d;
		refresh();
	}

	function setSource(s: Source) {
		sourceOpen = false;
		creating = false;
		q.source = s;
		refresh(true);
	}

	async function otherColumns(key: string): Promise<Column[]> {
		const pv = await DocumentService.PreviewQuery(newQuery('__autre', otherSource(key)), -1, false);
		if (pv.error) throw new Error(pv.error);
		return pv.columns ?? [];
	}

	async function finish(load: boolean) {
		error = '';
		if (!q.name.trim()) return void (error = 'Donnez un nom à la requête.');
		busy = true;
		try {
			const msg = await onfinish(clone(q), load);
			if (msg) error = msg;
		} finally {
			busy = false;
		}
	}

	const labels = $derived(preview?.labels ?? ['Source', ...(q.steps ?? []).map((s) => s.op)]);

	// Ruban, comme celui de Power Query : onglets Accueil, Transformer et
	// Ajouter une colonne ; dans chaque groupe, de grands boutons puis une
	// pile de petits.
	interface Cmd {
		label: string;
		icon: IconName;
		run: () => void;
		hint?: string;
		off?: boolean;
		on?: boolean;
	}
	interface Group {
		title: string;
		big: Cmd[];
		small?: Cmd[];
	}
	const TABS = ['Accueil', 'Transformer', 'Ajouter une colonne'] as const;
	let tab = $state<(typeof TABS)[number]>('Accueil');
	const op = (o: Op, icon: IconName, label?: string): Cmd => {
		const info = OPS.find((x) => x.op === o)!;
		return { label: label ?? info.label, icon, hint: info.hint, run: () => addStep(o) };
	};
	const none = $derived(!chosen.length);
	const ribbon = $derived<Record<(typeof TABS)[number], Group[]>>({
		Accueil: [
			{
				title: 'Fermer',
				big: [{ label: 'Fermer et charger', icon: 'close-load', hint: 'Enregistrer la requête et charger son résultat dans un tableau', off: busy, run: () => finish(true) }],
				small: [
					{ label: 'Enregistrer', icon: 'save', hint: 'Enregistrer la requête sans la charger (connexion seulement)', off: busy, run: () => finish(false) },
					{ label: 'Annuler', icon: 'win-close', hint: 'Fermer l’éditeur de requête sans enregistrer les changements (le classeur reste ouvert)', run: onclose }
				]
			},
			{
				title: 'Requête',
				big: [{ label: 'Actualiser l’aperçu', icon: 'refresh', hint: 'Relire les sources', run: () => refresh(true) }],
				small: [
					{ label: 'Source…', icon: 'database', hint: 'Modifier la source de la requête', run: () => (sourceOpen = true) },
					{ label: 'Voir le SQL', icon: 'script', hint: 'Afficher le SQL de l’aperçu', on: showSQL, run: () => (showSQL = !showSQL) },
					{ label: 'Défaire', icon: 'undo', hint: 'Retirer la dernière action sur les étapes (Ctrl+Z)', off: !canUndo, run: undoStep },
					{ label: 'Rétablir', icon: 'redo', hint: 'Remettre l’action défaite (Ctrl+Y)', off: !canRedo, run: redoStep }
				]
			},
			{ title: 'Gérer les colonnes', big: [op('select', 'choose-columns'), op('remove', 'delete-column')] },
			{ title: 'Réduire les lignes', big: [op('top', 'keep-rows'), op('distinct', 'remove-duplicates'), op('filter', 'filter')] },
			{
				title: 'Trier',
				big: [op('sort', 'sort', 'Trier…')],
				small: [
					{ label: 'Croissant', icon: 'sort-asc', hint: 'Trier par les colonnes choisies, croissant', off: none, run: () => quick('sortAsc') },
					{ label: 'Décroissant', icon: 'sort-desc', hint: 'Trier par les colonnes choisies, décroissant', off: none, run: () => quick('sortDesc') }
				]
			},
			{ title: 'Combiner', big: [op('merge', 'merge-queries', 'Fusionner des requêtes'), op('append', 'append-queries', 'Ajouter des requêtes')] }
		],
		Transformer: [
			{ title: 'Tableau', big: [op('group', 'group-by')] },
			{
				title: 'Toute colonne',
				big: [op('type', 'convert-type')],
				small: [op('rename', 'rename'), op('replace', 'replace'), op('unpivot', 'unpivot', 'Dépivoter les colonnes'), op('pivot', 'pivot-column', 'Pivoter la colonne')]
			},
			{ title: 'Colonne de texte', big: [op('split', 'split-column')], small: [op('transform', 'text-format')] }
		],
		'Ajouter une colonne': [{ title: 'Général', big: [op('addcolumn', 'custom-column'), op('index', 'index-column')] }]
	});
</script>

<svelte:window onkeydown={onKey} />
<div class="editor" role="dialog" aria-modal="true" aria-label="Éditeur de requête" use:modal>
	<header class="title">
		<span class="app">Éditeur de requête</span>
		<span class="name">{q.name}</span>
		<nav class="tabs" aria-label="Onglets du ruban">
			{#each TABS as t (t)}
				<button class="tab" data-label={t} class:on={tab === t} onclick={() => (tab = t)}>{t}</button>
			{/each}
		</nav>
		<span class="grow"></span>
		<button class="close" title="Annuler : fermer l’éditeur de requête sans enregistrer (le classeur reste ouvert)" aria-label="Annuler et fermer l’éditeur de requête" onclick={onclose}><Icon name="win-close" size={16} /></button>
	</header>
	<nav class="ribbon">
		{#each ribbon[tab] as g (g.title)}
			<section class="group">
				<div class="cmds">
					{#each g.big as c (c.label)}
						<button class="cmd" class:pressed={c.on} disabled={c.off} title={c.hint ?? c.label} onclick={c.run}>
							<Icon name={c.icon} />
							<span class="label">{c.label}</span>
						</button>
					{/each}
					{#if g.small?.length}
						<div class="stack">
							{#each g.small as c (c.label)}
								<button class="cmd row-cmd" class:pressed={c.on} disabled={c.off} title={c.hint ?? c.label} onclick={c.run}>
									<Icon name={c.icon} size={16} />
									<span class="label">{c.label}</span>
								</button>
							{/each}
						</div>
					{/if}
				</div>
				<div class="gt">{g.title}</div>
			</section>
		{/each}
	</nav>
	<div class="main">
		<section class="preview">
			{#if preview?.error}
				<div class="err">
					<strong>{preview.errorStep === -1 ? 'Source' : `Étape ${preview.errorStep + 1} (${labels[preview.errorStep + 1] ?? ''})`}</strong> : {preview.error}
					{#if preview.errorStep >= 0 && preview.step < (q.steps ?? []).length}<span class="muted"> — aperçu de l'étape précédente</span>{/if}
				</div>
			{/if}
			<div class="table-wrap" class:dim={loading}>
				{#if preview && (preview.columns ?? []).length}
					<table>
						<thead>
							<tr>
								<th class="num"></th>
								{#each shown as c (c.name)}
									{@const ic = typeIconName(typeOf(c.type))}
									<th
										class:on={chosen.includes(c.name)}
										class:open={menu?.col === c.name}
										onclick={(ev) => clickHeader(ev, c.name)}
										oncontextmenu={(ev) => onHeaderMenu(ev, c.name)}
										title="{c.name} : {typeName(c.type)} — clic droit ou ▾ : menu de la colonne ; Ctrl ou Maj : plusieurs colonnes"
									>
										<span class="head">
											<span class="type-icon" aria-label={typeName(c.type)}>{#if ic}<Icon name={ic} size={14} />{:else}{typeIcon(typeOf(c.type))}{/if}</span>
											<span class="cname">{c.name}</span>
											<button class="drop" aria-label="Menu de la colonne {c.name}" aria-haspopup="menu" aria-expanded={menu?.col === c.name} onclick={(ev) => onDrop(ev, c.name)}>▾</button>
										</span>
									</th>
								{/each}
							</tr>
						</thead>
						<tbody>
							{#each preview.rows ?? [] as r, i (i)}
								<tr>
									<td class="num">{i + 1}</td>
									{#each r as v, j (j)}
										<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
										<td
											class:on={onCol[j]}
											class:right={preview.columns?.[j]?.type === 'integer' || preview.columns?.[j]?.type === 'decimal'}
											onclick={(ev) => preview?.columns?.[j] && clickCell(ev, preview.columns[j].name)}
											oncontextmenu={(ev) => preview?.columns?.[j] && onHeaderMenu(ev, preview.columns[j].name)}>{v}</td
										>
									{/each}
								</tr>
							{/each}
						</tbody>
					</table>
				{:else if loading}
					<p class="muted pad">Calcul de l'aperçu…</p>
				{:else if !preview?.error}
					<p class="muted pad">Aucune donnée.</p>
				{/if}
			</div>
			<footer class="status">
				{#if preview}
					{(preview.columns ?? []).length} colonne(s), {preview.count.toLocaleString('fr-FR')} ligne(s){preview.sample ? ' — aperçu calculé sur les 1 000 premières lignes de chaque source' : ''}{preview.count > (preview.rows ?? []).length ? ` (${(preview.rows ?? []).length} affichées)` : ''}
					<button class="link" onclick={() => (showSQL = !showSQL)}>{showSQL ? 'Masquer' : 'Voir'} le SQL</button>
				{/if}
			</footer>
			{#if showSQL && preview?.sql}<pre class="sql">{preview.sql}</pre>{/if}
		</section>
		<aside class="side">
			<h3>Propriétés</h3>
			<label class="field">Nom <input bind:value={q.name} /></label>
			<h3>Étapes appliquées</h3>
			<ol class="steps">
				<li class:on={sel === -1} class:bad={preview?.errorStep === -1}>
					<button class="step" onclick={() => select(-1)}>{labels[0]}</button>
					<button class="x" title="Modifier la source" onclick={() => (sourceOpen = true)}>⚙</button>
				</li>
				{#each q.steps ?? [] as s, i (i)}
					<li class:on={sel === i} class:bad={preview?.errorStep === i}>
						<button class="step" onclick={() => select(i)}>{labels[i + 1] ?? s.op}</button>
						<button class="x" title="Modifier l'étape" onclick={() => editStep(i)}>⚙</button>
						<button class="x" title="Monter" disabled={i === 0} onclick={() => move(i, -1)}>↑</button>
						<button class="x" title="Descendre" disabled={i === (q.steps ?? []).length - 1} onclick={() => move(i, 1)}>↓</button>
						<button class="x" title="Supprimer l'étape" onclick={() => removeStep(i)}>✕</button>
					</li>
				{/each}
			</ol>
			<p class="muted">Une nouvelle étape s'insère après l'étape choisie. Cliquez un en-tête de colonne pour la choisir (Ctrl ou Maj : plusieurs) ; son ▾ ou un clic droit ouvre le menu de la colonne.</p>
			{#if error}<p class="error">{error}</p>{/if}
		</aside>
	</div>
</div>

{#if menu}
	{@const m = menu}
	{@const one = chosen.length === 1}
	{@const many = chosen.length > 1 ? ` (${chosen.length} colonnes)` : ''}
	<!-- Menu de la colonne, comme celui de Power Query : il agit sur les colonnes choisies. -->
	<div class="menu" role="menu" tabindex="-1" style:left="{m.x}px" style:top="{m.y}px" use:dismiss={closeMenu} use:inView>
		<div class="menu-title" title={one && clipped(chosen[0]) ? chosen[0] : undefined}>{one ? `« ${clip(chosen[0])} »` : `${chosen.length} colonnes choisies`}</div>
		<button role="menuitem" onclick={() => quick('remove')}><Icon name="delete-column" size={16} />Supprimer{many}</button>
		<button role="menuitem" onclick={() => quick('keep')}><Icon name="choose-columns" size={16} />Supprimer les autres colonnes</button>
		<button role="menuitem" onclick={() => quick('distinct')}><Icon name="remove-duplicates" size={16} />Supprimer les doublons</button>
		<hr />
		<button role="menuitem" disabled={!one} onclick={() => addStep('rename')}><Icon name="rename" size={16} />Renommer…</button>
		<Submenu label="Modifier le type">
			{#snippet icon()}<Icon name="convert-type" size={16} />{/snippet}
			{@const cur = one ? shown.find((c) => c.name === chosen[0])?.type : undefined}
			{#each TYPES as [t, label] (t)}
				{@const ic = typeIconName(t as ColumnType)}
				<button role="menuitemradio" aria-checked={cur === t} onclick={() => quick('type', t)}>
					<span class="check">{cur === t ? '✓' : ''}</span><span class="type-icon">{#if ic}<Icon name={ic} size={14} />{:else}{typeIcon(t as ColumnType)}{/if}</span>{label}
				</button>
			{/each}
		</Submenu>
		<Submenu label="Format">
			{#snippet icon()}<Icon name="text-format" size={16} />{/snippet}
			<button role="menuitem" onclick={() => quick('transform', 'trim')}>Supprimer les espaces (SUPPRESPACE)</button>
			<button role="menuitem" onclick={() => quick('transform', 'upper')}>MAJUSCULE</button>
			<button role="menuitem" onclick={() => quick('transform', 'lower')}>minuscule</button>
		</Submenu>
		<button role="menuitem" disabled={!one} onclick={() => addStep('replace')}><Icon name="replace" size={16} />Remplacer les valeurs…</button>
		<button role="menuitem" disabled={!one} onclick={() => addStep('split')}><Icon name="split-column" size={16} />Fractionner la colonne…</button>
		<hr />
		<button role="menuitem" onclick={() => quick('sortAsc')}><Icon name="sort-asc" size={16} />Trier par ordre croissant</button>
		<button role="menuitem" onclick={() => quick('sortDesc')}><Icon name="sort-desc" size={16} />Trier par ordre décroissant</button>
		<button role="menuitem" disabled={!one} onclick={() => addStep('filter')}><Icon name="filter" size={16} />Filtrer les lignes…</button>
		<hr />
		<button role="menuitem" onclick={() => addStep('group')}><Icon name="group-by" size={16} />Regrouper par…</button>
		<button role="menuitem" onclick={() => quick('unpivot')}><Icon name="unpivot" size={16} />Dépivoter les colonnes</button>
		<button role="menuitem" disabled={chosen.length >= shown.length} onclick={() => quick('unpivotOthers')}><Icon name="unpivot" size={16} />Dépivoter les autres colonnes</button>
		<button role="menuitem" disabled={!one} onclick={() => addStep('pivot')}><Icon name="pivot-column" size={16} />Pivoter la colonne…</button>
	</div>
{/if}
{#if form}
	<StepForm step={form.step} cols={form.cols} {others} {otherColumns} onok={applyForm} oncancel={() => (form = null)} />
{/if}
{#if sourceOpen}
	<SourceForm
		source={q.source}
		queries={otherQueries}
		{tables}
		{newConnection}
		{askPassword}
		onok={setSource}
		oncancel={() => {
			sourceOpen = false;
			const r = sourceCancel(creating, q, !!preview);
			if (r === 'close') onclose();
			else if (r === 'refresh') refresh();
		}}
	/>
{/if}

<style>
	.editor {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: #fff;
		display: flex;
		flex-direction: column;
		font-size: 12px;
		color: #1f2328;
	}
	.title {
		display: flex;
		align-items: center;
		gap: 10px;
		height: 32px;
		padding: 0 4px 0 12px;
		background: #217346;
		color: #fff;
	}
	.title .app {
		font-weight: 600;
	}
	.title .name {
		opacity: 0.9;
	}
	.grow {
		flex: 1;
	}
	.title .close {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 0;
		background: none;
		border: none;
		color: #fff;
		width: 32px;
		height: 28px;
	}
	.title .close:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.2);
	}
	/* Onglets du ruban dans la barre de titre, comme ceux du classeur (Ribbon.svelte). */
	.tabs {
		display: flex;
		align-self: stretch;
		gap: 2px;
		margin-left: 12px;
	}
	.tabs .tab {
		height: auto;
		font: inherit;
		color: #e8f3ec;
		background: none;
		border: none;
		padding: 0 14px;
		border-radius: 4px 4px 0 0;
		margin-top: 4px;
	}
	/* Largeur du gras réservée : l'onglet actif ne décale pas les autres. */
	.tab::after {
		content: attr(data-label);
		display: block;
		height: 0;
		overflow: hidden;
		visibility: hidden;
		font-weight: 600;
	}
	.tabs .tab:hover:not(.on) {
		background: rgba(255, 255, 255, 0.12);
	}
	.tabs .tab.on {
		background: #f3f4f6;
		color: #14532d;
		font-weight: 600;
	}
	.ribbon {
		display: flex;
		height: 78px;
		flex: none;
		border-bottom: 1px solid #d4d7db;
		background: #f3f4f6;
		padding: 4px 6px;
		overflow-x: auto;
		scrollbar-width: none;
	}
	.group {
		flex: none;
		display: flex;
		flex-direction: column;
		border-right: 1px solid #d4d7db;
		padding: 0 8px;
	}
	.cmds {
		display: flex;
		gap: 2px;
		flex: 1;
		min-height: 0;
	}
	.stack {
		display: grid;
		grid-template-rows: repeat(3, 1fr);
		grid-auto-flow: column;
		gap: 0 2px;
	}
	.ribbon .cmd {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: flex-start;
		gap: 2px;
		min-width: 54px;
		/* Libellé sur deux lignes au plus, comme le ruban de Power Query. */
		max-width: 82px;
		height: auto;
		padding: 2px 6px;
		font-size: 11px;
		background: none;
		border: 1px solid transparent;
		border-radius: 3px;
		color: #1f2328;
		--icon-accent: #217346;
	}
	.ribbon .cmd .label {
		line-height: 1.15;
		text-align: center;
	}
	.ribbon .cmd:hover:not(:disabled) {
		background: #e2e8e4;
		border-color: #c5d6cb;
	}
	.ribbon .cmd.pressed {
		background: #d3e7dc;
		box-shadow: inset 0 0 0 1px #8fbc9f;
	}
	.ribbon .cmd:disabled {
		color: #9aa0a6;
		opacity: 1;
		--icon-accent: currentColor;
	}
	.ribbon .cmd.row-cmd {
		flex-direction: row;
		align-items: center;
		min-width: 0;
		max-width: none;
		gap: 6px;
		padding: 1px 6px;
		font-size: 12px;
	}
	.ribbon .cmd.row-cmd .label {
		white-space: nowrap;
		text-align: left;
	}
	.gt {
		text-align: center;
		color: #6b7280;
		font-size: 11px;
		white-space: nowrap;
	}
	/* Menu d'une colonne de l'aperçu (comme le menu ▾ des colonnes de la feuille). */
	.menu {
		position: fixed;
		z-index: calc(var(--z-dialog) + 5);
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 240px;
		max-width: 320px; /* MENU_MAX_WIDTH */
		font-size: 12px;
		color: #1f2328;
		--icon-accent: #217346;
		--menu-item-padding: 5px 14px 5px 10px; /* entrées des sous-menus (Submenu) */
	}
	.menu-title {
		padding: 4px 14px;
		font-size: 11px;
		color: #6b7280;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.menu button {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		height: auto;
		text-align: left;
		padding: 5px 14px 5px 10px;
		background: none;
		border: none;
		border-radius: 0;
		color: #1f2328;
		white-space: nowrap;
	}
	.menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.menu button:disabled {
		color: #9aa0a6;
		opacity: 1;
		--icon-accent: currentColor;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e3e5e8;
		margin: 4px 0;
	}
	.check {
		display: inline-block;
		width: 12px;
		color: #217346;
	}
	.type-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 22px;
		font-size: 10px;
		font-weight: 600;
		color: #6b8f78;
		--icon-accent: #217346;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 2px 8px;
		cursor: pointer;
		height: 24px;
	}
	button:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.main {
		flex: 1;
		display: flex;
		min-height: 0;
	}
	.preview {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.err {
		padding: 6px 10px;
		background: #fdecea;
		color: #b42318;
		border-bottom: 1px solid #f4c7c3;
	}
	.table-wrap {
		flex: 1;
		overflow: auto;
	}
	.table-wrap.dim {
		opacity: 0.55;
	}
	table {
		border-collapse: collapse;
		font-variant-numeric: tabular-nums;
	}
	th,
	td {
		border: 1px solid #e1e4e8;
		padding: 2px 6px;
		white-space: nowrap;
		max-width: 260px;
		overflow: hidden;
		text-overflow: ellipsis;
		text-align: left;
	}
	th {
		position: sticky;
		top: 0;
		background: #f3f4f6;
		font-weight: 600;
		cursor: pointer;
		user-select: none;
		padding: 0 0 0 2px;
		z-index: 1;
	}
	th.num {
		z-index: 2;
	}
	.head {
		display: flex;
		align-items: center;
		gap: 4px;
		height: 24px;
	}
	.cname {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	/* ▾ de l'en-tête, comme celui des colonnes de la feuille. */
	th .drop {
		flex: none;
		width: 18px;
		height: 18px;
		margin-right: 2px;
		padding: 0;
		font-size: 10px;
		line-height: 1;
		color: #4b5563;
		background: #fff;
		border: 1px solid #c9cdd2;
		border-radius: 2px;
	}
	th .drop:hover:not(:disabled),
	th.open .drop {
		border-color: #217346;
		background: #e3efe7;
		color: #14532d;
	}
	th.on {
		background: #cfe3d6;
	}
	td.on {
		background: #e3efe7;
	}
	td.right {
		text-align: right;
	}
	.num {
		color: #9aa0a6;
		background: #f7f8f9;
		text-align: right;
		position: sticky;
		left: 0;
	}
	.status {
		border-top: 1px solid #e1e4e8;
		padding: 3px 10px;
		color: #4b5563;
		display: flex;
		gap: 8px;
		align-items: center;
	}
	.link {
		border: none;
		background: none;
		color: #217346;
		padding: 0;
		height: auto;
	}
	.sql {
		margin: 0;
		max-height: 160px;
		overflow: auto;
		background: #f7f8f9;
		border-top: 1px solid #e1e4e8;
		padding: 6px 10px;
		font-size: 11px;
	}
	.side {
		width: 280px;
		flex: none;
		border-left: 1px solid #d4d7db;
		background: #fafbfb;
		padding: 8px 10px;
		overflow: auto;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	h3 {
		margin: 4px 0 0;
		font-size: 12px;
		font-weight: 600;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	input {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		border: 1px solid #e5e7eb;
		background: #fff;
	}
	.steps li {
		display: flex;
		align-items: center;
	}
	.steps li.on {
		background: #d9eadf;
	}
	.steps li.bad .step {
		color: #b42318;
	}
	.steps li.bad .step::before {
		content: '⚠ ';
	}
	.step {
		flex: 1;
		text-align: left;
		border: none;
		background: none;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.x {
		border: none;
		background: none;
		padding: 0 3px;
		color: #4b5563;
	}
	.muted {
		color: #6b7280;
		margin: 0;
	}
	.pad {
		padding: 12px;
	}
	.error {
		color: #b42318;
		margin: 0;
	}
</style>
