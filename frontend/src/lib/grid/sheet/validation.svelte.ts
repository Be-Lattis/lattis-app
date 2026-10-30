// Validation des données (liste déroulante, règles, référence pointée,
// alerte, message de saisie, cercles) et colonnes de référence (décision
// 0016 : basculer sur la ligne citée). Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { type SyncKind } from '../cache';
import { type AxisLike } from '../anim';
import { type CellStyle } from '../style';
import { cleanSource, dvTarget, dvSource, validate, validateSource, type DvList, type MergeRange } from '../sheetfmt';
import { dvForm, dvFromForm, dvInputOf, dvLegacy, dvListOf, dvRule, type DvAlert, type DvForm } from '../dvrule';
import { type Cell, type CellRange } from '../mergesel';
import { type KeyChoice } from '../keychoice';
import { refMessage, refTarget, type Citation } from '../reflink';
import { rangeRef } from '../formulaEdit';
import { qualifyRef } from '../crossref';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetSource, type Entry, type WholeSel, type DvPick, type InputTip } from './model';
import { TOP_H } from './consts';
import type { RefTable } from '../formulaEdit';

/** Ce que createValidation lit et modifie dans Sheet.svelte. */
export interface ValidationHost {
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	readonly dvLists: Map<string, DvList>;
	readonly dvGen: number;
	readonly source: SheetSource;
	active: { row: number; col: number; };
	readonly mergeOf: (r: number, c: number) => MergeRange | undefined;
	readonly screenX: (c: number) => number;
	readonly cax: () => AxisLike;
	readonly screenY: (r: number) => number;
	readonly rowTop: (r: number) => number;
	readonly rh: (r: number) => number;
	readonly locked: string;
	readonly cells: Map<string, string>;
	readonly key: (r: number, c: number) => string;
	readonly keptOut: (row: number, col: number) => boolean;
	readonly tableHintBox: { x: number; y: number; w: number; h: number } | null;
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly closeMenus: () => void;
	listMenu: { x: number; y: number; values: string[]; row: number; col: number; error?: string; loading?: boolean } | null;
	readonly toView: (v: number) => number;
	readonly wrap: HTMLDivElement;
	readonly say: (m: string) => void;
	readonly cellInfo: (r: number, c: number) => { e: Entry; i: number; kind: "header"; value: string; n?: undefined; row?: undefined; } | { e: undefined; kind: "free"; value: string; i?: undefined; n?: undefined; row?: undefined; } | { e: Entry; i: number; n: number; row: { id: number; cells: unknown[]; text: string[]; bad?: Map<number, string>; sty?: Map<number, string>; marks?: Set<number>; links?: Map<number, string>; sync?: Map<number, SyncKind>; locked?: boolean; } | undefined; kind: "data"; value: string; };
	readonly write: (row: number, col: number, raw: string, keep?: boolean, onrefused?: (e: unknown) => void, key?: KeyChoice) => Promise<void>;
	readonly onreflink: ((to: { table: string; id: number; key?: string; filter?: { column: string; value: string; }; }, from: { row: number; col: number; label: string; }) => void) | undefined;
	readonly sources: { name: string; columns: string[]; types: string[]; kind?: string; key?: number; }[];
	readonly shownText: (r: number, c: number) => string;
	readonly sheetName: string;
	readonly addrA1: (r: number, c: number) => string;
	refItems: {
		row: number;
		col: number;
		go?: { table: string; value: string; found?: boolean };
		cite?: { table: string; value: string; list: Citation[] | null };
	} | null;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null;
	validDlg: (DvForm & { target: string; error: string; preview: DvList | null; loading: boolean }) | null;
	readonly selection: () => CellRange;
	readonly wholeKind: () => "rows" | "cols" | null;
	previewTimer: ReturnType<typeof setTimeout> | undefined;
	readonly applyStyle: (patch: Record<string, unknown>) => void;
	dvPick: DvPick | null;
	anchor: { row: number; col: number; };
	wholeSel: WholeSel | null;
	readonly sheetId: () => number;
	dvPickRef: string;
	readonly refTables: () => RefTable[];
	readonly onsheet: ((sheet: number) => void) | undefined;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly schedule: () => void;
	readonly refreshStatus: () => void;
	dvAlert: (DvAlert & { row: number; col: number; raw: string; keep: boolean }) | null;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	readonly ctx: CanvasRenderingContext2D;
	readonly MARGIN: number;
	readonly viewW: number;
	readonly viewH: number;
	inputTip: InputTip | null;
}

