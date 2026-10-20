<script lang="ts">
	// Onglets de feuilles en bas, comme Excel : clic pour changer de feuille,
	// double-clic pour renommer, clic droit pour insérer, renommer, supprimer,
	// déplacer ou copier, colorer, masquer ou afficher, glisser pour
	// réordonner, « + » pour ajouter à la fin. Une feuille protégée porte un
	// cadenas ; la structure du classeur protégée fige les onglets (ni ajout,
	// ni renommage, ni déplacement, ni suppression, ni masquage). Le menu se
	// ferme sur Échap, sur un clic ailleurs et quand la fenêtre change de taille.
	import Icon from '#lib/icons/Icon.svelte';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { dismiss } from '#lib/grid/dismiss.js';
	import Submenu from '#lib/ui/Submenu.svelte';
	import { clip, clipped } from '#lib/ui/menu.js';
	import { isCoarse, revealX } from '#lib/ui/mobile.js';
	import { cycle, modal } from '#lib/ui/focus.js';
	import { sheetNameProblem, refusedName } from '#lib/ui/names.js';
	import { nameAlertOpen, refuseName } from '#lib/ui/nameAlert.js';
	import { TAB_COLORS, moveTarget, tabWindow, textOn, visibleSheets, type SimilarMark, type TabSheet } from './sheets.js';
	import { allLabel, choiceLabel, choiceStatus, newLabel, nextChoice, type ChoiceActions } from './choice.js';
	import RowFormulaDialog from '#lib/ui/RowFormulaDialog.svelte';
	import { errorText, t } from '#lib/i18n/index.js';

	type Sheet = TabSheet;
	interface Props {
		/** Toutes les feuilles, masquées comprises (elles n'ont pas d'onglet). */
		sheets: Sheet[];
		active: number;
		onselect: (id: number) => void;
		onadd?: () => void;
		/** Insère une feuille avant la feuille before. */
		oninsert?: (before: number) => void;
		/** Nouvel onglet page HTML, avant la feuille before (absente : à la fin). */
		onpage?: (before?: number) => void;
		/** Nouvel onglet Document, avant la feuille before (absente : à la fin). */
		ondocpage?: (before?: number) => void;
		/** Place la feuille id au rang to (0 : première). */
		onmove?: (id: number, to: number) => void;
		/** Renomme la feuille ; un refus (exception) rouvre la saisie après la boîte « nom invalide ». */
		onrename?: (id: number, name: string) => unknown;
		ondelete?: (id: number) => void;
		/** Supprime plusieurs feuilles (onglets sélectionnés par Ctrl+clic ou Maj+clic). */
		ondeletemany?: (ids: number[]) => void;
		/** Range toutes les feuilles dans cet ordre (glisser plusieurs onglets). */
		onorder?: (ids: number[]) => void;
		/** Copie la feuille id et place la copie au rang to. */
		oncopy?: (id: number, to: number) => void;
		/** Couleur d'onglet (« #rrggbb », vide : aucune). */
		oncolor?: (id: number, color: string) => void;
		/** Couleur de plusieurs onglets (sélection), en une étape d'annulation. */
		oncolormany?: (ids: number[], color: string) => void;
		/** Masque (true) ou affiche (false) une feuille. */
		onhide?: (id: number, hidden: boolean) => void;
		/** Masquer plusieurs feuilles (sélection), en une étape d'annulation. */
		onhidemany?: (ids: number[]) => void;
		/** Défilement horizontal de la grille : barre en bout de ligne, comme Excel. */
		hscroll?: { content: number; view: number; left: number };
		/** atEnd : la barre est au bout (la feuille s'agrandit). */
		onhscroll?: (left: number, atEnd: boolean) => void;
		/** Barre tirée (true) puis relâchée (false) : la largeur reste figée pendant le geste. */
		onhhold?: (on: boolean) => void;
		/** Protéger la feuille, ou ôter sa protection (menu de l'onglet). */
		onprotect?: (id: number) => void;
		/** Structure du classeur protégée : onglets figés. */
		locked?: boolean;
		/** Action refusée (structure protégée) : message pour la barre d'état. */
		onrefused?: (msg: string) => void;
		/** Onglet à choix (« Ventes · 2025 ▾ ») : liste des valeurs, « Choisir par… » (absent : pas de flèche). */
		onchoice?: ChoiceActions;
		/**
		 * Onglets semblables (regrouper-onglets-semblables.md §4.1) : chaque
		 * groupe souligne ses onglets d'un trait de sa couleur, et une puce
		 * « ⧉ 12 onglets semblables » suit la série (clic : volet Regrouper ;
		 * clic droit : Ne plus proposer).
		 */
		similar?: SimilarMark[];
		onmergegroup?: (i: number) => void;
		ondismissgroup?: (i: number) => void;
		/** Clic droit › Regrouper des onglets… sur un onglet ou une sélection d'onglets. */
		onmergesheets?: (ids: number[]) => void;
	}

	let { sheets, active, onselect, onadd: add, oninsert: insert, onpage: page, ondocpage: docPage, onmove: move, onrename: rename, ondelete: del, ondeletemany: delMany, onorder: order, oncopy: copy, oncolor: color, oncolormany: colorMany, onhide: hide, onhidemany: hideMany, hscroll, onhscroll, onhhold, onprotect, locked = false, onrefused, similar = [], onmergegroup, ondismissgroup, onmergesheets, onchoice }: Props = $props();
	// Onglets semblables : trait de couleur sous chaque onglet d'un groupe ;
	// la puce survolée fait ressortir les siens.
	const marks = $derived(new Map(similar.flatMap((g, i) => g.sheets.map((id) => [id, { color: g.color, group: i }] as const))));
	let hot = $state<number | null>(null);
	let chipMenu = $state<{ group: number; x: number } | null>(null);
	const CHIP_W = 150;


	// --- Onglet à choix (choice.ts) ---------------------------------------------
	/** Libellé de l'onglet : « Ventes · 2025 » pour un onglet à choix. */
	const label = (s: Sheet) => (s.choice ? choiceLabel(s.name, s.choice) : s.name);
	// Liste de la flèche : valeurs (null : en cours de lecture).
	let choiceMenu = $state<{ id: number; x: number; values: { text: string; count: number }[] | null } | null>(null);
	// « Nouvelle année… » : valeur proposée (suite de la série), modifiable.
	let adding = $state<{ id: number; name: string; draft: string } | null>(null);
	// « Choisir par › Une formule… » : feuille de la boîte (null : fermée).
	let byFormula = $state<{ id: number; name: string; formula: string } | null>(null);
	// « Choisir par… » : colonnes proposées pour l'onglet du menu (null : en cours de lecture).
	let candidates = $state<{ table: string; column: string; values: number }[] | null>(null);

	async function openChoice(s: Sheet, el: HTMLElement | null) {
		if (!onchoice || !s.choice) return;
		menu = null;
		const m = { id: s.id, x: el ? leftOf(el) : 0, values: null as { text: string; count: number }[] | null };
		choiceMenu = m;
		try {
			const values = await onchoice.values(s.id);
			if (choiceMenu?.id === s.id) choiceMenu = { ...choiceMenu, values };
		} catch (e) {
			choiceMenu = null;
			onrefused?.(errorText(e));
		}
	}

	function choose(id: number, value: string, all: boolean) {
		choiceMenu = null;
		if (id !== active) onselect(id);
		onchoice?.choose(id, value, all);
	}

	function startAdding(id: number) {
		const s = sheets.find((x) => x.id === id);
		const values = (choiceMenu?.values ?? []).map((v) => v.text);
		choiceMenu = null;
		if (!s?.choice) return;
		adding = { id, name: s.choice.name, draft: nextChoice(values) };
	}

	function confirmAdding() {
		const a = adding;
		adding = null;
		if (!a || !a.draft.trim()) return;
		if (a.id !== active) onselect(a.id);
		onchoice?.add(a.id, a.draft.trim());
	}
	const onadd = $derived(locked ? undefined : add);
	const oninsert = $derived(locked ? undefined : insert);
	const onpage = $derived(locked ? undefined : page);
	const ondocpage = $derived(locked ? undefined : docPage);
	// Menu du bouton « + » (clic droit) : feuille ou onglet HTML.
	let addMenu = $state<{ x: number } | null>(null);
	const onmove = $derived(locked ? undefined : move);
	const onrename = $derived(locked ? undefined : rename);
	const ondelete = $derived(locked ? undefined : del);
	const ondeletemany = $derived(locked ? undefined : delMany);
	const onorder = $derived(locked ? undefined : order);
	const oncopy = $derived(locked ? undefined : copy);
	const oncolor = $derived(locked ? undefined : color);
	const onhide = $derived(locked ? undefined : hide);
	const oncolormany = $derived(locked ? undefined : colorMany);
	const onhidemany = $derived(locked ? undefined : hideMany);
	// Onglets affichés : les feuilles masquées n'en ont pas.
	const shown = $derived(visibleSheets(sheets));
	const hiddenSheets = $derived(sheets.filter((s) => s.hidden));
	// Barre de défilement horizontale : une zone qui défile au même rapport
	// que la grille (contenu / largeur visible).
	let bar = $state<HTMLDivElement>();
	let barW = $state(0);
	// held : la barre est tirée à la souris ; elle ne suit alors pas la
	// grille (sinon elle « glisse » sous la souris).
	let held = false;
	$effect(() => {
		const h = hscroll;
		if (!bar || !h || !h.view || held) return;
		const want = (h.left * barW) / h.view;
		if (Math.abs(bar.scrollLeft - want) > 1) bar.scrollLeft = want;
	});
	function onBarScroll() {
		if (!bar || !hscroll?.view || !barW) return;
		const left = (bar.scrollLeft * hscroll.view) / barW;
		// Défilement venu de la grille elle-même : rien à renvoyer.
		if (!held && Math.abs(left - hscroll.left) < 1) return;
		onhscroll?.(left, bar.scrollLeft + bar.clientWidth >= bar.scrollWidth - 2);
	}

	let renaming = $state<number | null>(null);
	let draft = $state('');
	let renameInput = $state<HTMLInputElement>();
	// Renommage envoyé, réponse attendue : ni second envoi ni abandon.
	let renameBusy = false;
	let menu = $state<{ id: number; x: number } | null>(null);
	// Dialogues « Déplacer ou copier » et « Afficher ».
	let moving = $state<{ id: number; before: number | null; copy: boolean } | null>(null);
	let unhiding = $state<number | null>(null);

	function closeMenu() {
		menu = null;
	}

	function confirmMove() {
		const m = moving;
		moving = null;
		if (!m) return;
		const order = sheets.map((s) => s.id);
		const to = moveTarget(order, m.id, m.before, m.copy);
		if (m.copy) oncopy?.(m.id, to);
		else if (to !== order.indexOf(m.id)) onmove?.(m.id, to);
	}

	function confirmUnhide() {
		const id = unhiding;
		unhiding = null;
		if (id !== null) onhide?.(id, false);
	}

	/** Donne le focus à la liste du dialogue (Échap et Entrée y sont lus). */
	const focus = (node: HTMLElement) => {
		node.focus();
	};

	/** Flèches sur un onglet (atteint par Tab ou F6) : onglet voisin ; Entrée ou Espace l'affiche ; Alt+↓ ouvre la liste d'un onglet à choix. */
	function tabKeys(e: KeyboardEvent) {
		const t = e.target as HTMLElement;
		if (t.classList?.contains('sheet') && e.altKey && !e.ctrlKey && e.key === 'ArrowDown') {
			const s = sheets.find((x) => String(x.id) === t.dataset.sheet);
			if (s?.choice && onchoice) {
				e.preventDefault();
				openChoice(s, t);
			}
			return;
		}
		if (!t.classList?.contains('sheet') || e.ctrlKey || e.altKey || e.metaKey) return;
		const all = tabEls.filter((el) => el?.isConnected);
		const i = all.indexOf(t as HTMLButtonElement);
		const to = e.key === 'ArrowRight' ? cycle(all.length, i, 1) : e.key === 'ArrowLeft' ? cycle(all.length, i, -1) : e.key === 'Home' ? 0 : e.key === 'End' ? all.length - 1 : -1;
		if (to < 0 || !all[to]) return;
		e.preventDefault();
		all[to].focus();
	}

	/** Échap ferme un dialogue, Entrée le valide. */
	function dialogKeys(e: KeyboardEvent, ok: () => void, cancel: () => void) {
		e.stopPropagation();
		if (e.key === 'Escape') cancel();
		else if (e.key === 'Enter') {
			e.preventDefault();
			ok();
		}
	}
	function startRename(s: Sheet) {
		if (locked) onrefused?.(t('shell.tabs.locked'));
		if (!onrename) return;
		renaming = s.id;
		draft = s.name;
	}

	/**
	 * Fin du renommage : Échap (save faux) abandonne ; sinon un nom refusé
	 * (vide, déjà pris, refus du moteur) ouvre la boîte « nom invalide » et
	 * la saisie reprend sur le texte tapé, sélectionné.
	 */
	async function finish(save: boolean) {
		const id = renaming;
		if (id === null || renameBusy || nameAlertOpen()) return;
		const cur = sheets.find((s) => s.id === id);
		const name = draft.trim();
		if (!save || !onrename || !cur || name === cur.name) {
			renaming = null;
			return;
		}
		const problem = sheetNameProblem(name, sheets.filter((s) => s.id !== id).map((s) => s.name));
		if (problem) {
			await refuseName(() => renameInput, problem);
			return;
		}
		renameBusy = true;
		try {
			await onrename(id, name);
			renaming = null;
		} catch (e) {
			await refuseName(() => renameInput, refusedName(t('shell.tabs.renameRefused'), e));
		} finally {
			renameBusy = false;
		}
	}

	const index = $derived(shown.findIndex((s) => s.id === active));

	// Doigt (téléphone, tablette) : tous les onglets, dans une bande qui
	// défile au doigt (l'onglet actif y reste visible) ; pas de barre de
	// défilement de la grille, qui défile elle-même au doigt.
	const coarse = isCoarse();
	let host: HTMLElement;
	let strip = $state<HTMLElement>();
	/** Gauche d'un onglet dans la barre (les menus s'y posent), bande défilée comprise. */
	const leftOf = (el: HTMLElement) => el.getBoundingClientRect().left - host.getBoundingClientRect().left;
	$effect(() => {
		void active;
		void vis.length;
		const st = strip;
		if (!coarse || !st || st.scrollWidth <= st.clientWidth) return;
		const el = st.querySelector<HTMLElement>('.sheet.on');
		if (!el) return;
		const left = el.getBoundingClientRect().left - st.getBoundingClientRect().left + st.scrollLeft;
		const to = revealX(left, left + el.offsetWidth, st.scrollLeft, st.clientWidth);
		if (to !== st.scrollLeft) st.scrollLeft = to;
	});

	// Trop d'onglets pour garder à la barre de défilement sa largeur minimale
	// (le quart de la barre, 150 px au moins) : ceux qui ne tiennent pas se
	// cachent derrière « … », qui les liste tous ; l'onglet actif reste visible.
	let width = $state(0);
	let navW = $state(0);
	const MORE_W = 30;
	const ADD_W = 24;
	const measurer = typeof document !== 'undefined' ? document.createElement('canvas').getContext('2d') : null;
	function tabWidth(s: Sheet, on: boolean): number {
		if (!measurer) return 100;
		measurer.font = `${on ? '600 ' : ''}13px 'Segoe UI', system-ui, -apple-system, sans-serif`;
		return Math.ceil(measurer.measureText(label(s)).width) + 37 + (s.protected || locked ? 16 : 0) + (s.choice ? 30 : 0);
	}
	const range = $derived.by(() => {
		if (!width || coarse) return { from: 0, to: shown.length - 1 };
		const room = width - navW - ADD_W - (hscroll ? Math.max(width / 4, 150) : 0) - CHIP_W * similar.length;
		return tabWindow(
			shown.map((s) => tabWidth(s, s.id === active)),
			index,
			room,
			MORE_W
		);
	});
	const vis = $derived(shown.slice(range.from, range.to + 1));
	const hiddenTabs = $derived(shown.length - vis.length);
	let listing = $state(false);

	// Plusieurs onglets sélectionnés (Ctrl+clic, Maj+clic, comme Excel) : ils
	// se glissent et se suppriment ensemble. Vide : l'onglet actif seul.
	let picked = $state<Set<number>>(new Set());
	const group = $derived(picked.size > 1 ? picked : null);
	// Menu ouvert sur un onglet de la sélection : ses actions visent toutes les feuilles choisies.
	const many = $derived(!!menu && !!group?.has(menu.id));
	$effect(() => {
		// Feuilles disparues ou masquées : retirées de la sélection.
		if (picked.size && [...picked].some((id) => !shown.some((s) => s.id === id))) picked = new Set([...picked].filter((id) => shown.some((s) => s.id === id)));
	});
	function pick(e: MouseEvent, s: Sheet) {
		if (e.ctrlKey || e.metaKey) {
			const p = new Set(picked.size ? picked : [active]);
			if (p.has(s.id) && p.size > 1) p.delete(s.id);
			else p.add(s.id);
			picked = p;
			return;
		}
		if (e.shiftKey) {
			const a = shown.findIndex((x) => x.id === active);
			const b = shown.findIndex((x) => x.id === s.id);
			if (a >= 0 && b >= 0) picked = new Set(shown.slice(Math.min(a, b), Math.max(a, b) + 1).map((x) => x.id));
			return;
		}
		picked = new Set();
		onselect(s.id);
	}
	/** Ordre de toutes les feuilles après le dépôt du groupe g avant la feuille before (null : à la fin). */
	function groupOrder(g: Set<number>, before: number | null): number[] {
		const order = sheets.map((s) => s.id);
		const moved = order.filter((id) => g.has(id));
		const others = order.filter((id) => !g.has(id));
		// Déposé sur un onglet du groupe : avant le premier onglet qui n'en est pas, après lui.
		let at = before === null ? -1 : order.indexOf(before);
		while (at >= 0 && at < order.length && g.has(order[at])) at++;
		const k = at < 0 || at >= order.length ? others.length : others.indexOf(order[at]);
		return [...others.slice(0, k), ...moved, ...others.slice(k)];
	}

	// Glisser un onglet : au-delà de quelques pixels, l'onglet suit la souris et
	// un trait marque où il sera déposé (drop : rang d'insertion dans sheets).
	let drag = $state<{ id: number; x0: number; moved: boolean; drop: number } | null>(null);
	let tabEls: HTMLElement[] = $state([]);
	let dropX = $state(0);

	function dragStart(e: PointerEvent, s: Sheet) {
		// Au doigt, glisser fait défiler la bande (Déplacer : menu de l'onglet).
		if (!onmove || e.button !== 0 || vis.length < 2 || e.pointerType === 'touch') return;
		drag = { id: s.id, x0: e.clientX, moved: false, drop: -1 };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function dragMove(e: PointerEvent) {
		if (!drag) return;
		if (!drag.moved && Math.abs(e.clientX - drag.x0) < 6) return;
		drag.moved = true;
		const rects = tabEls.map((el) => el?.getBoundingClientRect());
		let k = rects.findIndex((r) => r && e.clientX < r.left + r.width / 2);
		if (k < 0) k = vis.length;
		drag.drop = k;
		const box = host.getBoundingClientRect();
		const r = rects[Math.min(k, rects.length - 1)];
		dropX = r ? (k < rects.length ? r.left : r.right) - box.left : 0;
	}
	function dragEnd() {
		const d = drag;
		drag = null;
		if (!d?.moved || d.drop < 0) return;
		if (group?.has(d.id) && onorder) {
			const next = groupOrder(group, vis[d.drop]?.id ?? shown[range.to + 1]?.id ?? null);
			if (next.some((id, i) => id !== sheets[i]?.id)) onorder(next);
			return;
		}
		// Rang parmi toutes les feuilles (les masquées gardent leur place).
		const order = sheets.map((s) => s.id);
		// Déposé après le dernier onglet visible : avant le premier caché (ou en dernier).
		const to = moveTarget(order, d.id, vis[d.drop]?.id ?? shown[range.to + 1]?.id ?? null, false);
		if (to !== order.indexOf(d.id)) onmove?.(d.id, to);
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="sheet-tabs" bind:this={host} bind:clientWidth={width} data-region="tabs" onkeydown={tabKeys}>
	<div class="nav" bind:clientWidth={navW}>
		<button disabled={index <= 0} onclick={() => onselect(shown[index - 1].id)} title={t('shell.tabs.previousTooltip')} aria-label={t('shell.tabs.previous')}>◀</button>
		<button disabled={index < 0 || index >= shown.length - 1} onclick={() => onselect(shown[index + 1].id)} title={t('shell.tabs.nextTooltip')} aria-label={t('shell.tabs.next')}>▶</button>
	</div>
	<div class="strip" class:scroll={coarse} bind:this={strip}>
	{#each vis as s, si (s.id)}
		{#if renaming === s.id}
			<!-- svelte-ignore a11y_autofocus -->
			<input
				class="rename"
				bind:this={renameInput}
				bind:value={draft}
				aria-label={t('shell.tabs.renameAria')}
				autofocus
				onfocus={(e) => e.currentTarget.select()}
				onkeydown={(e) => {
					e.stopPropagation();
					if (e.key === 'Enter') {
						e.preventDefault();
						finish(true);
					}
					if (e.key === 'Escape') finish(false);
				}}
				onblur={() => finish(true)}
			/>
		{:else}
			<button
				bind:this={tabEls[si]}
				class="sheet"
				class:choice={!!s.choice}
				data-sheet={s.id}
				data-label={label(s) + (s.choice ? ' ▾' : '')}
				data-region-focus={s.id === active ? '' : undefined}
				aria-current={s.id === active ? 'page' : undefined}
				class:on={s.id === active}
				class:colored={!!s.color}
				style:--tab-color={s.color || null}
				style:--tab-text={s.color ? textOn(s.color) : null}
				class:dragging={drag?.moved && drag.id === s.id}
				onpointerdown={(e) => dragStart(e, s)}
				onpointermove={dragMove}
				onpointerup={dragEnd}
				onpointercancel={() => (drag = null)}
				class:picked={!!group?.has(s.id)}
				class:grouped={marks.has(s.id)}
				class:hot={hot !== null && marks.get(s.id)?.group === hot}
				style:--group-color={marks.get(s.id)?.color ?? null}
				onclick={(e) => pick(e, s)}
				ondblclick={() => startRename(s)}
				oncontextmenu={(e) => {
					e.preventDefault();
					if (del || rename || insert || onprotect || copy || color || hide || onchoice) {
						menu = { id: s.id, x: leftOf(e.currentTarget as HTMLElement) };
						choiceMenu = null;
						candidates = null;
						if (onchoice && !s.kind) {
							const id = s.id;
							onchoice
								.candidates(id)
								.then((c) => {
									if (menu?.id === id) candidates = c;
								})
								.catch(() => (candidates = []));
						}
					}
				}}
				title={(s.choice ? choiceStatus(s.choice) + (onchoice ? t('shell.tabs.tip.choiceKeys') : t('shell.tabs.tip.end')) : '') + (s.protected ? t('shell.tabs.tip.sheetProtected') : '') + (locked ? t('shell.tabs.tip.bookProtected') : onmove ? t('shell.tabs.tip.renameMove') : t('shell.tabs.tip.rename'))}
				>{#if s.protected || locked}<span class="lock" aria-label={s.protected ? t('shell.tabs.sheetProtected') : t('shell.tabs.bookProtected')}><Icon name="lock" size={12} /></span>{/if}{#if s.kind === 'html'}<span class="kind" aria-label={t('shell.tabs.htmlPage')}><Icon name="web" size={12} /></span>{:else if s.kind === 'doc'}<span class="kind" aria-label={t('shell.tabs.document')}><Icon name="document" size={12} /></span>{/if}{s.choice ? s.name : label(s)}{#if s.choice}<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
					<span
						class="choice-arrow"
						class:off={!onchoice}
						aria-hidden="true"
						onpointerdown={(e) => e.stopPropagation()}
						onclick={(e) => {
							e.stopPropagation();
							openChoice(s, tabEls[si]);
						}}>{label(s).slice(s.name.length + 3)} ▾</span
					>{/if}</button
			>
		{/if}
	{/each}
	</div>
	{#each similar as g, gi (gi)}
		{@const out = g.sheets.filter((id) => shown.some((s) => s.id === id) && !vis.some((s) => s.id === id)).length}
		<!-- Puce au bout de la série : survol, les onglets du groupe ressortent ; clic, le volet Regrouper. -->
		<button
			class="chip"
			style:--group-color={g.color}
			onmouseenter={() => (hot = gi)}
			onmouseleave={() => (hot = null)}
			onfocus={() => (hot = gi)}
			onblur={() => (hot = null)}
			onclick={() => onmergegroup?.(gi)}
			oncontextmenu={(e) => {
				e.preventDefault();
				chipMenu = { group: gi, x: (e.currentTarget as HTMLElement).offsetLeft };
			}}
			title={out ? t('shell.tabs.chipTooltipOut', { label: g.label, n: out }) : t('shell.tabs.chipTooltip', { label: g.label })}>⧉ {g.label}{out ? ` (+${out})` : ''}</button
		>
	{/each}
	{#if chipMenu}
		{@const cm = chipMenu}
		<div class="menu" role="menu" style:left="{cm.x}px" use:dismiss={() => (chipMenu = null)}>
			<button
				role="menuitem"
				onclick={() => {
					chipMenu = null;
					onmergegroup?.(cm.group);
				}}><span class="ic"><Icon name="append-queries" size={16} /></span>{t('shell.tabs.mergeShort')}</button
			>
			<button
				role="menuitem"
				onclick={() => {
					chipMenu = null;
					hot = null;
					ondismissgroup?.(cm.group);
				}}><span class="ic"><Icon name="no-color" size={16} /></span>{t('shell.tabs.dismiss')}</button
			>
		</div>
	{/if}
	{#if hiddenTabs > 0}
		<!-- Toujours à droite des onglets affichés (choix de Louis). -->
		<button class="more" onclick={() => (listing = true)} title={t('shell.tabs.moreTooltip', { n: hiddenTabs })}>…</button>
	{/if}
	{#if drag?.moved && drag.drop >= 0}<span class="drop-mark" style:left="{dropX - 1}px"></span>{/if}
	<button
		class="add"
		disabled={!onadd}
		onclick={() => onadd?.()}
		oncontextmenu={(e) => {
			e.preventDefault();
			if (onadd || onpage) addMenu = { x: (e.currentTarget as HTMLElement).offsetLeft };
		}}
		title={locked ? t('shell.tabs.add.locked') : page ? t('shell.tabs.add.tooltipPage') : t('shell.tabs.add.tooltip')}>+</button
	>
	{#if addMenu}
		<div class="menu" role="menu" style:left="{addMenu.x}px" use:dismiss={() => (addMenu = null)}>
			<button
				role="menuitem"
				disabled={!onadd}
				onclick={() => {
					addMenu = null;
					onadd?.();
				}}><span class="ic"><Icon name="sheet" size={16} /></span>{t('shell.tabs.add.sheet')}</button
			>
			{#if page}
			<button
				role="menuitem"
				disabled={!onpage}
				onclick={() => {
					addMenu = null;
					onpage?.();
				}}><span class="ic"><Icon name="web" size={16} /></span>{t('shell.tabs.add.page')}</button
			>
			{/if}
			<button
				role="menuitem"
				disabled={!ondocpage}
				onclick={() => {
					addMenu = null;
					ondocpage?.();
				}}><span class="ic"><Icon name="document" size={16} /></span>{t('shell.tabs.add.doc')}</button
			>
		</div>
	{/if}
	{#if hscroll && !coarse}
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div
			class="hbar"
			bind:this={bar}
			bind:clientWidth={barW}
			onscroll={onBarScroll}
			onpointerdown={() => {
				held = true;
				onhhold?.(true);
				const up = () => {
					window.removeEventListener('pointerup', up);
					held = false;
					onhhold?.(false);
				};
				window.addEventListener('pointerup', up);
			}}
		>
			<!-- Un peu de marge au bout : tirer la barre jusqu’au bout agrandit la feuille, comme Excel. -->
			<div style:width="{hscroll.view ? (barW * Math.max(hscroll.content, hscroll.view)) / hscroll.view + barW * 0.2 : 0}px" style:height="1px"></div>
		</div>
	{/if}
	{#if menu}
		{@const m = menu}
		{@const ms = sheets.find((x) => x.id === m.id)}
		<div class="menu" role="menu" style:left="{m.x}px" use:dismiss={closeMenu}>
			<div class="menu-title" title={!group?.has(m.id) && clipped(ms?.name) ? ms?.name : undefined}>{group?.has(m.id) ? t('shell.tabs.menu.selected', { n: group.size }) : t('shell.tabs.menu.title', { name: clip(ms?.name) })}</div>
			<!-- Ordre (demande de Louis) : la situation (plusieurs onglets choisis : les regrouper) ; le plus courant
			     (insérer ; renommer, déplacer, couleur) ; les données (choisir par, regrouper) ; protection et
			     affichage ; la suppression en dernier. -->
			{#snippet mergeItem()}
				<button
					role="menuitem"
					title={t('shell.tabs.menu.mergeTooltip')}
					onclick={() => {
						const ids = many ? [...(group ?? [])] : [m.id];
						closeMenu();
						if (many) picked = new Set();
						onmergesheets?.(ids);
					}}><span class="ic"><Icon name="append-queries" size={16} /></span>{t('shell.tabs.menu.merge')}</button
				>
			{/snippet}
			{#if many && onmergesheets}
				{@render mergeItem()}
				<hr />
			{/if}
			<Submenu label={t('shell.tabs.menu.insert')} disabled={!oninsert && !onpage && !ondocpage}>
				{#snippet icon()}<span class="ic"><Icon name="sheet" size={16} /></span>{/snippet}
				<button
					role="menuitem"
					disabled={!oninsert}
					onclick={() => {
						const id = m.id;
						closeMenu();
						oninsert?.(id);
					}}><span class="ic"><Icon name="sheet" size={16} /></span>{t('shell.tabs.menu.sheet')}</button
				>
				{#if page}
					<button
						role="menuitem"
						disabled={!onpage}
						onclick={() => {
							const id = m.id;
							closeMenu();
							onpage?.(id);
						}}><span class="ic"><Icon name="web" size={16} /></span>{t('shell.tabs.htmlPage')}</button
					>
				{/if}
				<button
					role="menuitem"
					disabled={!ondocpage}
					onclick={() => {
						const id = m.id;
						closeMenu();
						ondocpage?.(id);
					}}><span class="ic"><Icon name="document" size={16} /></span>{t('shell.tabs.document')}</button
				>
			</Submenu>
			<button
				role="menuitem"
				disabled={!onrename || many}
				title={many ? t('shell.tabs.one') : undefined}
				onclick={() => {
					// La feuille d'abord : ms se relit sur le menu, que la fermeture efface.
					const sheet = ms;
					closeMenu();
					if (sheet) startRename(sheet);
				}}><span class="ic"><Icon name="rename" size={16} /></span>{t('shell.tabs.menu.rename')}</button
			>
			<button
				role="menuitem"
				disabled={(!onmove && !oncopy) || many}
				title={many ? t('shell.tabs.one') : undefined}
				onclick={() => {
					const id = m.id;
					closeMenu();
					const i = sheets.findIndex((x) => x.id === id);
					moving = { id, before: sheets[i + 1]?.id ?? null, copy: !onmove };
				}}><span class="ic"><Icon name="move-sheet" size={16} /></span>{t('shell.tabs.menu.move')}</button
			>
			<Submenu label={t('shell.tabs.menu.color')} disabled={!oncolor}>
				{#snippet icon()}<span class="ic"><Icon name="tab-color" size={16} /></span>{/snippet}
				<div class="swatches">
					{#each TAB_COLORS as c (c)}
						<button
							class="swatch"
							class:sel={ms?.color === c}
							style:background={c}
							title={c}
							aria-label={t('shell.tabs.menu.colorAria', { color: c })}
							onclick={() => {
								const id = m.id;
								const ids = many ? [...(group ?? [])] : null;
								closeMenu();
								if (ids) oncolormany?.(ids, c);
								else oncolor?.(id, c);
							}}
						></button>
					{/each}
				</div>
				<button
					role="menuitem"
					disabled={!ms?.color}
					onclick={() => {
						const id = m.id;
						const ids = many ? [...(group ?? [])] : null;
						closeMenu();
						if (ids) oncolormany?.(ids, '');
						else oncolor?.(id, '');
					}}><span class="ic"><Icon name="no-color" size={16} /></span>{t('shell.tabs.menu.noColor')}</button
				>
			</Submenu>
			{#if (onchoice && !ms?.kind) || (onmergesheets && !many)}<hr />{/if}
			{#if onchoice && !ms?.kind}
				<!-- Onglet à choix : une colonne d'un tableau de la feuille choisit ce qu'elle montre. -->
				<Submenu label={t('shell.tabs.menu.chooseBy')} disabled={many} title={many ? t('shell.tabs.one') : t('shell.tabs.menu.chooseByTooltip')}>
					{#snippet icon()}<span class="ic"><Icon name="filter" size={16} /></span>{/snippet}
					{#if candidates === null}
						<div class="menu-note">{t('shell.tabs.menu.readingColumns')}</div>
					{:else if candidates.length === 0}
						<div class="menu-note">{t('shell.tabs.menu.noColumns')}</div>
					{:else}
						{#each candidates as c (c.table + '\u0000' + c.column)}
							<button
								role="menuitem"
								title={t('shell.tabs.menu.columnTooltip', { column: c.column, table: c.table, n: c.values })}
								onclick={() => {
									const id = m.id;
									closeMenu();
									onchoice?.by(id, c.table, c.column);
								}}
								><span class="ic">{#if ms?.choice && !ms.choice.formula && ms.choice.name.toLowerCase() === c.column.toLowerCase()}●{/if}</span><span class="cname">{c.column}</span><span class="menu-hint">{c.table}</span></button
							>
						{/each}
					{/if}
					<hr />
					<button
						role="menuitem"
						title={t('shell.tabs.menu.formulaTooltip')}
						onclick={() => {
							const id = m.id;
							const f = ms?.choice?.formula;
							closeMenu();
							byFormula = { id, name: f ? (ms?.choice?.name ?? '') : '', formula: f ?? '' };
						}}
						><span class="ic">{#if ms?.choice?.formula}●{/if}</span><span class="cname">{t('shell.tabs.menu.formula')}</span>{#if ms?.choice?.formula}<span class="menu-hint">{clip(ms.choice.formula)}</span>{/if}</button
					>
					{#if ms?.choice}
						<hr />
						<button
							role="menuitem"
							onclick={() => {
								const id = m.id;
								closeMenu();
								onchoice?.clear(id);
							}}><span class="ic"></span>{t('shell.tabs.menu.stopChoosing', { name: clip(ms.choice.name) })}</button
						>
					{/if}
				</Submenu>
			{/if}
			{#if onmergesheets && !many}{@render mergeItem()}{/if}
			<hr />
			<button
				role="menuitem"
				disabled={!onprotect || many}
				title={many ? t('shell.tabs.one') : undefined}
				onclick={() => {
					const id = m.id;
					closeMenu();
					onprotect?.(id);
				}}
				><span class="ic"><Icon name={ms?.protected ? 'unlock' : 'lock-sheet'} size={16} /></span>{ms?.protected ? t('shell.tabs.menu.unprotect') : t('shell.tabs.menu.protect')}</button
			>
			<button
				role="menuitem"
				disabled={!onhide || shown.length <= (many ? group?.size ?? 1 : 1)}
				title={shown.length <= (many ? group?.size ?? 1 : 1) ? t('shell.tabs.menu.keepOne') : undefined}
				onclick={() => {
					const id = m.id;
					const ids = many ? [...(group ?? [])] : null;
					closeMenu();
					if (ids) {
						picked = new Set();
						onhidemany?.(ids);
					} else onhide?.(id, true);
				}}><span class="ic"><Icon name="hide" size={16} /></span>{many ? t('shell.tabs.menu.hideMany', { n: group?.size ?? 0 }) : t('shell.tabs.menu.hide')}</button
			>
			<button
				role="menuitem"
				disabled={!onhide || hiddenSheets.length === 0}
				onclick={() => {
					closeMenu();
					unhiding = hiddenSheets[0]?.id ?? null;
				}}><span class="ic"><Icon name="show" size={16} /></span>{t('shell.tabs.menu.unhide')}</button
			>
			<hr />
			{#if many}
				<button
					role="menuitem"
					disabled={(group?.size ?? 0) >= shown.length || !ondeletemany}
					onclick={() => {
						const ids = [...(group ?? [])];
						closeMenu();
						picked = new Set();
						ondeletemany?.(ids);
					}}><span class="ic"><Icon name="trash" size={16} /></span>{t('shell.tabs.menu.deleteMany', { n: group?.size ?? 0 })}</button
				>
			{:else}
				<button
					role="menuitem"
					disabled={shown.length <= 1 || !ondelete}
				onclick={() => {
					const id = m.id;
					closeMenu();
					ondelete?.(id);
				}}><span class="ic"><Icon name="trash" size={16} /></span>{t('shell.tabs.menu.delete')}</button
				>
			{/if}
		</div>
	{/if}
	{#if choiceMenu}
		{@const cm = choiceMenu}
		{@const cs = sheets.find((x) => x.id === cm.id)}
		{#if cs?.choice}
			{@const ch = cs.choice}
			<div class="menu choice-menu" role="menu" aria-label={t('shell.tabs.choice.aria', { name: cs.name })} style:left="{cm.x}px" use:dismiss={() => (choiceMenu = null)}>
				<div class="menu-title">{ch.name}</div>
				<button role="menuitemradio" aria-checked={ch.all} use:focus onclick={() => choose(cm.id, '', true)}><span class="ic">{ch.all ? '●' : ''}</span>{allLabel(ch.name)}</button>
				{#if cm.values === null}
					<div class="menu-note">{t('shell.tabs.choice.reading')}</div>
				{:else}
					<div class="choice-values">
						{#each cm.values as v (v.text)}
							<button
								role="menuitemradio"
								aria-checked={!ch.all && ch.value === v.text}
								title={v.count ? t('shell.tabs.choice.rows', { n: v.count }) : t('shell.tabs.choice.noRows')}
								onclick={() => choose(cm.id, v.text, false)}
								><span class="ic">{!ch.all && ch.value === v.text ? '●' : ''}</span>{v.text}{#if !v.count}<span class="menu-hint">{t('shell.tabs.choice.empty')}</span>{/if}</button
							>
						{/each}
					</div>
				{/if}
				{#if !ch.formula}
					<hr />
					<button role="menuitem" disabled={cm.values === null} onclick={() => startAdding(cm.id)}><span class="ic"><Icon name="plus" size={16} /></span>{newLabel(ch.name)}</button>
				{:else}
					<div class="menu-note" title={ch.formula}>{t('shell.tabs.choice.computed', { formula: clip(ch.formula) })}</div>
				{/if}
			</div>
		{/if}
	{/if}
</div>

{#if byFormula && onchoice}
	{@const bf = byFormula}
	{@const actions = onchoice}
	<RowFormulaDialog
		title={t('shell.tabs.byFormula.title')}
		intro={t('shell.tabs.byFormula.intro')}
		nameLabel={t('shell.tabs.byFormula.name')}
		name={bf.name}
		formula={bf.formula}
		guess={!bf.name}
		follow={t('shell.tabs.byFormula.follow')}
		help={actions.formulaHelp?.(bf.id)}
		preview={(f) => actions.preview(bf.id, f)}
		onok={async (name, f) => {
			await actions.byFormula(bf.id, name, f);
			byFormula = null;
		}}
		oncancel={() => (byFormula = null)}
	/>
{/if}

{#if adding}
	{@const ad = adding}
	<div class="modal-back" role="presentation" use:backdrop={() => (adding = null)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label={newLabel(ad.name)} tabindex="-1" use:modal onkeydown={(e) => dialogKeys(e, confirmAdding, () => (adding = null))}>
			<h2>{newLabel(ad.name).replace('…', '')}</h2>
			<p>{t('shell.tabs.adding.intro', { name: ad.name })}</p>
			<label for="choice-new">{t('shell.tabs.adding.label', { name: ad.name })}</label>
			<input id="choice-new" bind:value={adding.draft} use:focus onfocus={(e) => e.currentTarget.select()} />
			<div class="actions">
				<button class="primary" disabled={!ad.draft.trim()} onclick={confirmAdding}>{t('common.ok')}</button>
				<button onclick={() => (adding = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if moving}
	{@const mv = moving}
	<div class="modal-back" role="presentation" use:backdrop={() => (moving = null)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.tabs.move.title')} tabindex="-1" use:modal onkeydown={(e) => dialogKeys(e, confirmMove, () => (moving = null))}>
			<h2>{t('shell.tabs.move.title')}</h2>
			<p>{t('shell.tabs.move.sheet', { name: sheets.find((x) => x.id === mv.id)?.name ?? '' })}</p>
			<label for="move-before">{t('shell.tabs.move.before')}</label>
			<select id="move-before" size="8" bind:value={moving.before} use:focus>
				{#each sheets as s (s.id)}
					{#if s.id !== mv.id || moving.copy}
						<option value={s.id}>{s.hidden ? t('shell.tabs.move.hidden', { name: s.name }) : s.name}</option>
					{/if}
				{/each}
				<option value={null}>{t('shell.tabs.move.last')}</option>
			</select>
			<label class="check"><input type="checkbox" bind:checked={moving.copy} disabled={!oncopy || !onmove} /> {t('shell.tabs.move.copy')}</label>
			<div class="actions">
				<button class="primary" onclick={confirmMove}>{t('common.ok')}</button>
				<button onclick={() => (moving = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

{#if listing}
	<div class="modal-back" role="presentation" use:backdrop={() => (listing = false)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.tabs.all.title')} tabindex="-1" use:modal onkeydown={(e) => dialogKeys(e, () => (listing = false), () => (listing = false))} use:focus>
			<h2>{t('shell.tabs.all.title')}</h2>
			<ul class="all-sheets">
				{#each shown as s (s.id)}
					<li>
						<button
							class:on={s.id === active}
							style:--tab-color={s.color || null}
							onclick={() => {
								listing = false;
								onselect(s.id);
							}}
							>{#if s.protected || locked}<span class="lock"><Icon name="lock" size={12} /></span>{/if}{s.name}</button
						>
					</li>
				{/each}
			</ul>
			<div class="actions">
				<button onclick={() => (listing = false)}>{t('common.close')}</button>
			</div>
		</div>
	</div>
{/if}

{#if unhiding !== null}
	<div class="modal-back" role="presentation" use:backdrop={() => (unhiding = null)}>
		<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.tabs.unhide.title')} tabindex="-1" use:modal onkeydown={(e) => dialogKeys(e, confirmUnhide, () => (unhiding = null))}>
			<h2>{t('shell.tabs.unhide.title')}</h2>
			<label for="unhide">{t('shell.tabs.unhide.label')}</label>
			<select id="unhide" size="6" bind:value={unhiding} ondblclick={confirmUnhide} use:focus>
				{#each hiddenSheets as s (s.id)}
					<option value={s.id}>{s.name}</option>
				{/each}
			</select>
			<div class="actions">
				<button class="primary" onclick={confirmUnhide}>{t('common.ok')}</button>
				<button onclick={() => (unhiding = null)}>{t('common.cancel')}</button>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Onglet d'une sélection de plusieurs feuilles (Ctrl+clic, Maj+clic). */
	.sheet.picked:not(.on) {
		background: #e3efe7;
		box-shadow: inset 0 -2px 0 #217346;
	}
	/* Onglets semblables : un même trait de couleur sous chacun ; la puce survolée les fait ressortir. */
	.sheet.grouped,
	.sheet.grouped:hover:not(.on):not(.dragging) {
		box-shadow: inset 0 -3px 0 var(--group-color);
	}
	.sheet.grouped.hot {
		background: color-mix(in srgb, var(--group-color) 16%, transparent);
		box-shadow: inset 0 -4px 0 var(--group-color);
	}
	.chip {
		flex: none;
		align-self: center;
		margin: 0 4px;
		padding: 1px 8px;
		border: 1px dashed var(--group-color);
		border-radius: 10px;
		background: #fff;
		color: var(--group-color);
		font: inherit;
		font-size: 12px;
		white-space: nowrap;
		cursor: pointer;
	}
	.chip:hover,
	.chip:focus-visible {
		background: color-mix(in srgb, var(--group-color) 10%, #fff);
	}
	.hbar {
		margin-left: auto;
		flex: 0 1 38%;
		/* Largeur minimale : le quart de la barre, 150 px au moins (au-delà, des onglets se cachent derrière « … »). */
		min-width: max(25%, 150px);
		overflow-x: auto;
		overflow-y: hidden;
		align-self: center;
	}
	/* Barre dessinée sur fond transparent (celle de la feuille a le même style) ; le curseur s'épaissit au survol. */
	.hbar::-webkit-scrollbar {
		height: 12px;
		background: transparent;
	}
	.hbar::-webkit-scrollbar-track {
		background: transparent;
	}
	.hbar::-webkit-scrollbar-thumb {
		min-width: 32px;
		border: 3px solid transparent;
		border-radius: 6px;
		background: rgba(60, 64, 67, 0.35) padding-box;
	}
	.hbar::-webkit-scrollbar-thumb:hover {
		border-width: 2px;
		background-color: rgba(60, 64, 67, 0.55);
	}
	.hbar::-webkit-scrollbar-thumb:active {
		border-width: 2px;
		background-color: rgba(60, 64, 67, 0.65);
	}
	.sheet-tabs {
		position: relative;
		display: flex;
		align-items: stretch;
		height: 28px;
		background: #f3f4f6;
		border-top: 1px solid #d4d7db;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		user-select: none;
	}
	/* Onglets des feuilles : dans la barre (souris), ou bande qui défile au doigt. */
	.strip {
		display: contents;
	}
	.strip.scroll {
		display: flex;
		flex: 0 1 auto;
		min-width: 0;
		overflow-x: auto;
		overflow-y: hidden;
		scrollbar-width: none;
		overscroll-behavior-x: contain;
	}
	.strip.scroll::-webkit-scrollbar {
		display: none;
	}
	.nav {
		display: flex;
		align-items: center;
		padding: 0 6px;
		gap: 2px;
	}
	.nav button,
	.add {
		background: none;
		border: none;
		color: #4b5563;
		font: inherit;
		width: 24px;
		cursor: pointer;
	}
	.more {
		background: none;
		border: none;
		border-right: 1px solid #d4d7db;
		color: #4b5563;
		font: inherit;
		font-weight: 600;
		width: 30px;
		cursor: pointer;
	}
	.more:hover {
		background: #e5e7eb;
	}
	.all-sheets {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: 60vh;
		overflow-y: auto;
		border: 1px solid #e3e5e8;
	}
	.all-sheets button {
		display: block;
		width: 100%;
		text-align: left;
		font: inherit;
		padding: 5px 10px;
		background: none;
		border: none;
		border-left: 3px solid var(--tab-color, transparent);
		cursor: pointer;
	}
	.all-sheets button:hover {
		background: #eef5f0;
	}
	.all-sheets button.on {
		font-weight: 600;
		color: #217346;
		background: #e3efe7;
	}
	.nav button:disabled,
	.add:disabled {
		color: #c0c4c9;
		cursor: default;
	}
	.sheet {
		font: inherit;
		/* Un nom long ou à espaces reste sur une ligne (« Ventes mensuelles »). */
		white-space: nowrap;
		flex: none;
		padding: 0 18px;
		background: none;
		border: none;
		border-right: 1px solid #d4d7db;
		color: #374151;
		cursor: pointer;
		transition:
			background-color 0.12s,
			color 0.12s,
			box-shadow 0.12s;
	}
	/* Survol : fond éclairci et trait sous l'onglet, comme Excel. */
	.sheet:hover:not(.on):not(.dragging) {
		background: #e8ebee;
		color: #111827;
		box-shadow: inset 0 -2px 0 #a7c4b2;
	}
	.sheet.colored:not(.on):hover {
		background: var(--tab-color);
		filter: brightness(1.06);
		box-shadow: inset 0 -2px 0 rgba(0, 0, 0, 0.25);
	}
	.sheet.on:hover {
		background: #f7faf8;
	}
	/* Largeur du texte en gras réservée : passer en gras (onglet actif) ne
	   change pas la largeur, les autres onglets ne bougent pas. */
	.sheet::after {
		content: attr(data-label);
		display: block;
		height: 0;
		overflow: hidden;
		visibility: hidden;
		font-weight: 600;
	}
	.sheet.on {
		background: #fff;
		color: #217346;
		font-weight: 600;
		box-shadow: inset 0 -3px 0 #217346;
	}
	/* Couleur d'onglet : fond de l'onglet inactif, soulignement de l'actif. */
	.sheet.colored:not(.on) {
		background: var(--tab-color);
		color: var(--tab-text);
	}
	.sheet.colored.on {
		box-shadow: inset 0 -4px 0 var(--tab-color);
	}
	.sheet.dragging {
		opacity: 0.6;
		cursor: grabbing;
	}
	.drop-mark {
		position: absolute;
		top: 3px;
		bottom: 3px;
		width: 2px;
		background: #217346;
		pointer-events: none;
	}
	.rename {
		font: inherit;
		width: 120px;
		border: 1px solid #217346;
		outline: none;
		padding: 0 6px;
	}
	.add {
		font-size: 16px;
	}
	.menu {
		position: absolute;
		bottom: 28px;
		z-index: 10;
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
		padding: 4px 0;
		min-width: 140px;
		max-width: 320px; /* MENU_MAX_WIDTH */
		--menu-item-padding: 6px 14px 6px 10px; /* entrées des sous-menus (Submenu) */
	}
	.menu-title {
		padding: 4px 14px 4px 10px;
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
		text-align: left;
		padding: 6px 14px 6px 10px;
		background: none;
		border: none;
		font: inherit;
		white-space: nowrap;
		cursor: pointer;
		--icon-accent: #217346;
	}
	/* Colonne d'icônes de largeur fixe : les libellés restent alignés. */
	.ic {
		flex: none;
		display: flex;
		width: 16px;
		color: #4b5563;
	}
	.menu button:disabled .ic {
		color: inherit;
		--icon-accent: currentColor;
	}
	.menu button:hover:not(:disabled) {
		background: #e3efe7;
	}
	.menu button:disabled {
		color: #9aa0a6;
	}
	.menu hr {
		border: none;
		border-top: 1px solid #e5e7eb;
		margin: 4px 0;
	}
	.swatches {
		display: grid;
		grid-template-columns: repeat(5, 20px);
		gap: 6px;
		padding: 6px 12px;
	}
	.menu .swatch {
		width: 20px;
		height: 20px;
		padding: 0;
		border: 1px solid rgba(0, 0, 0, 0.2);
	}
	.menu .swatch:hover,
	.menu .swatch.sel {
		outline: 2px solid #217346;
		outline-offset: 1px;
	}
	.modal-back {
		position: fixed;
		inset: 0;
		z-index: 40;
		background: rgba(0, 0, 0, 0.2);
		display: flex;
		align-items: center;
		justify-content: center;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
		padding: 16px 18px;
		min-width: 300px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.dialog h2 {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.dialog p {
		margin: 0;
	}
	.dialog select {
		font: inherit;
		width: 100%;
	}
	.dialog .check {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.dialog .actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 4px;
	}
	.dialog .actions button {
		font: inherit;
		padding: 4px 16px;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 2px;
		cursor: pointer;
	}
	.dialog .actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	/* Onglet à choix : « Ventes · 2025 ▾ » d'un seul tenant. La partie droite,
	   sur la même ligne de texte que le nom, occupe toute la hauteur de
	   l'onglet (rembourrage vertical débordant, coupé par l'onglet) jusqu'à son
	   bord droit ; un trait vertical la sépare du nom, avec un point médian
	   posé en son milieu, pour qu'on lise un seul onglet. */
	.sheet.choice {
		overflow: hidden;
	}
	.choice-arrow {
		position: relative;
		display: inline;
		margin: -20px -18px -20px 10px;
		padding: 20px 10px 20px 10px;
		border-left: 1px solid rgba(0, 0, 0, 0.18);
		cursor: pointer;
	}
	/* Point dessiné (pas un caractère) : centré au pixel près sur le trait,
	   qui occupe la colonne -1px..0 de la boîte (bordure gauche). */
	.choice-arrow::before {
		content: '';
		position: absolute;
		left: -0.5px;
		top: 50%;
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: currentColor;
		transform: translate(-50%, -50%);
		pointer-events: none;
	}
	.choice-arrow:hover:not(.off) {
		background: rgba(0, 0, 0, 0.08);
	}
	.sheet.choice.on .choice-arrow {
		color: #217346;
	}
	.choice-menu {
		min-width: 180px;
	}
	.choice-values {
		max-height: 50vh;
		overflow-y: auto;
	}
	.menu-note {
		padding: 6px 14px 6px 10px;
		color: #6b7280;
		font-style: italic;
		white-space: normal;
		max-width: 260px;
	}
	.menu-hint {
		margin-left: auto;
		padding-left: 12px;
		color: #9aa0a6;
		font-size: 11px;
		/* Un long nom de tableau ne déborde pas du menu (320 px au plus) : « … », nom complet en info-bulle. */
		min-width: 0;
		max-width: 50%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex-shrink: 1;
	}
	.cname {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex-shrink: 1;
	}
	.dialog input {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #d4d7db;
	}
	.kind {
		display: inline-flex;
		vertical-align: -1px;
		margin-right: 4px;
		--icon-accent: #217346;
	}
	.lock {
		display: inline-flex;
		vertical-align: -1px;
		margin-right: 4px;
		color: #6b7280;
		--icon-accent: #b45309;
	}
	/* Doigt : barre plus haute (cibles d'au moins 32 px), au-dessus de la barre d'état. */
	@media (pointer: coarse) {
		.sheet-tabs {
			height: 36px;
		}
		.nav button,
		.add {
			width: 32px;
		}
		.menu {
			bottom: 36px;
		}
	}
</style>
