// Notes (Révision) et commentaires en fil : post-it au survol, édition,
// épinglage. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { untrack } from 'svelte';
import { type AxisLike } from '../anim';
import { type CellStyle } from '../style';
import { type MergeRange } from '../sheetfmt';
import { type CellRange } from '../mergesel';
import { threadAt, type CommentHost } from '../comments';
import { t as tr } from '#lib/i18n/index.js';
import { type SheetSource, type Entry, type Layer } from './model';

/** Ce que createNotes lit et modifie dans Sheet.svelte. */
export interface NotesHost {
	readonly mergeOf: (r: number, c: number) => MergeRange | undefined;
	readonly styleAt: (r: number, c: number, e?: Entry | undefined) => CellStyle | undefined;
	noteDlg: { row: number; col: number; text: string; had: boolean; pinned: boolean; x: number; y: number } | null;
	noteTip: { x: number; y: number; text: string; key: string } | null;
	readonly key: (r: number, c: number) => string;
	readonly screenX: (c: number) => number;
	readonly cax: () => AxisLike;
	readonly viewW: number;
	readonly screenY: (r: number) => number;
	readonly source: SheetSource;
	readonly locked: string;
	readonly say: (m: string) => void;
	readonly closeMenus: () => void;
	readonly active: { row: number; col: number; };
	readonly toView: (v: number) => number;
	readonly wrap: HTMLDivElement;
	readonly menuAction: (fn: (() => Promise<unknown>) | undefined, keepFocus?: boolean) => Promise<boolean>;
	readonly copyFill: (down: boolean) => void;
	readonly clearSelection: (range?: CellRange, all?: boolean) => Promise<void>;
	lastRepeat: (() => void) | null;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	readonly cells: Map<string, string>;
	readonly inRangeZone: (row: number, col: number) => boolean;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly comments: CommentHost | undefined;
	readonly mounted: boolean;
	readonly schedule: () => void;
	readonly refreshStatus: () => void;
	readonly ctx: CanvasRenderingContext2D;
	commentDlg: { row: number; col: number; x: number; y: number; preview: boolean } | null;
}

