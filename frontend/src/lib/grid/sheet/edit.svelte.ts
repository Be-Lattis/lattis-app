// Saisie : éditeur de cellule et barre de formule (ouverture, validation,
// annulation, écriture au classeur), effacement, zone de nom. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { flushSync } from 'svelte';
import { type SyncKind } from '../cache';
import { type AxisLike } from '../anim';
import { type Clip } from '../clipboard';
import { type CellStyle } from '../style';
import { cellFont } from '../textwrap';
import { type MergeRange } from '../sheetfmt';
import { dvAlertOf, type DvAlert } from '../dvrule';
import { isSingleCell, type CellRange } from '../mergesel';
import { type Zone } from '../findzone';
import { handQuestion, keyQuestion, type KeyChoice, type KeyQuestion } from '../keychoice';
import { bandAt, bandOpen, bandRefusal } from '../hidden';
import { editMode, viewKey } from '../readonlyedit';
import { splitGroup } from '../twolevel';
import { fillBlock, insertAt, shortcutOf, timeText, todayText } from '../shortcuts';
import { pivotsCleared } from '../pivotzone';
import { handEditText, pivotHandAt } from '../pivothand';
import { signature, type Catalog, type Completion, type FunctionDoc } from '../autocomplete';
import { checkFormula, closeParens, editorWidth, isFormula, serverErrorPos, type FormulaError } from '../formulaEdit';
import { fxLeave, type FormulaHold } from '../crossref';
import { parseGoto, type GotoTable } from '../goto';
import { cellNameProblem, columnNameProblem, refusedName, tableNameProblem, type NameProblem } from '#lib/ui/names.js';
import { nameAlertOpen, refuseName } from '#lib/ui/nameAlert.js';
import { alignment } from '../format';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetTable, type SheetSource, type TextField, type Range, type Entry } from './model';

/** Ce que createEdit lit et modifie dans Sheet.svelte. */
export interface EditHost {
	readonly locked: string;
	readonly ownerAt: (r: number, c: number) => Entry | undefined;
	readonly layoutLocked: string;
	readonly lockedRowAt: (r: number, c: number) => boolean;
	readonly NOT_YOURS: string;
	readonly pivotAt: (r: number, c: number) => SheetPivot | undefined;
	readonly PIVOT_READONLY: string;
	readonly queryAt: (r: number, c: number) => Entry | undefined;
	readonly QUERY_READONLY: string;
	readonly structure: boolean;
	readonly cellInfo: (r: number, c: number) => { e: Entry; i: number; kind: "header"; value: string; n?: undefined; row?: undefined; } | { e: undefined; kind: "free"; value: string; i?: undefined; n?: undefined; row?: undefined; } | { e: Entry; i: number; n: number; row: { id: number; cells: unknown[]; text: string[]; bad?: Map<number, string>; sty?: Map<number, string>; marks?: Set<number>; links?: Map<number, string>; sync?: Map<number, SyncKind>; locked?: boolean; } | undefined; kind: "data"; value: string; };
	readonly shownValue: (r: number, c: number) => string;
	active: { row: number; col: number; };
	readonly say: (m: string) => void;
	readonly entries: Entry[];
	readonly selection: () => CellRange;
	anchor: { row: number; col: number; };
	editByTyping: boolean;
	editValueText: string;
	clipRange: Clip | null;
	readonly schedule: () => void;
	editRO: string;
	editing: boolean;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly input: HTMLTextAreaElement | undefined;
	message: string;
	editErr: FormulaError | null;
	completion: (Completion & { sel: number; where: 'cell' | 'fx' }) | null;
	sig: { fn: FunctionDoc; index: number; where: 'cell' | 'fx' } | null;
	readonly refreshFx: () => void;
	readonly wrap: HTMLDivElement;
	readonly spills: { row: number; col: number; rows: string[][] }[];
	readonly screenX: (c: number) => number;
	readonly screenY: (r: number) => number;
	readonly boxOf: (r: number, c: number) => MergeRange | undefined;
	readonly textWidth: (s: string, bold?: boolean) => number;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly cells: Map<string, string>;
	readonly key: (r: number, c: number) => string;
	readonly mergeOf: (r: number, c: number) => MergeRange | undefined;
	readonly cax: () => AxisLike;
	readonly viewW: number;
	editorH: number;
	readonly rspan: (r0: number, r1: number) => number;
	readonly rh: (r: number) => number;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly source: SheetSource;
	writing: Promise<void>;
	readonly isNumeric: (s: string) => boolean;
	typed: { row: number; col: number; t: number; };
	lastEditMs: number;
	columnFlash: { table: number; name: string; t: number; done: Set<string> } | null;
	readonly autoLinkAfter: (row: number, col: number, raw: string) => void;
	dvAlert: (DvAlert & { row: number; col: number; raw: string; keep: boolean }) | null;
	keyDlg: (KeyQuestion & { row: number; col: number; raw: string; keep: boolean; choice: KeyChoice }) | null;
	handDlg: { table: string; columns: string[]; choice: string; write?: { row: number; col: number; raw: string; keep: boolean }; t?: SheetTable } | null;
	readonly refreshStatus: () => void;
	readonly refusal: (row: number, col: number, text: string) => { message: string; } | { value: string; };
	lastRepeat: (() => void) | null;
	fxText: string;
	lastRef: { el: TextField; start: number; end: number; value: string } | null;
	fxEditing: boolean;
	readonly boxes: MergeRange[];
	readonly onFormulaInput: (el: TextField, where: "cell" | "fx") => void;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly catalog: Catalog | undefined;
	readonly queryIn: (r0: number, c0: number, r1: number, c1: number, whole?: boolean) => Entry | undefined;
	readonly pivots: SheetPivot[];
	readonly PIVOT_PARTIAL: string;
	panel: { kind: 'pivot' | 'chart'; id: number } | null;
	readonly inTable: (e: Entry, r: number, c: number) => boolean;
	readonly enterMove: (back: boolean) => void;
	readonly tabMove: (back: boolean) => void;
	readonly completionKey: (e: KeyboardEvent, el: TextField, set: (v: string) => void) => boolean;
	readonly fxInput: HTMLInputElement | undefined;
	readonly hold: FormulaHold | null;
	readonly finishHold: (finish: "commit" | "cancel") => void;
	readonly fxLock: string;
	readonly tabPress: boolean;
	readonly selectRange: (r0: number, c0: number, r1: number, c1: number) => void;
	readonly tableBox: { name: string; col: string } | null;
	readonly singleCol: (e: Entry, i: number) => boolean;
	readonly nameBox: string;
	readonly names: Map<string, string>;
	readonly nameInput: HTMLInputElement | undefined;
}