export function createValidation(sh: ValidationHost) {
	/** Propriété dv de la cellule (liste saisie ou plage source), hors en-tête. */
	function dvAt(r: number, c: number): unknown {
		const e = sh.tableAt(r, c);
		if (e && r === e.t.row) return undefined;
		return sh.styleAt(r, c, e)?.dv;
	}

	/** Plage source de la validation de la cellule (liste relue), ou undefined. */
	function dvSrcAt(r: number, c: number): string | undefined {
		return dvSource(dvAt(r, c));
	}

	/**
	 * Valeurs permises de la cellule (validation par liste), ou undefined.
	 * Liste tirée d'une plage : les valeurs relues (vide tant qu'elles ne le
	 * sont pas : la cellule a quand même sa liste déroulante).
	 */
	function listOf(r: number, c: number): string[] | undefined {
		const l = dvListOf(dvAt(r, c));
		if (!l) return undefined;
		if (l.src) return sh.dvLists.get(l.src)?.values ?? [];
		return l.list;
	}

	/** Relit la liste d'une plage source ; une erreur de lecture est une liste en erreur. */
	async function loadDv(src: string): Promise<DvList> {
		const gen = sh.dvGen;
		let l: DvList;
		try {
			l = sh.source.validationList ? await sh.source.validationList(src) : { values: [], error: tr('grid.validation.readImpossible') };
		} catch (err) {
			l = { values: [], error: errorText(err) };
		}
		if (gen === sh.dvGen) sh.dvLists.set(src, l);
		return l;
	}

	/** La cellule active a une liste tirée d'une plage : la relire (vérification de la saisie). */
	function prefetchDv() {
		const src = dvSrcAt(sh.active.row, sh.active.col);
		const k = `${sh.dvGen}|${src}`;
		if (!src || sh.dvLists.has(src) || dvLoading.has(k)) return;
		dvLoading.add(k);
		void loadDv(src).finally(() => dvLoading.delete(k));
	}
	const dvLoading = new Set<string>();

	/** Bouton ▾ de la liste, juste à droite de la cellule active (comme Excel). */
	function dropButton() {
		const g = sh.mergeOf(sh.active.row, sh.active.col);
		const right = g ? sh.screenX(g.col) + sh.cax().offset(g.col + g.cols) - sh.cax().offset(g.col) : sh.screenX(sh.active.col) + sh.cax().size(sh.active.col);
		const bottom = sh.screenY(sh.active.row) + (g ? sh.rowTop(g.row + g.rows) - sh.rowTop(g.row) : sh.rh(sh.active.row));
		const h = Math.min(18, sh.rh(sh.active.row) - 2);
		return { x: right + 1, y: bottom - h - 1, w: 16, h };
	}

	/** La cellule active est dans un bloc d'au moins deux cellules libres, hors tableau : l'icône s'affiche. */
	function asTableHint(): boolean {
		const { row, col } = sh.active;
		if (!sh.source.formatAsTable || sh.locked || !sh.cells.has(sh.key(row, col)) || sh.tableAt(row, col) || sh.keptOut(row, col) || listOf(row, col)) return false;
		// Le bloc libre (4-connexité, hors tableaux et légendes) : au moins trois
		// cellules, et pas seulement des phrases (un texte de plusieurs lignes
		// n'est pas une liste à mettre en tableau).
		const seen = new Set<string>();
		const stack = [[row, col]];
		let n = 0;
		let short = 0;
		while (stack.length && seen.size < 200) {
			const [r, c] = stack.pop()!;
			const k = sh.key(r, c);
			if (seen.has(k) || !sh.cells.has(k) || sh.tableAt(r, c) || sh.keptOut(r, c)) continue;
			seen.add(k);
			n++;
			const v = (sh.cells.get(k) ?? '').trim();
			if (v.split(/s+/).length <= 3 && v.length <= 40) short++;
			stack.push([r + 1, c], [r - 1, c], [r, c + 1], [r, c - 1]);
		}
		return n >= 3 && short * 2 >= n;
	}
	function tableHintAt(ev: MouseEvent): boolean {
		const b = sh.tableHintBox;
		if (!b) return false;
		const { x, y } = sh.toLocal(ev);
		return x >= b.x && x <= b.x + b.w && y >= b.y && y <= b.y + b.h;
	}

	/** Ouvre la liste déroulante ; une liste tirée d'une plage est relue à chaque ouverture. */
	function openList() {
		const values = listOf(sh.active.row, sh.active.col);
		if (!values) return;
		const src = dvSrcAt(sh.active.row, sh.active.col);
		const b = dropButton();
		sh.closeMenus();
		const at = { row: sh.active.row, col: sh.active.col };
		const cached = src ? sh.dvLists.get(src) : undefined;
		sh.listMenu = { x: sh.toView(sh.screenX(sh.active.col)), y: sh.toView(b.y + b.h) + 2, values, ...at, error: cached?.error, loading: !!src && !cached };
		if (!src) return;
		void loadDv(src).then((l) => {
			const m = sh.listMenu;
			if (m && m.row === at.row && m.col === at.col) sh.listMenu = { ...m, values: l.values, error: l.error, loading: false };
		});
	}

	function pickFromList(v: string) {
		const m = sh.listMenu;
		sh.listMenu = null;
		sh.wrap.focus();
		if (!m || sh.locked) {
			if (sh.locked) sh.say(sh.locked);
			return;
		}
		if (v !== sh.cellInfo(m.row, m.col).value) sh.write(m.row, m.col, v);
	}

	/** La cellule cite la clé d'un tableau (validation tirée de sa colonne clé) et a une valeur : la référence. */
	function refAt(r: number, c: number): { src: string; table: string; key: string; value: string } | null {
		if (!sh.onreflink || !sh.source.referenceRow) return null;
		const src = dvSrcAt(r, c);
		const t = refTarget(src, sh.sources);
		const value = t ? sh.shownText(r, c).trim() : '';
		return t && src && value ? { src, ...t, value } : null;
	}

	/** Adresse de la cellule pour « Revenir à » : Feuille!C5. */
	const spotLabel = (r: number, c: number) => (sh.sheetName ? `${sh.sheetName}!` : '') + sh.addrA1(r, c);

	/** Basculer sur la ligne citée par la cellule (Ctrl+clic, clic droit, bouton ↗). */
	async function followRef(r: number, c: number) {
		const ref = refAt(r, c);
		if (!ref) return;
		try {
			const k = await sh.source.referenceRow!(ref.src, ref.value);
			if (!k.found || k.id === undefined) {
				sh.say(tr('grid.refLink.missing', { value: ref.value, table: k.table || ref.table }));
				return;
			}
			sh.onreflink?.({ table: k.table, id: k.id, key: k.key }, { row: r, col: c, label: spotLabel(r, c) });
		} catch (err) {
			sh.say(errorText(err));
		}
	}

	function loadRefItems(r: number, c: number) {
		sh.refItems = null;
		const ref = refAt(r, c);
		if (ref) {
			const at = { row: r, col: c, go: { table: ref.table, value: ref.value } as { table: string; value: string; found?: boolean } };
			sh.refItems = at;
			sh.source.referenceRow!(ref.src, ref.value).then(
				(k) => {
					if (sh.refItems?.row === r && sh.refItems.col === c) sh.refItems = { ...sh.refItems, go: { table: k.table || ref.table, value: ref.value, found: k.found } };
				},
				() => undefined
			);
			return;
		}
		const e = sh.tableAt(r, c);
		if (!e || r <= e.t.row || e.t.key !== c - e.t.col || !sh.source.citations || !sh.onreflink) return;
		const value = sh.shownText(r, c).trim();
		if (!value) return;
		const table = e.t.name;
		sh.refItems = { row: r, col: c, cite: { table, value, list: null } };
		sh.source.citations(table, value).then(
			(list) => {
				// Aucune colonne ne cite ce tableau : pas d'entrée.
				if (sh.refItems?.row !== r || sh.refItems.col !== c) return;
				sh.refItems = list?.length ? { ...sh.refItems, cite: { table, value, list } } : null;
			},
			() => (sh.refItems = null)
		);
	}

	/** Lignes qui citent : le tableau qui cite est filtré sur la valeur, la première ligne sélectionnée. */
	function openCitation(r: number, c: number, ci: Citation, value: string) {
		sh.menu = null;
		sh.refItems = null;
		sh.onreflink?.({ table: ci.table, id: ci.first, key: ci.column, filter: { column: ci.column, value } }, { row: r, col: c, label: spotLabel(r, c) });
	}

	/** Bouton ↗ de la cellule active d'une référence, juste après la flèche de la liste. */
	function linkButton() {
		const b = dropButton();
		return { ...b, x: b.x + b.w + 1 };
	}

	let dvDlgSeq = 0;

	/** Données › Validation des données : liste des valeurs permises de la sélection. */
	function openValidation() {
		if (!sh.source.applyStyle) return;
		if (sh.locked) {
			sh.say(sh.locked);
			return;
		}
		sh.closeMenus();
		const dv = dvAt(sh.active.row, sh.active.col);
		sh.validDlg = { target: validationTarget(), ...dvForm(dvRule(dv)), error: '', preview: null, loading: false };
		if (sh.validDlg.type === 'list' && sh.validDlg.mode === 'range') refreshPreview(0);
		// Bornes et formule relues dans la langue de la session, écrites pour la cellule active.
		const seq = ++dvDlgSeq;
		if (dvRule(dv) && !dvLegacy(dv) && sh.source.validationAt)
			void sh.source
				.validationAt(sh.active.row, sh.active.col)
				.then((r) => {
					const cur = sh.validDlg;
					if (!cur || seq !== dvDlgSeq || !r?.type) return;
					const f = dvForm(r);
					cur.min = f.min;
					cur.max = f.max;
					cur.formula = f.formula;
				})
				.catch(() => undefined);
	}

	/** Ce que vise la validation (la sélection), pour le titre du dialogue. */
	function validationTarget(): string {
		const e = sh.tableAt(sh.active.row, sh.active.col);
		const table = e ? { name: e.t.name, row: e.t.row, last: e.last, col: e.t.col, columns: e.t.columns.map((c) => c.name) } : null;
		return dvTarget({ sel: sh.selection(), whole: sh.wholeKind(), table, sheet: sh.sheetName });
	}

	/** Relit l'aperçu de la plage source du dialogue (après delay ms). */
	function refreshPreview(delay = 250) {
		clearTimeout(sh.previewTimer);
		const d = sh.validDlg;
		if (!d) return;
		const src = cleanSource(d.src);
		if (d.type !== 'list' || d.mode !== 'range' || !src || !sh.source.validationList) {
			d.preview = null;
			d.loading = false;
			return;
		}
		d.loading = true;
		sh.previewTimer = setTimeout(async () => {
			const l = await loadDv(src);
			const cur = sh.validDlg;
			if (!cur || cleanSource(cur.src) !== src) return;
			cur.preview = l;
			cur.loading = false;
		}, delay);
	}

	function applyValidation(clear = false) {
		const d = sh.validDlg;
		if (!d) return;
		let dv: unknown = null;
		if (!clear) {
			// La formule (et une borne en formule) est écrite pour la cellule active.
			const p = dvFromForm(d, sh.addrA1(sh.active.row, sh.active.col));
			if ('error' in p) {
				d.error = p.error;
				d.tab = 'options';
				return;
			}
			dv = p.dv;
		}
		clearTimeout(sh.previewTimer);
		sh.validDlg = null;
		sh.wrap.focus();
		sh.applyStyle({ dv });
		const l = dvListOf(dv);
		if (!dv) sh.say(tr('grid.validation.removed'));
		else if (l?.list) sh.say(tr('grid.validation.list.set', { n: l.list.length }));
		else if (l?.src) {
			// Tirée de la clé d'un tableau : la colonne en devient une référence (décision 0016).
			const t = sh.onreflink && sh.source.referenceRow ? refTarget(l.src, sh.sources) : null;
			const e = sh.tableAt(sh.active.row, sh.active.col);
			if (t) sh.say(refMessage(e ? e.t.columns[sh.active.col - e.t.col].name : d.target, t.table));
			else sh.say(tr('grid.validation.list.source', { src: l.src }));
		} else sh.say(tr('grid.validation.set'));
	}

	function startPick() {
		if (!sh.validDlg) return;
		sh.dvPick = { active: { ...sh.active }, anchor: { ...sh.anchor }, whole: sh.wholeSel, sheet: sh.sheetId() };
		sh.dvPickRef = pickedRef() ?? '';
		sh.wrap.focus();
		sh.say(tr('grid.validation.pick'));
	}

	/**
	 * Référence de la sélection pointée : Tableau[Colonne] (cellules de la
	 * colonne ou son en-tête), Tableau[Colonne][a:b] ou Cell[A1:A10] ; sur une
	 * autre feuille que celle de la cellule, Feuille2!Cell[A1:A10].
	 */
	function pickedRef(): string | null {
		const ref = rangeRef(sh.selection(), sh.refTables());
		return ref !== null && sh.dvPick && sh.dvPick.sheet !== sh.sheetId() ? qualifyRef(ref, sh.sheetName) : ref;
	}

	function endPick(keep: boolean) {
		const p = sh.dvPick;
		if (!p || p.back) return;
		const ref = keep ? pickedRef() : null;
		if (keep && ref === null) {
			sh.say(tr('grid.validation.pick.oneColumn'));
			return;
		}
		if (p.sheet !== sh.sheetId() && sh.onsheet) {
			// Retour à la feuille de la cellule : fini quand elle est relue (applyData).
			sh.dvPick = { ...p, back: true, ref };
			sh.onsheet(p.sheet);
			return;
		}
		finishPick(p, ref);
	}

	/** Sélection d'avant rétablie, référence reprise dans le dialogue. */
	function finishPick(p: DvPick, ref: string | null | undefined) {
		sh.dvPick = null;
		sh.active = p.active;
		sh.anchor = p.anchor;
		sh.wholeSel = p.whole;
		sh.ensureVisible(p.active.row, p.active.col);
		sh.schedule();
		sh.refreshStatus();
		const d = sh.validDlg;
		if (d && ref) {
			d.src = ref;
			d.mode = 'range';
			d.error = '';
			refreshPreview(0);
		}
	}

	/** Saisie refusée par la validation de la cellule : le message, sinon null (value : la valeur de la liste). */
	function refusal(row: number, col: number, text: string): { message: string } | { value: string } {
		if (text.trimStart().startsWith('=')) return { value: text }; // une formule n'est pas vérifiée
		// Validation sous forme objet (nombre, date, formule, alerte…) : vérifiée par le moteur (dvAlert).
		if (!dvLegacy(dvAt(row, col))) return { value: text };
		const src = dvSrcAt(row, col);
		const r = src ? validateSource(src, sh.dvLists.get(src), text) : validate(listOf(row, col), text);
		return r.ok ? { value: r.value } : { message: r.message };
	}

	function answerDvAlert(a: 'yes' | 'retry' | 'cancel') {
		const d = sh.dvAlert;
		sh.dvAlert = null;
		sh.wrap.focus();
		if (!d) return;
		if (a === 'yes') {
			void sh.write(d.row, d.col, d.raw, d.keep, undefined, 'dv');
			return;
		}
		if (a === 'retry') {
			// Le texte tapé revient dans l'éditeur.
			sh.active = { row: d.row, col: d.col };
			sh.anchor = { ...sh.active };
			sh.ensureVisible(d.row, d.col);
			sh.startEdit(true, d.raw);
		}
	}

	// Entourer les données non valides : cellules relues après chaque mise à jour.
	let circlesOn = false;
	let dvCircles: { row: number; col: number }[] = [];
	let circleSeq = 0;

	async function refreshCircles(announce = false) {
		const get = sh.source.invalidCells;
		if (!get) return;
		const seq = ++circleSeq;
		let cells: { row: number; col: number }[] = [];
		try {
			cells = (await get()) ?? [];
		} catch (err) {
			sh.say(errorText(err));
		}
		if (seq !== circleSeq || !circlesOn) return;
		dvCircles = cells;
		if (announce) sh.say(cells.length ? tr('grid.validation.circles.found', { n: cells.length }) : tr('grid.validation.circles.none'));
		sh.schedule();
	}

	/** Données › Validation ▾ › Entourer les données non valides. */
	function circleInvalid() {
		circlesOn = true;
		void refreshCircles(true);
	}

	/** Données › Validation ▾ › Effacer les cercles. */
	function clearCircles() {
		circlesOn = false;
		circleSeq++;
		dvCircles = [];
		sh.schedule();
	}

	/** Feuille relue : cercles relus (effacés en changeant de feuille). */
	function dvAfterData(switching: boolean) {
		if (switching) clearCircles();
		else if (circlesOn) void refreshCircles();
	}

	/** Cercles rouges autour des données non valides, et message de saisie de la cellule active. */
	function drawValidation() {
		if (dvCircles.length) {
			sh.ctx.save();
			sh.ctx.beginPath();
			sh.ctx.rect(sh.MARGIN, TOP_H, sh.viewW - sh.MARGIN, sh.viewH - TOP_H);
			sh.ctx.clip();
			sh.ctx.strokeStyle = '#e02020';
			sh.ctx.lineWidth = 1.5;
			for (const { row, col } of dvCircles) {
				const h = sh.rh(row);
				const w = sh.cax().size(col);
				const x = sh.screenX(col);
				const y = sh.screenY(row);
				if (h <= 0 || w <= 0 || x > sh.viewW || y > sh.viewH || x + w < sh.MARGIN || y + h < TOP_H) continue;
				sh.ctx.beginPath();
				sh.ctx.ellipse(x + w / 2, y + h / 2, w / 2 + 3, h / 2 + 2, 0, 0, Math.PI * 2);
				sh.ctx.stroke();
			}
			sh.ctx.restore();
		}
		const inp = sh.validDlg || sh.dvAlert || sh.dvPick ? null : dvInputOf(dvAt(sh.active.row, sh.active.col));
		let tip: InputTip | null = null;
		if (inp) {
			const y = sh.screenY(sh.active.row) + sh.rh(sh.active.row) + 4;
			const x = sh.screenX(sh.active.col) + Math.min(24, sh.cax().size(sh.active.col) / 2);
			if (y > TOP_H && y < sh.viewH && x > sh.MARGIN && x < sh.viewW) tip = { x: sh.toView(Math.min(x, sh.viewW - 250)), y: sh.toView(y), ...inp };
		}
		if (JSON.stringify(tip) !== JSON.stringify(sh.inputTip)) sh.inputTip = tip;
	}

	return {
		openValidation,
		circleInvalid,
		clearCircles,
		dvAt,
		listOf,
		prefetchDv,
		dropButton,
		asTableHint,
		tableHintAt,
		openList,
		pickFromList,
		refAt,
		followRef,
		loadRefItems,
		openCitation,
		linkButton,
		refreshPreview,
		applyValidation,
		startPick,
		pickedRef,
		endPick,
		finishPick,
		refusal,
		answerDvAlert,
		dvAfterData,
		drawValidation
	};
}
