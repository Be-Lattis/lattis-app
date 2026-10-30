// Rechercher et remplacer (Ctrl+F, Ctrl+H) de la feuille : état de la barre,
// recherche dans la zone choisie, passage d'une cellule trouvée à l'autre,
// remplacement. Sorti de Sheet.svelte (docs/decoupage-sheet.md).
import { flushSync } from 'svelte';
import { type MergeRange } from '../sheetfmt';
import { mergeRect } from '../mergesel';
import { findKey, finderAction, foundCell, hitCount, inZone, replacedLabel, type Zone } from '../findzone';
import { defaultFindOpts, nextFound, optsKey, replaceLook, replaceSummary, searchAll, searchQuery, type FindOpts, type FoundCell } from '../findopts';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetData, type SheetSource } from './model';

/** Ce que createFind lit et modifie dans Sheet.svelte. */
export interface FindHost {
	finder: { replace: boolean; text: string; by: string; count: string; info: string; zone: Zone | null; at: { row: number; col: number; } | null; opts: FindOpts; more: boolean; list: FoundCell[] | null; total: number; busy: boolean; current: number; } | null;
	readonly schedule: () => void;
	readonly active: { row: number; col: number; };
	readonly editing: boolean;
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	readonly data: SheetData;
	readonly searchZone: () => Zone | null;
	readonly source: SheetSource;
	readonly findInput: HTMLInputElement | undefined;
	readonly wrap: HTMLDivElement;
	readonly replaceInput: HTMLInputElement | undefined;
	matchKey: string;
	matches: FoundCell[];
	readonly sheetName: string;
	readonly onsheet: ((sheet: number) => void) | undefined;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
}