export function createEdit(h: EditHost) {
	// --- Édition ------------------------------------------------------------

	/**
	 * Pourquoi la cellule ne se modifie pas ('' si elle se modifie) : feuille
	 * protégée, tableau publié figé, ligne d'un autre membre, tableau croisé,
	 * tableau chargé par une requête (hors colonnes saisies). Elle s'ouvre
	 * alors en lecture seule (double clic, F2, barre de formule), pour copier.
	 */
	function readOnlyReason(r: number, c: number): string {
		if (h.locked) return h.locked;
		// Tableau publié en lecture seule (classeur partagé figé, hors ligne).
		const ro = h.ownerAt(r, c)?.t.readOnly;
		if (ro) return ro;
		// Cellule libre d'un classeur collaboratif sans droit de conception
		// (la ligne juste sous un tableau l'agrandit : c'est une saisie).
		if (h.layoutLocked && !h.ownerAt(r, c) && !h.ownerAt(r - 1, c)) return h.layoutLocked;
		// Participant (classeur collaboratif) : les lignes des autres membres.
		if (h.lockedRowAt(r, c)) return h.NOT_YOURS;
		// Tableau croisé : seules ses colonnes saisies à droite se modifient.
		const pv = h.pivotAt(r, c);
		if (pv && pivotHandAt(pv, r, c) < 0) return h.PIVOT_READONLY;
		if (h.queryAt(r, c)) return h.QUERY_READONLY;
		return '';
	}

	/**
	 * Pourquoi la structure du tableau t ne se change pas ('' si elle se
	 * change) : feuille protégée, structure protégée ou classeur partagé sans
	 * droit de conception (layoutLocked), tableau publié en lecture seule. Le
	 * moteur refuserait ; les menus grisent ou retirent ces actions.
	 */
	function designWhy(t: { readOnly?: string }): string {
		if (h.locked) return h.locked;
		if (h.structure) return h.layoutLocked || tr('grid.protect.structure');
		return t.readOnly ?? '';
	}

	/** Texte de la cellule dans l'éditeur et la barre de formule (un tableau croisé : sa valeur affichée). */
	function editText(r: number, c: number): string {
		const v = h.cellInfo(r, c).value;
		const p = v === '' ? h.pivotAt(r, c) : undefined;
		// Colonne saisie à formule : sa formule (pivothand.ts).
		return p ? handEditText(p, r, c, h.shownValue(r, c)) : v;
	}

	function startEdit(byTyping: boolean, initial?: string) {
		const reason = readOnlyReason(h.active.row, h.active.col);
		const mode = editMode(reason, byTyping);
		if (mode === 'refuse') {
			h.say(reason);
			return;
		}
		if (mode === 'view') {
			openReadOnly(reason);
			return;
		}
		const band = bandAt(
			h.entries.map((x) => x.t),
			h.active.row,
			h.active.col
		);
		// Onglet à choix : les lignes des autres valeurs se remplissent comme
		// un onglet neuf (la ligne saisie reçoit la valeur choisie).
		if (band && !bandOpen(band)) {
			h.say(bandRefusal(band.name));
			return;
		}
		const info = h.cellInfo(h.active.row, h.active.col);
		// Ligne pas encore chargée (juste après une saisie qui l'a créée) : la
		// frappe ne doit pas être perdue ; seule l'édition de la valeur (F2)
		// attend les données.
		if (info.kind === 'data' && !info.row && initial === undefined) return;
		// Sélection d'avant la saisie : Ctrl+Entrée valide dans toute la plage.
		editRange = { sel: h.selection(), anchor: { ...h.anchor } };
		h.anchor = { ...h.active };
		h.editByTyping = byTyping;
		h.editValueText = initial ?? info.value;
		// Saisir dans une cellule annule le copier-coller en cours (Excel).
		if (h.clipRange) {
			h.clipRange = null;
			h.schedule();
		}
		// L'éditeur doit avoir le focus avant la frappe suivante : sinon une
		// saisie rapide perdrait des caractères.
		flushSync(() => {
			h.editRO = '';
			h.editing = true;
		});
		h.ensureVisible(h.active.row, h.active.col);
		if (!h.input) return;
		placeEditor();
		h.input.focus();
		const n = h.input.value.length;
		h.input.setSelectionRange(n, n);
	}

	/**
	 * Double clic ou F2 sur une cellule en lecture seule (readonlyedit.ts) :
	 * l'éditeur s'ouvre sur son texte, à sélectionner et copier ; rien ne s'y
	 * saisit, la barre d'état dit pourquoi.
	 */
	function openReadOnly(reason: string) {
		const info = h.cellInfo(h.active.row, h.active.col);
		if (info.kind === 'data' && !info.row) return; // ligne pas encore chargée
		editRange = null;
		h.anchor = { ...h.active };
		h.editByTyping = false;
		h.editValueText = editText(h.active.row, h.active.col);
		flushSync(() => {
			h.editRO = reason;
			h.editing = true;
		});
		h.say(reason);
		h.ensureVisible(h.active.row, h.active.col);
		if (!h.input) return;
		placeEditor();
		h.input.focus();
		const n = h.input.value.length;
		h.input.setSelectionRange(n, n);
	}

	/** Referme l'éditeur en lecture seule, sans rien écrire ni message de refus. */
	function closeReadOnly(focusGrid = true) {
		if (h.message === h.editRO) h.say('');
		h.editing = false;
		h.editRO = '';
		h.editErr = null;
		h.completion = h.sig = null;
		h.refreshFx();
		if (focusGrid) h.wrap.focus();
		h.schedule();
	}

	/** Une valeur déversée (formule qui déverse) est-elle montrée en (r, c) ? */
	function spilledAt(r: number, c: number): boolean {
		for (const sp of h.spills) {
			const v = sp.rows[r - sp.row]?.[c - sp.col];
			if (v) return true;
		}
		return false;
	}

	function placeEditor() {
		if (!h.input) return;
		const x = h.screenX(h.active.col);
		const y = h.screenY(h.active.row);
		const box = h.input.parentElement!;
		box.style.transform = `translate(${x}px, ${y}px)`;
		// Comme Excel : l'éditeur s'élargit vers la droite tant que les cellules
		// voisines sont vides, puis passe à la ligne ; une fusion le fait au moins
		// aussi grand qu'elle.
		const row = h.active.row;
		const g = h.boxOf(h.active.row, h.active.col);
		const longest = Math.max(...h.input.value.split('\n').map((l) => h.textWidth(l)));
		// Vide : ni valeur, ni tableau, ni tableau croisé, ni fusion, ni valeur
		// déversée (l'éditeur passait sur le résultat d'une formule qui déverse).
		const empty = (c: number) => !h.tableAt(row, c) && !h.cells.has(h.key(row, c)) && !h.pivotAt(row, c) && !h.mergeOf(row, c) && !spilledAt(row, c);
		const last = g ? g.col + g.cols - 1 : h.active.col;
		const width = editorWidth(longest + 16, h.active.col, (c) => h.cax().size(c), (c) => c > last && empty(c), Math.max(80, h.viewW - x - 4));
		const mergedW = g ? h.cax().offset(g.col + g.cols) - h.cax().offset(g.col) : 0;
		// Largeur de confort seulement si la cellule voisine est vide : l'éditeur
		// ne passe jamais par-dessus une cellule qui a un contenu (il passe à la
		// ligne dans sa colonne).
		const floor = empty(last + 1) ? 80 : 0;
		box.style.width = `${Math.max(width, mergedW, floor)}px`;
		h.input.style.height = '0px';
		h.editorH = Math.max(g ? h.rspan(g.row, g.row + g.rows - 1) : h.rh(row), h.input.scrollHeight + 4);
		h.input.style.height = '';
		box.style.height = `${h.editorH}px`;
		const e = h.tableAt(h.active.row, h.active.col);
		const st = h.styleAt(h.active.row, h.active.col, e);
		h.input.style.textAlign = isFormula(h.input.value) ? 'left' : (st?.al ?? (e && h.active.row > e.t.row ? alignment(e.t.columns[h.active.col - e.t.col].type) : 'left'));
		h.input.style.font = st?.ff || st?.fs || st?.b || st?.i ? cellFont(st) : '';
	}

	// Classeur collaboratif : la cellule en cours de saisie est signalée à la
	// source, et libérée une fois la saisie écrite (ou abandonnée).
	let editingAt: { row: number; col: number } | null = null;
	$effect(() => {
		// Éditeur en lecture seule : rien ne se saisit, rien à signaler.
		const on = h.editing && !h.editRO;
		if (!h.source.editing) return;
		if (on && !editingAt) {
			editingAt = { row: h.active.row, col: h.active.col };
			h.source.editing(editingAt.row, editingAt.col, true).catch(() => undefined);
		} else if (!on && editingAt) {
			const at = editingAt;
			editingAt = null;
			const done = h.source.editing;
			h.writing = h.writing.catch(() => undefined).then(() => done(at.row, at.col, false).then(() => undefined, () => undefined));
		}
	});
	// keep : saisie au clavier ; refusée, son texte revient dans l'éditeur
	// (sur sa cellule) au lieu d'être perdu.
	// onrefused : reçoit le refus du moteur (renommage depuis la zone de nom).
	// key : réponse à la question de la clé (keyDlg).
	function write(row: number, col: number, raw: string, keep = false, onrefused?: (e: unknown) => void, key?: KeyChoice): Promise<void> {
		h.writing = h.writing.catch(() => undefined).then(() => writeNow(row, col, raw, keep, onrefused, key));
		return h.writing;
	}

	async function writeNow(row: number, col: number, raw: string, keep = false, onrefused?: (e: unknown) => void, key?: KeyChoice) {
		const pv = h.pivotAt(row, col);
		if (pv && pivotHandAt(pv, row, col) < 0) {
			h.message = h.PIVOT_READONLY;
			return;
		}
		if (h.queryAt(row, col)) {
			h.message = h.QUERY_READONLY;
			return;
		}
		const info = h.cellInfo(row, col);
		const target =
			info.kind === 'free'
				? {}
				: { table: info.e!.t, column: info.i, index: info.kind === 'data' ? info.n : undefined, rowId: info.kind === 'data' ? info.row?.id : undefined };
		const t0 = performance.now();
		// Affichage immédiat (optimiste) d'une valeur saisie dans un tableau :
		// le moteur confirme ensuite (valeur typée), ou la saisie est annulée.
		const optimistic = info.kind === 'data' && info.n !== undefined && info.row && !isFormula(raw) && !info.e!.t.columns[info.i!]?.formula;
		const before = optimistic ? info.row!.cells[info.i!] : undefined;
		if (optimistic) {
			const v = h.isNumeric(raw) ? Number(raw.replace(/[\s  ]/g, '').replace(',', '.')) : raw;
			info.e!.cache.set(info.n!, info.i!, raw.trim() === '' ? null : v);
			h.schedule();
		}
		h.typed = { row, col, t: performance.now() };
		try {
			// Colonne sur un niveau saisie depuis la bande : c'est son en-tête.
			const m = await h.source.setCell(info.kind === 'header' ? info.e!.t.row : row, col, raw, target, key);
			h.lastEditMs = performance.now() - t0;
			if (info.kind === 'data' && isFormula(raw) && info.e) {
				const name = info.e.t.columns[info.i!]?.name;
				if (name) h.columnFlash = { table: info.e.t.id, name, t: performance.now(), done: new Set() };
				h.schedule();
			}
			if (m && typeof m === 'object') {
				if (info.kind === 'data' && info.n !== undefined) info.e!.cache.set(info.n, info.i!, m.value);
				h.message = '';
			} else h.message = m ?? '';
			// Adresse web ou e-mail tapée : un lien, à part (Ctrl+Z le retire, le texte reste).
			if (keep && info.kind !== 'header') h.autoLinkAfter(row, col, raw);
		} catch (e) {
			if (optimistic) info.e!.cache.set(info.n!, info.i!, before); // saisie refusée : valeur d'avant
			h.message = errorText(e);
			// Formule refusée : la saisie n'est pas perdue, l'éditeur se rouvre.
			const k = h.message.indexOf('formule : ');
			const q = key ? null : keyQuestion(h.message, e);
			const hq = key ? null : handQuestion(h.message, e);
			const dva = key === 'dv' ? null : dvAlertOf(e);
			if (dva) {
				// Validation des données : l'alerte demande quoi faire (Réessayer, Oui, OK…).
				h.message = '';
				h.dvAlert = { ...dva, row, col, raw, keep };
			} else if (q) {
				// Recherche dans un tableau sans clé : la boîte demande quoi utiliser.
				h.message = '';
				h.keyDlg = { ...q, row, col, raw, keep, choice: 'index' };
			} else if (hq) {
				// Première valeur d'une colonne saisie : la boîte demande la jointure.
				h.message = '';
				h.handDlg = { ...hq, choice: hq.columns[0] ?? '#', write: { row, col, raw, keep } };
			} else if (onrefused) onrefused(e);
			else if (k >= 0) {
				const msg = h.message.slice(k);
				queueMicrotask(() => reopenFormula(row, col, raw, msg));
			} else if (keep && !h.editing) {
				// Saisie refusée : le texte tapé reste dans l'éditeur.
				const why = h.message;
				h.active = { row, col };
				h.anchor = { ...h.active };
				startEdit(true, raw);
				h.message = why;
				// En-tête refusé (nom de colonne pris…) : la boîte l'explique.
				if (info.kind === 'header') void refuseName(() => h.input, refusedName(tr('grid.names.refused.column'), e));
			}
		}
		h.refreshFx();
		h.refreshStatus();
		h.schedule();
	}

	/**
	 * Valide la saisie ; faux si une formule a une faute de syntaxe (l'éditeur
	 * reste alors ouvert sur le texte tapé, la faute soulignée et expliquée ;
	 * les parenthèses oubliées en fin de formule sont fermées d'office), ou si
	 * la validation de la cellule (liste déroulante) refuse la valeur.
	 */
	/** Nom de colonne tapé dans un en-tête : pourquoi il est refusé (déjà pris dans le tableau), null s'il passe. */
	function headerProblem(row: number, col: number, text: string): NameProblem | null {
		const info = h.cellInfo(row, col);
		if (info.kind !== 'header' || text === info.value || isFormula(text)) return null;
		const t = info.e!.t;
		// Deux lignes : l'en-tête se saisit sans son groupe, qui reste.
		const group = t.twoLevel ? splitGroup(t.columns[info.i].name).group : '';
		const full = group && text.trim() ? `${group}.${text}` : text;
		return columnNameProblem(full, t.columns.filter((_, i) => i !== info.i).map((c) => c.name), t.name);
	}

	function commit(): boolean {
		if (!h.editing) return true;
		// Lecture seule : rien à valider, l'éditeur se referme.
		if (h.editRO) {
			closeReadOnly();
			return true;
		}
		if (nameAlertOpen()) return false;
		if (isFormula(h.editValueText)) {
			const text = closeParens(h.editValueText);
			const err = checkFormula(text);
			if (err) {
				showFormulaError(err);
				return false;
			}
			h.editValueText = text;
		}
		const { row, col } = h.active;
		let text = h.editValueText;
		const named = headerProblem(row, col, text);
		if (named) {
			void refuseName(() => h.input, named);
			return false;
		}
		if (text !== h.cellInfo(row, col).value) {
			const r = h.refusal(row, col, text);
			if ('message' in r) {
				h.say(r.message);
				return false;
			}
			text = r.value;
		}
		h.editing = false;
		h.editErr = null;
		if (text !== h.cellInfo(row, col).value) {
			write(row, col, text, true);
			h.lastRepeat = () => write(h.active.row, h.active.col, text);
		}
		h.wrap.focus();
		return true;
	}

	/**
	 * La saisie de la cellule passe dans la barre de formule (clic dans la
	 * barre, comme Excel) : même texte, rien de validé ; le curseur est celui
	 * que le clic place dans la barre.
	 */
	function handToFx() {
		h.fxText = h.editValueText;
		h.editing = false;
		h.editErr = null;
		h.lastRef = null;
		h.fxEditing = true;
		h.schedule();
	}

	// Sélection au début de la saisie (startEdit ne garde que la cellule active).
	let editRange: { sel: Range; anchor: { row: number; col: number } } | null = null;

	/** Ctrl+Entrée : la saisie va dans toute la sélection (une seule étape d'annulation), qui reste sélectionnée. */
	function commitAll() {
		const r = editRange;
		const sel = r?.sel;
		if (!r || !sel || isSingleCell(sel, h.active, h.boxes) || !h.source.paste) {
			commit();
			return;
		}
		if ((sel.r1 - sel.r0 + 1) * (sel.c1 - sel.c0 + 1) > 10000) {
			h.say(tr('grid.edit.multiLimit'));
			return;
		}
		h.editing = false;
		const text = h.editValueText;
		const paste = h.source.paste;
		const fill = (s: Range) =>
			(h.writing = h.writing.catch(() => undefined).then(async () => {
				try {
					h.message = (await paste(s.r0, s.c0, fillBlock(s, text))) ?? '';
				} catch (err) {
					h.message = errorText(err);
				}
				h.refreshFx();
				h.refreshStatus();
				h.schedule();
			}));
		fill(sel);
		h.lastRepeat = () => fill(h.selection());
		h.anchor = r.anchor;
		h.wrap.focus();
		h.refreshFx();
		h.schedule();
	}

	/** Remplace le texte de l'éditeur de cellule et place le curseur. */
	function setEditorText(value: string, caret: number) {
		flushSync(() => (h.editValueText = value));
		h.input?.setSelectionRange(caret, caret);
		if (h.input) h.onFormulaInput(h.input, 'cell');
	}

	/** Hauteur de l'éditeur : une ligne de plus par saut de ligne (Alt+Entrée). */
	function editorHeight(s: string): number {
		return 18 * s.split('\n').length + 4;
	}

	/**
	 * Formule validable (bouton « Valider ») : sans faute, ou presque, quand il
	 * ne manque que des parenthèses fermantes, que la validation ajoute.
	 */
	function formulaReady(text: string): boolean {
		return !checkFormula(closeParens(text));
	}

	/** Montre une faute de formule dans l'éditeur ouvert : message, caractère sélectionné. */
	function showFormulaError(err: FormulaError) {
		h.editErr = err;
		h.say(tr('grid.formula.error', { error: err.msg }));
		const el = h.input;
		if (!el) return;
		requestAnimationFrame(() => {
			el.focus();
			el.setSelectionRange(err.pos, Math.max(err.end, err.pos));
		});
	}

	/**
	 * Une formule refusée par le serveur (fonction mal employée, plage
	 * impossible…) : l'éditeur se rouvre sur le texte tapé, si l'utilisateur
	 * n'a pas commencé autre chose.
	 */
	function reopenFormula(row: number, col: number, raw: string, msg: string) {
		if (h.editing || h.fxEditing || !isFormula(raw)) return;
		h.moveTo(row, col);
		startEdit(true, raw);
		const pos = serverErrorPos(raw, msg);
		const text = msg.replace(/^formule : (position \d+ : )?/, '');
		if (pos !== null) showFormulaError({ pos, end: pos + 1, msg: text });
		else h.say(tr('grid.formula.error', { error: text }));
	}

	function cancel() {
		if (h.editRO) {
			closeReadOnly();
			return;
		}
		h.editing = false;
		h.editErr = null;
		h.refreshFx(); // la barre de formule reprend le contenu de la cellule
		h.wrap.focus();
		h.schedule();
	}

	/** ↑ / ↓ dans une formule : curseur au début ou à la fin du texte (Maj : sélection jusque-là). */
	function caretToEdge(end: boolean, extend: boolean) {
		const el = h.input;
		if (!el) return;
		const n = el.value.length;
		if (!extend) el.setSelectionRange(end ? n : 0, end ? n : 0);
		else if (end) el.setSelectionRange(el.selectionStart ?? 0, n, 'forward');
		else el.setSelectionRange(0, el.selectionEnd ?? n, 'backward');
		const s = h.catalog ? signature(el.value, el.selectionStart ?? n, h.catalog) : null;
		h.sig = s ? { ...s, where: 'cell' } : null;
	}

	/**
	 * Efface la sélection : tableaux entièrement compris supprimés, contenu des
	 * autres cellules de tableau effacé. Cellules libres : contenu seul, ou
	 * tout (all : Supprimer, avec mise en forme, fusions, notes, validations).
	 */
	async function clearSelection(range = h.selection(), all = false) {
		const { r0, r1, c0, c1 } = range;
		if (h.queryIn(r0, c0, r1, c1, true)) {
			h.say(h.QUERY_READONLY);
			return;
		}
		// Tableaux croisés : pris en entier (ou en erreur), supprimés avec le
		// reste ; touchés en partie seulement, rien ne s'efface (Excel).
		const pv = pivotsCleared(h.pivots, range);
		if (pv.partial) {
			h.say(h.PIVOT_PARTIAL);
			return;
		}
		const pivotGone = pv.gone.length === 1 ? tr('grid.pivot.deleted', { name: pv.gone[0].config.name ?? '' }) : pv.gone.length ? tr('grid.pivot.deletedMany', { n: pv.gone.length }) : '';
		if (h.panel?.kind === 'pivot' && pv.gone.some((p) => p.id === h.panel!.id)) h.panel = null;
		const wipe = all ? h.source.clearCells : undefined;
		// Les tableaux entièrement compris dans la zone sont supprimés.
		const whole = h.entries.filter((e) => e.t.row >= r0 && e.last <= r1 && e.t.col >= c0 && e.lastCol <= c1);
		if (h.source.clearRange) {
			// Tout en une seule étape d'annulation.
			const clear = h.source.clearRange;
			const tables = whole.map((e) => e.t);
			h.writing = h.writing.catch(() => undefined).then(() =>
				clear(r0, c0, r1, c1, all).then(
					() => {
						if (tables.length > 1) h.say(tr('grid.table.deletedMany', { n: tables.length }));
						else if (tables.length) h.say(tr('grid.table.deleted', { name: tables[0].name }));
						else if (pivotGone) h.say(pivotGone);
						else if (all) h.say(tr('grid.cells.deleted'));
						h.refreshFx();
						h.schedule();
					},
					(err) => h.say(errorText(err))
				)
			);
			await h.writing;
			return;
		}
		const rest = (r: number, c: number) => !whole.some((e) => h.inTable(e, r, c));
		let n = 0;
		for (const e of h.entries) {
			if (whole.includes(e)) continue;
			const rr = Math.max(0, Math.min(r1, e.last) - Math.max(r0, e.t.row) + 1);
			const cc = Math.max(0, Math.min(c1, e.lastCol) - Math.max(c0, e.t.col) + 1);
			n += rr * cc;
		}
		if (!wipe)
			for (const k of h.cells.keys()) {
				const [r, c] = k.split(',').map(Number);
				if (r >= r0 && r <= r1 && c >= c0 && c <= c1) n++;
			}
		if (n > 1000) {
			h.say(tr('grid.clear.limit'));
			return;
		}
		if (pv.gone.length && h.source.deletePivot) {
			const del = h.source.deletePivot;
			const ids = pv.gone.map((p) => p.id);
			h.writing = h.writing.catch(() => undefined).then(async () => {
				try {
					for (const id of ids) await del(id);
					h.say(pivotGone);
				} catch (err) {
					h.say(errorText(err));
				}
			});
			await h.writing;
		}
		if (whole.length && h.source.deleteTables) {
			const del = h.source.deleteTables;
			const tables = whole.map((e) => e.t);
			h.writing = h.writing.catch(() => undefined).then(() =>
				del(tables).then(
					() => h.say(tables.length > 1 ? tr('grid.table.deletedMany', { n: tables.length }) : tr('grid.table.deleted', { name: tables[0].name })),
					(err) => h.say(errorText(err))
				)
			);
			await h.writing;
		}
		for (const e of h.entries) {
			if (whole.includes(e)) continue;
			for (let r = Math.max(r0, e.t.row); r <= Math.min(r1, e.last); r++)
				for (let c = Math.max(c0, e.t.col); c <= Math.min(c1, e.lastCol); c++) {
					const info = h.cellInfo(r, c);
					if (info.kind !== 'header' && info.value !== '') await write(r, c, '');
				}
		}
		if (wipe) {
			h.writing = h.writing.catch(() => undefined).then(() =>
				wipe(r0, c0, r1, c1).then(
					() => h.say(tr('grid.cells.deleted')),
					(err) => h.say(errorText(err))
				)
			);
			await h.writing;
			return;
		}
		for (const k of [...h.cells.keys()]) {
			const [r, c] = k.split(',').map(Number);
			if (r >= r0 && r <= r1 && c >= c0 && c <= c1 && rest(r, c)) await write(r, c, '');
		}
	}

	function onEditorKey(e: KeyboardEvent) {
		if (h.editRO) {
			// Lecture seule : copier, sélectionner, déplacer le curseur ; rien d'autre.
			e.stopPropagation();
			const k = viewKey(e);
			if (k === 'native') return;
			e.preventDefault();
			if (k === 'close') closeReadOnly();
			else if (k === 'enter') {
				closeReadOnly();
				h.enterMove(e.shiftKey);
			} else if (k === 'tab') {
				closeReadOnly();
				h.tabMove(e.shiftKey);
			}
			return;
		}
		if (h.input && h.completionKey(e, h.input, (v) => (h.editValueText = v))) return;
		e.stopPropagation();
		const { row, col } = h.active;
		const sc = shortcutOf(e);
		if ((sc === 'today' || sc === 'now') && h.input) {
			// Date ou heure du jour au curseur, comme Excel pendant la saisie.
			const r = insertAt(h.input.value, h.input.selectionStart, h.input.selectionEnd, sc === 'today' ? todayText() : timeText());
			setEditorText(r.value, r.caret);
			e.preventDefault();
			return;
		}
		switch (e.key) {
			case 'Enter':
				if (e.altKey && h.input) {
					// Alt+Entrée : saut de ligne dans la cellule ; les flèches
					// déplacent alors le curseur dans le texte.
					const r = insertAt(h.input.value, h.input.selectionStart, h.input.selectionEnd, '\n');
					setEditorText(r.value, r.caret);
					h.editByTyping = false;
					break;
				}
				if (e.ctrlKey || e.metaKey) {
					commitAll();
					break;
				}
				if (commit()) h.enterMove(e.shiftKey);
				break;
			case 'Tab':
				if (commit()) h.tabMove(e.shiftKey);
				break;
			case 'Escape':
				cancel();
				break;
			case 'ArrowDown':
			case 'ArrowUp':
			case 'ArrowLeft':
			case 'ArrowRight':
				// Formule : les flèches déplacent le curseur dans le texte (← → d'un
				// caractère, Ctrl d'un mot, ↑ ↓ au début et à la fin, Maj pour
				// sélectionner) ; seuls Entrée, Tab et le clic ailleurs valident.
				if (isFormula(h.editValueText) || h.editValueText.trimStart() === '=') {
					if (e.key === 'ArrowUp' || e.key === 'ArrowDown') caretToEdge(e.key === 'ArrowDown', e.shiftKey);
					else return;
					break;
				}
				if (!h.editByTyping) return;
				if (!commit()) break;
				h.moveTo(
					row + (e.key === 'ArrowDown' ? 1 : e.key === 'ArrowUp' ? -1 : 0),
					col + (e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0)
				);
				break;
			default:
				return;
		}
		e.preventDefault();
	}

	// Barre de formule : même chemin d'écriture que l'éditeur de cellule.
	function onFxKey(e: KeyboardEvent) {
		if (h.fxInput && h.completionKey(e, h.fxInput, (v) => (h.fxText = v))) return;
		e.stopPropagation();
		if (h.hold && (e.key === 'Enter' || e.key === 'Escape')) {
			// Formule pointée sur une autre feuille : validée (Entrée) ou
			// abandonnée (Échap) en revenant à sa feuille, comme Excel.
			e.preventDefault();
			h.finishHold(e.key === 'Enter' ? 'commit' : 'cancel');
			return;
		}
		if (h.fxLock && !h.fxEditing) {
			// Lecture seule : Entrée ou Échap rendent la main à la grille, sans rien changer.
			if (e.key === 'Enter' || e.key === 'Escape') {
				e.preventDefault();
				h.wrap.focus();
			}
			return;
		}
		if (e.key === 'Escape') {
			h.fxEditing = false;
			h.editErr = null;
			h.refreshFx();
			h.wrap.focus();
		} else if (e.key === 'Enter') {
			const { row, col } = h.active;
			e.preventDefault();
			if (commitFx(true) === true) h.moveTo(row + 1, col);
			return;
		} else return;
		e.preventDefault();
	}

	/**
	 * Valide la saisie de la barre de formule dans la cellule active (Entrée,
	 * clic dans la grille, focus parti ailleurs), comme l'éditeur de cellule.
	 * 'formula' (formule fautive) ou 'refused' (valeur ou nom refusé) quand la
	 * saisie reste ouverte ; true si validée (ou rien à valider).
	 */
	function commitFx(focusGrid: boolean): true | 'formula' | 'refused' {
		if (!h.fxEditing) return true;
		if (nameAlertOpen()) return 'refused';
		const { row, col } = h.active;
		// Rien de changé (focus pris puis rendu) : la barre se referme, sans vérifier la formule.
		if (h.fxText !== h.cellInfo(row, col).value) {
			if (isFormula(h.fxText)) {
				const text = closeParens(h.fxText);
				const err = checkFormula(text);
				if (err) {
					h.editErr = err;
					h.say(tr('grid.formula.error', { error: err.msg }));
					const el = h.fxInput;
					if (el && document.activeElement === el) el.setSelectionRange(err.pos, Math.max(err.end, err.pos));
					else
						requestAnimationFrame(() => {
							el?.focus();
							el?.setSelectionRange(err.pos, Math.max(err.end, err.pos));
						});
					return 'formula';
				}
				h.fxText = text;
			}
		}
		h.editErr = null;
		let text = h.fxText;
		const named = headerProblem(row, col, text);
		if (named) {
			void refuseName(h.fxInput, named);
			return 'refused';
		}
		if (text !== h.cellInfo(row, col).value) {
			const r = h.refusal(row, col, text);
			if ('message' in r) {
				h.say(r.message);
				return 'refused';
			}
			text = r.value;
		}
		h.fxEditing = false;
		h.completion = h.sig = null;
		if (text !== h.cellInfo(row, col).value) {
			write(row, col, text);
			h.lastRepeat = () => write(h.active.row, h.active.col, text);
		}
		if (focusGrid) h.wrap.focus();
		h.refreshFx();
		return true;
	}

	/** Saisie en cours (cellule ou barre de formule) validée avant un clic dans la grille ; faux si elle reste ouverte. */
	function commitEditor(): boolean {
		if (h.editing) return commit();
		return commitFx(true) === true;
	}

	/** Barre de formule quittée (blur) : validée, gardée ou refermée (crossref.ts, fxLeave). */
	function leaveFx() {
		h.completion = h.sig = null;
		const action = fxLeave({
			editing: h.fxEditing,
			stillFocused: !!h.fxInput && document.activeElement === h.fxInput,
			alertOpen: nameAlertOpen(),
			pointing: !!h.hold || h.tabPress,
			text: h.fxText
		});
		if (action === 'keep') return;
		if (action === 'commit') {
			const r = commitFx(false);
			// Formule fautive : la saisie reste ouverte, le focus y revient (commitFx).
			if (r === 'formula') return;
			// Valeur refusée quand le focus part ailleurs : abandonnée, le message reste (comme l'éditeur de cellule).
			if (r === 'refused' && nameAlertOpen()) return;
		}
		h.fxEditing = false;
		h.editErr = null;
		h.refreshFx();
	}

	// --- Zone de nom ------------------------------------------------------------

	const gotoTable = (e: Entry): GotoTable => ({ name: e.t.name, row: e.t.row, col: e.t.col, last: e.last, lastCol: e.lastCol, columns: e.t.columns.map((c) => c.name) });

	/**
	 * Zone de nom, comme Excel : une adresse (F10, Cell[F10], A1:C3), un
	 * tableau ou une de ses cellules (Ventes[Prix][3]) y va ; faux sinon.
	 */
	function goToName(text: string): boolean {
		const cur = h.ownerAt(h.active.row, h.active.col);
		const g = parseGoto(text, h.entries.map(gotoTable), cur ? gotoTable(cur) : undefined);
		if (!g) return false;
		h.wrap.focus();
		if (g.r0 === g.r1 && g.c0 === g.c1) h.moveTo(g.r0, g.c0);
		else h.selectRange(g.r0, g.c0, g.r1, g.c1);
		return true;
	}

	/** Nom du tableau : une adresse y va (comme Excel), un autre nom renomme le tableau. */
	function onTableNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const t = h.ownerAt(h.active.row, h.active.col)?.t;
		if (e.key === 'Escape' || !t) {
			el.value = t?.name ?? '';
			h.wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const typed = el.value;
		const name = typed.trim();
		// Le champ revient au nom du tableau (aller ailleurs ne le change pas
		// toujours : même tableau, la valeur liée ne bouge pas).
		el.value = t.name;
		if (name !== t.name && goToName(name)) return;
		if (!h.source.renameTable || name === t.name) {
			h.wrap.focus();
			return;
		}
		// Nom refusé : la boîte l'explique, le champ garde le texte tapé.
		const problem = tableNameProblem(name, h.entries.filter((x) => x.t !== t).map((x) => x.t.name));
		if (problem) {
			void refuseName(el, problem, typed);
			return;
		}
		el.value = typed;
		const rename = h.source.renameTable;
		h.writing = h.writing.catch(() => undefined).then(async () => {
			try {
				await rename(t, name);
				h.message = tr('grid.table.renamed', { name });
				if (document.activeElement === el) h.wrap.focus();
			} catch (err) {
				void refuseName(el, refusedName(tr('grid.names.refused.table'), err), typed);
			}
			h.refreshFx();
			h.refreshStatus();
		});
	}

	/** Colonne[ligne] : Entrée renomme la colonne. */
	function onColumnNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		const en = h.ownerAt(h.active.row, h.active.col);
		if (e.key === 'Escape' || !en) {
			el.value = h.tableBox?.col ?? '';
			h.wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const c = h.active.col;
		const text = el.value.trim();
		// Bande des groupes : renomme le groupe (toutes ses colonnes).
		if (h.active.row < en.t.row && !h.singleCol(en, c - en.t.col)) {
			h.wrap.focus();
			if (text !== h.tableBox?.col) write(h.active.row, c, text, false, (err) => void refuseName(el, refusedName(tr('grid.names.refused.group'), err), text));
			return;
		}
		// Une autre cellule (Prix[3], Ventes[Prix][2], Cell[F10], A1:C3) : y
		// aller ; un nom seul (même « Q1 ») renomme la colonne.
		if (text !== h.tableBox?.col && /[[:]/.test(text)) {
			el.value = h.tableBox?.col ?? '';
			if (goToName(text)) return;
			el.value = text;
		}
		const name = text.replace(/\[.*$/, '').trim();
		const old = en.t.columns[c - en.t.col].name;
		if (name === old) {
			el.value = h.tableBox?.col ?? '';
			h.wrap.focus();
			return;
		}
		// Nom vide ou pris : la boîte l'explique, le champ garde le texte tapé.
		const problem = name
			? columnNameProblem(name, en.t.columns.filter((_, i) => i !== c - en.t.col).map((x) => x.name), en.t.name)
			: { title: tr('grid.names.column.invalid'), message: tr('grid.names.column.empty') };
		if (problem) {
			void refuseName(el, problem, text);
			return;
		}
		h.wrap.focus();
		write(en.t.row, c, name, false, (err) => void refuseName(el, refusedName(tr('grid.names.refused.column'), err), text));
	}

	function onNameKey(e: KeyboardEvent) {
		e.stopPropagation();
		const el = e.currentTarget as HTMLInputElement;
		if (e.key === 'Escape') {
			el.value = h.nameBox;
			h.wrap.focus();
			return;
		}
		if (e.key !== 'Enter') return;
		e.preventDefault();
		const text = el.value.trim();
		// Le nom d'une cellule existante : y aller (comme Excel).
		for (const [k, n] of h.names) {
			if (n.toLowerCase() === text.toLowerCase()) {
				const [r, c] = k.split(',').map(Number);
				h.wrap.focus();
				h.moveTo(r, c);
				return;
			}
		}
		// Une adresse (B3, Cell[B3], A1:C3) ou un tableau : y aller.
		if (goToName(text)) return;
		const { row, col } = h.active;
		if (h.tableAt(row, col)) {
			h.say(tr('grid.names.cell.inTable', { table: h.tableAt(row, col)!.t.name }));
			h.wrap.focus();
			return;
		}
		if (!h.source.setName || text === (h.names.get(h.key(row, col)) ?? '')) {
			h.wrap.focus();
			return;
		}
		// Nom refusé : la boîte l'explique, le champ garde le texte tapé.
		const problem = cellNameProblem(text);
		if (problem) {
			void refuseName(el, problem, text);
			return;
		}
		const setName = h.source.setName;
		h.writing = h.writing.catch(() => undefined).then(async () => {
			try {
				await setName(row, col, text);
				h.message = text ? tr('grid.names.cell.set', { name: text }) : tr('grid.names.cell.removed');
			} catch (err) {
				void refuseName(() => h.nameInput, refusedName(tr('grid.names.refused.cell'), err), text);
			}
			h.refreshFx();
			h.refreshStatus();
		});
		h.wrap.focus();
	}

	return {
		readOnlyReason,
		designWhy,
		editText,
		startEdit,
		closeReadOnly,
		spilledAt,
		placeEditor,
		write,
		commit,
		handToFx,
		formulaReady,
		reopenFormula,
		cancel,
		clearSelection,
		onEditorKey,
		onFxKey,
		commitEditor,
		leaveFx,
		onTableNameKey,
		onColumnNameKey,
		onNameKey
	};
}
