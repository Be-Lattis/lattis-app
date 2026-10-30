// Audit de formules (flèches des antécédents et des dépendants), barre de
// formule et annonce de la cellule active. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { untrack } from 'svelte';
import { arrowNotes } from '../auditarrows';
import { type CellRange } from '../mergesel';
import { explainError } from '../errors';
import { splitGroup } from '../twolevel';
import { panelAtCell } from '../pivotfields';
import { brokenPivotAt } from '../pivotzone';
import SheetButtons from '../SheetButtons.svelte';
import SheetObjects from '../SheetObjects.svelte';
import { selectionLabel, type CellSpeech } from '../a11y';
import { isErrorValue } from '../format';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetData, type SheetSource, type Link, type Entry } from './model';

/** Ce que createAudit lit et modifie dans Sheet.svelte. */
export interface AuditHost {
	readonly active: { row: number; col: number; };
	readonly anchor: { row: number; col: number; };
	links: Link[];
	readonly schedule: () => void;
	readonly source: SheetSource;
	readonly selection: () => CellRange;
	readonly visibleRows: () => { first: number; last: number; };
	readonly data: SheetData;
	readonly closeMenus: () => void;
	selChart: number | null;
	panel: { kind: 'pivot' | 'chart'; id: number } | null;
	readonly buttonsLayer: ReturnType<typeof SheetButtons> | undefined;
	readonly objectsLayer: ReturnType<typeof SheetObjects> | undefined;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly shown: Map<string, { text: string; error: boolean; }>;
	readonly key: (r: number, c: number) => string;
	readonly cells: Map<string, string>;
	nameBox: string;
	readonly cellName: (r: number, c: number) => string;
	message: string;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly pivots: SheetPivot[];
	pivotPanelAt: string;
	readonly openPivotOnLoad: boolean;
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly singleCol: (e: Entry, i: number) => boolean;
	readonly singleSelection: () => boolean;
	readonly inTable: (e: Entry, r: number, c: number) => boolean;
	tableBox: { name: string; col: string } | null;
	readonly dragging: boolean;
	readonly headerDrag: 'rows' | 'cols' | null;
	readonly fxEditing: boolean;
	fxRO: string;
	readonly readOnlyReason: (r: number, c: number) => string;
	fxText: string;
	readonly editText: (r: number, c: number) => string;
	readonly addrA1: (r: number, c: number) => string;
	readonly shownText: (r: number, c: number) => string;
	readonly names: Map<string, string>;
	speech: CellSpeech;
	speechSel: string;
	readonly refreshStatus: () => void;
}