export function createFind(h: FindHost) {
	let findOpts = defaultFindOpts();
	let findMore = false;
	let matchOrder: number[] = [];
	let matchTotal = 0;
	// Recherche la plus récente (une plus ancienne qui finit après est ignorée).
	let findGen = 0;
	// Occurrence sur une autre feuille : on y va, puis elle se sélectionne (applyData).
	let findGo: (FoundCell & { count: string }) | null = null;
	// Barre ouverte ou fermée : redessiner le surlignage des occurrences.
	$effect(() => {
		void h.finder;
		h.schedule();
	});

	/** Étendue de la cellule trouvée à repérer (toute sa fusion), ou null. */
	function foundRect(): Zone | null {
		const f = h.finder && foundCell(h.finder.at, h.finder.zone, h.active, h.editing);
		if (!f) return null;
		const g = h.boxOf(f.row, f.col);
		return g ? mergeRect(g) : { r0: f.row, r1: f.row, c0: f.col, c1: f.col };
	}

	/** Clé des occurrences cherchées : texte, zone (feuille) ou classeur, options. */
	function matchesKey(text: string, zone: Zone | null, o: FindOpts): string {
		return findKey(text, o.book ? null : zone, o.book ? -1 : (h.data.sheet ?? 0)) + optsKey(o);
	}

	/** Suit la sélection : zone affichée dans la barre, cellule trouvée oubliée hors zone. */
	function syncFinderZone() {
		if (!h.finder) return;
		const z = h.searchZone();
		const old = h.finder.zone;
		if (findKey('', z) !== findKey('', old)) {
			h.finder.zone = z;
			h.finder.count = '';
		}
		if (h.finder.at && (z ? !inZone(h.finder.at, z) : h.finder.at.row !== h.active.row || h.finder.at.col !== h.active.col)) h.finder.at = null;
	}

	/** Ouvre la barre de recherche (replace : avec le champ Remplacer par). */
	function openFinder(replace: boolean) {
		if (!h.source.find && !h.source.search) return;
		// Rendue tout de suite : le focus y est avant la frappe suivante.
		flushSync(() => {
			h.finder = {
				replace,
				text: h.finder?.text ?? '',
				by: h.finder?.by ?? '',
				count: '',
				info: '',
				zone: h.searchZone(),
				at: null,
				opts: h.finder?.opts ?? findOpts,
				more: h.finder?.more ?? findMore,
				list: h.finder?.list ?? null,
				total: h.finder?.total ?? 0,
				busy: false,
				current: -1
			};
		});
		h.findInput?.select();
		// Ouverte depuis un menu qui reprend le focus en se fermant : y revenir.
		requestAnimationFrame(() => {
			if (h.findInput && !h.findInput.closest('.finder')?.contains(document.activeElement)) h.findInput.select();
		});
	}

	/** Ferme la barre et rend la main à la grille. */
	function closeFinder() {
		if (h.finder) {
			findOpts = h.finder.opts;
			findMore = h.finder.more;
		}
		h.finder = null;
		findGen++;
		h.wrap.focus();
	}

	/** Affiche ou masque la ligne Remplacer par (le focus y va à l'ouverture). */
	function toggleReplace(open = !h.finder?.replace) {
		if (!h.finder || (!h.source.replace && !h.source.replaceAll)) return;
		const f = h.finder;
		flushSync(() => (f.replace = open));
		if (open) h.replaceInput?.select();
		else h.findInput?.focus();
	}

	/** Texte ou options changés : compteur, bilan et liste oubliés. */
	function findChanged() {
		if (!h.finder) return;
		h.finder.count = '';
		h.finder.info = '';
		h.finder.list = null;
		h.finder.current = -1;
	}

	/**
	 * Occurrences du texte cherché (relues si le texte, la zone, la feuille
	 * ou les options ont changé) ; faux si une recherche plus récente l'a
	 * remplacée. onPart : la liste au fur et à mesure (classeur).
	 */
	async function findAll(zone: Zone | null, onPart?: (hits: FoundCell[], total: number) => void): Promise<boolean> {
		if (!h.finder) return false;
		const o = h.finder.opts;
		const k = matchesKey(h.finder.text, zone, o);
		if (k === h.matchKey) return true;
		const sheet = h.data.sheet ?? 0;
		const gen = ++findGen;
		if (h.source.search) {
			const r = await searchAll(h.source.search, searchQuery(h.finder.text, sheet, zone, o), 5000, onPart, () => gen !== findGen);
			if (!r) return false;
			h.matches = r.hits;
			matchOrder = r.order;
			matchTotal = r.total;
		} else if (h.source.find) {
			const ms = await h.source.find(h.finder.text, zone);
			if (gen !== findGen) return false;
			h.matches = ms.map((m) => ({ ...m, sheet, k: 0, name: h.sheetName, text: '' }));
			matchOrder = [sheet];
			matchTotal = h.matches.length;
		}
		h.matchKey = k;
		return true;
	}

	/** Va à l'occurrence i (sur une autre feuille : elle s'y sélectionne une fois la feuille affichée). */
	function goFound(i: number, zone: Zone | null) {
		if (!h.finder) return;
		const m = h.matches[i];
		h.finder.current = i;
		h.finder.count = hitCount(i, matchTotal);
		if (m.sheet !== (h.data.sheet ?? 0)) {
			if (!h.onsheet) return;
			findGo = { ...m, count: h.finder.count };
			h.onsheet(m.sheet);
			return;
		}
		if (zone && !h.finder.opts.book) {
			h.finder.at = { row: m.row, col: m.col };
			h.ensureVisible(m.row, m.col);
			h.schedule();
		} else {
			h.moveTo(m.row, m.col);
			h.finder.at = { row: h.active.row, col: h.active.col };
		}
	}

	/** Arrivé sur la feuille d'une occurrence (applyData) : elle se sélectionne. */
	function findArrived(sheet: number) {
		const g = findGo;
		if (!g || g.sheet !== sheet) return;
		findGo = null;
		h.moveTo(g.row, g.col);
		if (h.finder) {
			h.finder.at = { row: h.active.row, col: h.active.col };
			h.finder.count = g.count;
		}
	}

	async function findNext(back = false) {
		if (!h.finder || !h.finder.text) return;
		const zone = h.searchZone();
		h.finder.zone = zone;
		if (!(await findAll(zone)) || !h.finder) return;
		h.finder.info = '';
		if (h.matches.length === 0) {
			h.finder.at = null;
			h.finder.current = -1;
			h.finder.count = tr('grid.find.none');
			h.schedule();
			return;
		}
		// Dans la sélection : à partir de la dernière cellule trouvée (la
		// sélection ne bouge pas) ; sur la feuille ou le classeur : de la
		// cellule active.
		const k = matchOrder.indexOf(h.data.sheet ?? 0);
		const inSel = zone && !h.finder.opts.book;
		const at = inSel ? (h.finder.at && inZone(h.finder.at, zone) ? h.finder.at : null) : h.active;
		const i = nextFound(h.matches, at && k >= 0 ? { k, row: at.row, col: at.col } : null, back, h.finder.opts.byCols);
		goFound(i, zone);
	}

	/** Rechercher tout : la liste des cellules trouvées, feuille après feuille. */
	async function listAll() {
		if (!h.finder || !h.finder.text) return;
		const f = h.finder;
		const zone = h.searchZone();
		f.zone = zone;
		f.info = '';
		h.matchKey = '';
		f.busy = true;
		f.list = [];
		const done = await findAll(zone, (hits, total) => {
			f.list = [...hits];
			f.total = total;
		});
		if (!done || h.finder !== f) return;
		f.busy = false;
		f.list = h.matches;
		f.total = matchTotal;
		f.current = -1;
		f.count = h.matches.length ? '' : tr('grid.find.none');
	}

	/** Remplace la cellule trouvée (si elle contient le texte), puis passe à la suivante (Excel). */
	async function replaceOne() {
		if (!h.finder || !h.finder.text || (!h.source.replace && !h.source.replaceAll)) return;
		const zone = h.searchZone();
		if (!(await findAll(zone)) || !h.finder) return;
		const sheet = h.data.sheet ?? 0;
		const at = zone && !h.finder.opts.book ? h.finder.at : h.active;
		if (at && inZone(at, h.finder.opts.book ? null : zone) && h.matches.some((m) => m.sheet === sheet && m.row === at.row && m.col === at.col)) {
			const cell = { r0: at.row, c0: at.col, r1: at.row, c1: at.col };
			try {
				if (h.source.replaceAll) {
					const r = await h.source.replaceAll(searchQuery(h.finder.text, sheet, cell, { ...replaceLook(h.finder.opts), book: false }), h.finder.by);
					if (h.finder && r.skipped) h.finder.info = replaceSummary(r, 'sheet');
				} else await h.source.replace!(h.finder.text, h.finder.by, cell);
			} catch (e) {
				if (h.finder) h.finder.info = errorText(e);
				return;
			}
			h.matchKey = '';
		}
		await findNext();
	}

	async function replaceAll() {
		if (!h.finder || !h.finder.text || (!h.source.replace && !h.source.replaceAll)) return;
		const f = h.finder;
		const { text, by, opts } = f;
		const zone = h.searchZone();
		try {
			if (h.source.replaceAll) {
				const r = await h.source.replaceAll(searchQuery(text, h.data.sheet ?? 0, zone, replaceLook(opts)), by);
				if (h.finder !== f) return;
				f.info = replaceSummary(r, opts.book ? 'book' : zone ? 'selection' : 'sheet');
			} else {
				const n = await h.source.replace!(text, by, zone);
				if (h.finder !== f) return;
				f.info = replacedLabel(n, !!zone);
			}
			f.count = '';
			f.at = null;
			h.matchKey = '';
			if (f.list) {
				const info = f.info;
				await listAll();
				f.info = info;
			}
		} catch (e) {
			if (h.finder) h.finder.info = errorText(e);
		}
	}

	/** Touches dans toute la barre (champs et boutons), voir finderAction. */
	function onFinderKey(e: KeyboardEvent) {
		e.stopPropagation();
		const a = finderAction(e);
		// Entrée sur un bouton, une liste ou une case : il agit, pas la recherche.
		if (!a || (e.key === 'Enter' && (e.target instanceof HTMLButtonElement || e.target instanceof HTMLSelectElement))) return;
		e.preventDefault();
		if (a === 'close') closeFinder();
		else if (a === 'next' || a === 'prev') findNext(a === 'prev');
		else if (a === 'focusFind') h.findInput?.select();
		else if (a === 'openReplace') toggleReplace(true);
	}

	return {
		openFinder,
		foundRect,
		matchesKey,
		syncFinderZone,
		closeFinder,
		toggleReplace,
		findChanged,
		goFound,
		findArrived,
		findNext,
		listAll,
		replaceOne,
		replaceAll,
		onFinderKey
	};
}