export function createNotes(h: NotesHost) {

	/** Note de la cellule, s'il y en a une. */
	function noteOf(r: number, c: number): string | undefined {
		const g = h.mergeOf(r, c);
		if (g) return h.styleAt(g.row, g.col, undefined)?.nt;
		return h.styleAt(r, c)?.nt;
	}

	/** Infobulle de la note sous la souris (comme Excel). */
	function updateNoteTip(cell: { row: number; col: number } | null) {
		const text = cell && !h.noteDlg ? noteOf(cell.row, cell.col) : undefined;
		if (!cell || !text) {
			if (h.noteTip) h.noteTip = null;
			return;
		}
		const g = h.mergeOf(cell.row, cell.col);
		const r = g?.row ?? cell.row;
		const c = g?.col ?? cell.col;
		const k = h.key(r, c);
		if (h.noteTip?.key === k) return;
		const right = h.screenX(c) + h.cax().offset(c + (g?.cols ?? 1)) - h.cax().offset(c);
		h.noteTip = { x: Math.min(right + 8, h.viewW - 240), y: h.screenY(r), text, key: k };
	}

	/** Révision › Nouvelle note (ou Modifier la note) sur la cellule active. */
	function editNote() {
		if (!h.source.setNote) return;
		if (h.locked) {
			h.say(h.locked);
			return;
		}
		h.closeMenus();
		h.noteTip = null;
		const text = noteOf(h.active.row, h.active.col) ?? '';
		const g = h.mergeOf(h.active.row, h.active.col);
		const pinned = !!(g ? h.styleAt(g.row, g.col, undefined) : h.styleAt(h.active.row, h.active.col))?.np;
		const right = h.screenX(h.active.col) + h.cax().size(h.active.col);
		h.noteDlg = { row: h.active.row, col: h.active.col, text, had: !!text, pinned, x: Math.min(h.toView(right) + 8, Math.max(8, h.toView(h.viewW) - 270)), y: Math.max(4, h.toView(h.screenY(h.active.row))) };
	}

	// Enregistrement automatique (comme un post-it) : à la fermeture, par
	// clic ailleurs ou Échap ; la corbeille supprime la note.
	function saveNote(remove = false) {
		const d = h.noteDlg;
		h.noteDlg = null;
		h.wrap.focus();
		const set = h.source.setNote;
		if (!d || !set) return;
		const text = remove ? '' : d.text;
		const before = noteOf(d.row, d.col) ?? '';
		if (text.trim() === before.trim()) return;
		if (text.trim() === '' && !d.had) return;
		h.menuAction(() => set(d.row, d.col, text));
	}

	/** Épingle ou désépingle la note en cours (enregistrée d'abord si elle est neuve). */
	function togglePin() {
		const d = h.noteDlg;
		const pin = h.source.pinNote;
		const set = h.source.setNote;
		if (!d || !pin || !set) return;
		if (!d.text.trim()) {
			h.say(tr('grid.note.pinEmpty'));
			return;
		}
		d.pinned = !d.pinned;
		const { row, col, text, pinned } = d;
		const changed = text.trim() !== (noteOf(row, col) ?? '').trim();
		d.had = true;
		h.menuAction(async () => {
			if (changed) await set(row, col, text);
			await pin(row, col, pinned);
		});
	}

	/**
	 * Accueil › Édition (Remplissage, Effacer, Trier et filtrer) et Mettre
	 * sous forme de tableau (Ctrl+L) : sur la sélection ou la cellule active.
	 */
	function editActive(what: 'fillDown' | 'fillRight' | 'clearContents' | 'clearFilters' | 'formatAsTable') {
		if (h.locked) {
			h.say(h.locked);
			return;
		}
		const { row, col } = h.active;
		switch (what) {
			case 'fillDown':
			case 'fillRight':
				h.copyFill(what === 'fillDown');
				return;
			case 'clearContents':
				h.clearSelection();
				h.lastRepeat = () => h.clearSelection();
				return;
			case 'clearFilters': {
				const e = h.tableAt(row, col);
				const clear = h.source.clearFilters;
				if (!e || !clear || !e.t.columns.some((c) => c.filter)) h.say(tr('grid.filter.notFiltered'));
				else h.menuAction(() => clear(e.t));
				return;
			}
			case 'formatAsTable': {
				const fmt = h.source.formatAsTable;
				if (!fmt) return;
				if (h.tableAt(row, col)) {
					h.say(tr('grid.table.alreadyIn'));
					return;
				}
				// Rien de visé (cellule vide hors d'une plage) : le moteur pose un
				// tableau vide d'une colonne « Colonne 1 », en-tête sur la cellule ;
				// la 1re cellule de données, dessous, devient active.
				const blank = !h.cells.has(h.key(row, col)) && !h.inRangeZone(row, col);
				void h.menuAction(() => fmt(row, col)).then((ok) => {
					if (ok && blank) h.moveTo(row + 1, col);
				});
				return;
			}
		}
	}

	/** Révision › Supprimer la note de la cellule active. */
	function deleteNote() {
		const set = h.source.setNote;
		if (!set || !noteOf(h.active.row, h.active.col)) {
			h.say(tr('grid.note.none'));
			return;
		}
		const { row, col } = h.active;
		h.menuAction(() => set(row, col, ''));
	}

	// Fils relus (ajout, réponse, collègue) : coins redessinés, ruban à jour.
	$effect(() => {
		void h.comments?.threads;
		untrack(() => {
			if (!h.mounted) return;
			h.schedule();
			h.refreshStatus();
		});
	});

	/** Coin violet des cellules qui ont un fil (pâle : résolu), comme le coin rouge des notes. */
	function drawCommentMarks(L: Layer) {
		for (const th of h.comments?.threads ?? []) {
			if (th.row < L.r0 || th.row > L.r1 || th.col < L.c0 || th.col > L.c1) continue;
			const g = h.mergeOf(th.row, th.col);
			const xr = L.cx(th.col + (g?.cols ?? 1));
			const y = L.ry(th.row);
			h.ctx.fillStyle = th.resolved ? '#c4b3e3' : '#7b4fc9';
			h.ctx.beginPath();
			h.ctx.moveTo(xr - 9, y + 1);
			h.ctx.lineTo(xr - 1, y + 1);
			h.ctx.lineTo(xr - 1, y + 9);
			h.ctx.closePath();
			h.ctx.fill();
		}
	}

	/** Place du fil de la cellule (à droite de la cellule ou de sa fusion). */
	function commentBox(row: number, col: number) {
		const g = h.mergeOf(row, col);
		const r = g?.row ?? row;
		const c = g?.col ?? col;
		const right = h.screenX(c) + h.cax().offset(c + (g?.cols ?? 1)) - h.cax().offset(c);
		return { row: r, col: c, x: Math.min(h.toView(right) + 8, Math.max(8, h.toView(h.viewW) - 300)), y: Math.max(4, h.toView(h.screenY(r))) };
	}

	/** Aperçu du fil sous la souris. */
	function updateCommentTip(cell: { row: number; col: number } | null) {
		if (h.commentDlg && !h.commentDlg.preview) return;
		const g = cell ? h.mergeOf(cell.row, cell.col) : undefined;
		const th = cell && h.comments ? threadAt(h.comments.threads, g?.row ?? cell.row, g?.col ?? cell.col) : undefined;
		if (!th || !cell) {
			if (h.commentDlg) h.commentDlg = null;
			return;
		}
		if (h.commentDlg?.row === th.row && h.commentDlg.col === th.col) return;
		h.commentDlg = { ...commentBox(th.row, th.col), preview: true };
	}

	/** Révision › Nouveau commentaire (ou Répondre), Ctrl+Alt+M : le fil de la cellule active. */
	function newComment() {
		if (!h.comments) return;
		h.closeMenus();
		h.commentDlg = { ...commentBox(h.active.row, h.active.col), preview: false };
	}

	/** Révision › Supprimer le commentaire : le fil de la cellule active. */
	function deleteComment() {
		const g = h.mergeOf(h.active.row, h.active.col);
		const row = g?.row ?? h.active.row;
		const col = g?.col ?? h.active.col;
		if (!h.comments || !threadAt(h.comments.threads, row, col)) {
			h.say(tr('grid.comment.none'));
			return;
		}
		const host = h.comments;
		h.menuAction(() => host.removeThread(row, col));
	}

	/** La cellule active a-t-elle un fil (ruban : Répondre, Supprimer) ? */
	function activeThread(): boolean {
		const g = h.mergeOf(h.active.row, h.active.col);
		return !!h.comments && !!threadAt(h.comments.threads, g?.row ?? h.active.row, g?.col ?? h.active.col);
	}

	/** Volet Commentaires : aller à la cellule d'un fil et l'ouvrir. */
	function showComment(row: number, col: number) {
		h.moveTo(row, col);
		queueMicrotask(() => newComment());
	}

	return {
		editNote,
		editActive,
		deleteNote,
		newComment,
		deleteComment,
		activeThread,
		showComment,
		noteOf,
		updateNoteTip,
		saveNote,
		togglePin,
		drawCommentMarks,
		updateCommentTip
	};
}