export function createAudit(h: AuditHost) {
	/** Cellules analysées au plus par « Afficher les liens » (la sélection peut être une colonne entière). */
	const MAX_LINK_CELLS = 400;
	// Sélection dont les liens sont affichés : ils partent dès qu'elle change.
	let linksOf = '';
	const selKey = () => `${sheetIdOf()}:${h.active.row},${h.active.col}:${h.anchor.row},${h.anchor.col}`;
	$effect(() => {
		const k = selKey();
		if (k !== linksOf && untrack(() => h.links.length)) {
			h.links = [];
			h.schedule();
		}
	});

	/** Formules › Afficher les liens (toute la sélection) ; clear : les retire (Échap). */
	async function auditActive(kind: 'show' | 'clear') {
		if (kind === 'clear' || !h.source.dependencies) {
			h.links = [];
			h.schedule();
			return;
		}
		const deps = h.source.dependencies;
		const { r0, r1, c0, c1 } = h.selection();
		// Lignes visibles d'abord : sur une colonne entière, ce sont celles qu'on voit.
		const { first, last } = h.visibleRows();
		const rows: number[] = [];
		for (let r = Math.max(r0, first); r <= Math.min(r1, last); r++) rows.push(r);
		for (let r = r0; r <= r1 && rows.length * (c1 - c0 + 1) < MAX_LINK_CELLS; r++) if (r < first || r > last) rows.push(r);
		const cellsToSee: { row: number; col: number }[] = [];
		for (const r of rows) for (let c = c0; c <= c1 && cellsToSee.length < MAX_LINK_CELLS; c++) cellsToSee.push({ row: r, col: c });
		const cut = (r1 - r0 + 1) * (c1 - c0 + 1) > cellsToSee.length;
		try {
			const found: Link[] = [];
			let pre = 0;
			let dep = 0;
			let away = 0;
			// Par petits groupes : le moteur n'est pas inondé de requêtes.
			for (let i = 0; i < cellsToSee.length; i += 16) {
				const part = await Promise.all(cellsToSee.slice(i, i + 16).map((p) => deps(p.row, p.col).then((d) => ({ ...p, d }))));
				for (const { row, col, d } of part) {
					if (!d.precedents.length && !d.dependents.length) continue;
					found.push({ row, col, pre: d.precedents, dep: d.dependents });
					pre += d.precedents.length;
					dep += d.dependents.length;
					away += [...d.precedents, ...d.dependents].filter((x) => x.sheet !== sheetIdOf()).length;
				}
			}
			h.links = found;
			linksOf = selKey();
			// Ligne précise sans place (masquée, clé introuvable…) : la flèche vise la colonne, la note dit pourquoi.
			const notes = arrowNotes(found.flatMap((l) => [...l.pre, ...l.dep]));
			say(
				found.length === 0
					? tr('grid.audit.none')
					: tr('grid.audit.links', { n: found.length, pre, dep }) +
							(away ? ' ' + tr('grid.audit.away', { n: away }) : '') +
							(cut ? ' · ' + tr('grid.audit.cut', { n: MAX_LINK_CELLS }) : '') +
							(notes.length ? ` · ${notes.slice(0, 2).join(' ; ')}${notes.length > 2 ? ` (+${notes.length - 2})` : ''}` : '') +
							' · ' + tr('grid.audit.clear')
			);
			h.schedule();
		} catch (err) {
			say(errorText(err));
		}
	}

	/** Feuille affichée (les cibles des autres feuilles n'ont pas de flèche). */
	function sheetIdOf(): number {
		return h.data?.sheet ?? 1;
	}

	/** Ferme les menus, dialogues et volets (changement de feuille ou de vue). */
	function closeOverlays() {
		h.closeMenus();
		h.links = [];
		h.selChart = null;
		h.panel = null;
		// Volet d'un contrôle : fermé aussi (ses réglages restent appliqués).
		h.buttonsLayer?.closeEditor();
		h.objectsLayer?.deselect();
	}

	/** Erreur de formule de la cellule (code et formule), ou null. */
	function errorAt(r: number, c: number): { code: string; formula: string } | null {
		const e = h.tableAt(r, c);
		if (e) {
			const i = c - e.t.col;
			const col = e.t.columns[i];
			if (r === e.t.row || !col.formula) return null;
			const v = e.cache.row(r - e.t.row - 1)?.cells[i];
			return isErrorValue(v) ? { code: v as string, formula: col.formula } : null;
		}
		const f = h.shown.get(h.key(r, c));
		return f?.error ? { code: f.text, formula: h.cells.get(h.key(r, c)) ?? '' } : null;
	}

	function refreshFx() {
		h.nameBox = h.cellName(h.active.row, h.active.col);
		const err = errorAt(h.active.row, h.active.col);
		if (err) h.message = explainError(err.code, err.formula, (r, c) => h.shown.get(h.key(r, c))?.text ?? h.cells.get(h.key(r, c))) || h.message;
		// Un TCD en erreur (source introuvable) ouvre aussi son volet sur la
		// cellule de son message : le supprimer ou changer sa source.
		const pv = h.pivotAt(h.active.row, h.active.col) ?? brokenPivotAt(h.pivots, h.active.row, h.active.col);
		// Le volet du TCD reste ouvert quand le TCD rétrécit sous la cellule
		// active (filtre, champ retiré) : il se ferme quand on la déplace.
		// Graphique sélectionné (même posé sur le TCD, ou relu après son
		// déplacement) : son volet reste ouvert, la cellule active n'y change rien.
		const at = `${h.active.row}:${h.active.col}`;
		const next = panelAtCell(h.panel, pv?.id, h.selChart !== null, at !== h.pivotPanelAt, h.openPivotOnLoad);
		if (next !== h.panel) h.panel = next;
		h.pivotPanelAt = at;
		const e = h.ownerAt(h.active.row, h.active.col);
		const label = (r: number, c: number) => {
			const k = e!.t.columns[c - e!.t.col];
			// Bande des groupes : le groupe, qu'on renomme ici comme une colonne.
			if (r < e!.t.row && !h.singleCol(e!, c - e!.t.col)) return splitGroup(k.name).group;
			return r === e!.t.row ? k.name : `${k.name}[${r - e!.t.row}]`;
		};
		const sel = h.selection();
		const multi = !h.singleSelection();
		if (e) {
			const inside = multi && h.inTable(e, sel.r0, sel.c0) && h.inTable(e, sel.r1, sel.c1);
			h.tableBox = { name: e.t.name, col: inside ? `${label(sel.r0, sel.c0)}:${label(sel.r1, sel.c1)}` : label(h.active.row, h.active.col) };
		} else h.tableBox = null;
		// Taille de la plage pendant le glissé seulement, comme Excel ; ensuite
		// l'adresse de la cellule active revient.
		if (multi && (h.dragging || h.headerDrag)) h.nameBox = tr('grid.nameBox.size', { rows: String(sel.r1 - sel.r0 + 1), cols: String(sel.c1 - sel.c0 + 1) });
		refreshSpeech(e, sel, multi);
		if (h.fxEditing) return;
		h.fxRO = h.readOnlyReason(h.active.row, h.active.col);
		h.fxText = h.editText(h.active.row, h.active.col);
	}

	/** Ce que le lecteur d'écran dit de la cellule active (a11y.ts). */
	function refreshSpeech(e: ReturnType<typeof h.tableAt>, sel: ReturnType<typeof h.selection>, multi: boolean) {
		const { row: r, col: c } = h.active;
		const s: CellSpeech = { addr: h.addrA1(r, c), value: h.shownText(r, c), name: h.names.get(h.key(r, c)) };
		if (e) {
			const k = e.t.columns[c - e.t.col];
			Object.assign(s, { table: e.t.name, column: k.name, header: r === e.t.row, computed: !!k.formula });
		}
		s.error = !!errorAt(r, c);
		h.speech = s;
		h.speechSel = multi ? selectionLabel(`${h.addrA1(sel.r0, sel.c0)}:${h.addrA1(sel.r1, sel.c1)}`, sel.r1 - sel.r0 + 1, sel.c1 - sel.c0 + 1) : '';
	}

	function say(m: string) {
		h.message = m;
		h.refreshStatus();
	}

	return {
		auditActive,
		closeOverlays,
		sheetIdOf,
		refreshFx,
		say
	};
}
