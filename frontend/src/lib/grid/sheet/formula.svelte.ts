// Saisie d'une formule : complétion et aide de signature, formule gardée
// pendant qu'on pointe une référence sur une autre feuille. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { flushSync } from 'svelte';
import { apply as applySuggestion, signature, suggest, type Catalog, type Completion, type FunctionDoc } from '../autocomplete';
import { type FormulaError } from '../formulaEdit';
import { holdFormula, updateHold, type FormulaHold } from '../crossref';
import { t as tr } from '#lib/i18n/index.js';
import { type TextField, type Entry } from './model';

/** Ce que createFormula lit et modifie dans Sheet.svelte. */
export interface FormulaHost {
	editErr: FormulaError | null;
	fxText: string;
	readonly placeEditor: () => void;
	readonly catalog: Catalog | undefined;
	suggestPos: { x: number; y: number; };
	readonly toView: (v: number) => number;
	readonly screenX: (c: number) => number;
	active: { row: number; col: number; };
	readonly screenY: (r: number) => number;
	readonly editorH: number;
	readonly rh: (r: number) => number;
	readonly tableAt: (r: number, c: number) => Entry | undefined;
	completion: (Completion & { sel: number; where: 'cell' | 'fx' }) | null;
	sig: { fn: FunctionDoc; index: number; where: 'cell' | 'fx' } | null;
	hold: FormulaHold | null;
	fxEditing: boolean;
	readonly fxInput: HTMLInputElement | undefined;
	editing: boolean;
	readonly editRO: string;
	readonly input: HTMLTextAreaElement | undefined;
	readonly editValueText: string;
	lastRef: { el: TextField; start: number; end: number; value: string } | null;
	readonly placedSheet: number | undefined;
	readonly say: (m: string) => void;
	anchor: { row: number; col: number; };
	readonly refreshFx: () => void;
	readonly wrap: HTMLDivElement;
	readonly schedule: () => void;
	readonly startEdit: (byTyping: boolean, initial?: string) => void;
	keepEditorUntil: number;
	readonly commit: () => boolean;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly onsheet: ((sheet: number) => void) | undefined;
}

export function createFormula(sh: FormulaHost) {
	function onFormulaInput(el: TextField, where: 'cell' | 'fx') {
		sh.editErr = null;
		if (where === 'cell') {
			sh.fxText = el.value;
			sh.placeEditor();
			// Une formule se lit de gauche à droite, quelle que soit la colonne.
			if (el.value.startsWith('=')) el.style.textAlign = 'left';
		}
		if (!sh.catalog) return;
		sh.suggestPos = { x: sh.toView(sh.screenX(sh.active.col)), y: sh.toView(sh.screenY(sh.active.row) + Math.max(sh.editorH, sh.rh(sh.active.row))) + 2 };
		const caret = el.selectionStart ?? el.value.length;
		const e = sh.tableAt(sh.active.row, sh.active.col);
		const c = suggest(el.value, caret, sh.catalog, e?.t.name);
		sh.completion = c ? { ...c, sel: 0, where } : null;
		const s = signature(el.value, caret, sh.catalog);
		sh.sig = s ? { ...s, where } : null;
	}

	/** Le curseur bouge sans saisie (flèches, clic) : l'argument en gras suit. */
	function moveSig(e: Event, where: 'cell' | 'fx') {
		if (!sh.catalog || (e instanceof KeyboardEvent && !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key))) return;
		const el = e.currentTarget as TextField;
		const s = signature(el.value, el.selectionStart ?? el.value.length, sh.catalog);
		sh.sig = s ? { ...s, where } : null;
	}

	/** Gère les touches de la liste de suggestions ; vrai si la touche est consommée. */
	function completionKey(e: KeyboardEvent, el: TextField, set: (v: string) => void): boolean {
		if (!sh.completion) return false;
		const c = sh.completion;
		switch (e.key) {
			case 'ArrowDown':
				sh.completion = { ...c, sel: (c.sel + 1) % c.items.length };
				break;
			case 'ArrowUp':
				sh.completion = { ...c, sel: (c.sel + c.items.length - 1) % c.items.length };
				break;
			case 'Tab':
			case 'Enter': {
				const caret = el.selectionStart ?? el.value.length;
				const r = applySuggestion(el.value, caret, c.from, c.items[c.sel]);
				set(r.text);
				sh.completion = null;
				requestAnimationFrame(() => {
					el.setSelectionRange(r.caret, r.caret);
					onFormulaInput(el, c.where);
				});
				break;
			}
			case 'Escape':
				sh.completion = null;
				break;
			default:
				return false;
		}
		e.preventDefault();
		e.stopPropagation();
		return true;
	}

	/** La feuille from est quittée : une formule en cours de saisie est gardée. */
	function keepFormula(from: number) {
		if (sh.hold) {
			// Déjà sur une autre feuille : on en change encore, la saisie suit.
			if (sh.fxEditing && sh.fxInput) sh.hold = updateHold(sh.hold, sh.fxText, sh.fxInput.selectionStart, sh.fxInput.selectionEnd);
			return;
		}
		if (!sh.editing && !sh.fxEditing) return;
		// Éditeur en lecture seule : rien à garder, il se referme.
		if (sh.editing && sh.editRO) {
			sh.editing = false;
			return;
		}
		const el = sh.editing ? sh.input : sh.fxInput;
		const text = sh.editing ? sh.editValueText : sh.fxText;
		const h = holdFormula({ sheet: from, row: sh.active.row, col: sh.active.col, where: sh.editing ? 'cell' : 'fx' }, text, el?.selectionStart ?? null, el?.selectionEnd ?? null);
		if (!h) return;
		sh.hold = h;
		sh.editing = false;
		sh.editErr = null;
		sh.completion = sh.sig = null;
		sh.lastRef = null;
	}

	/** Après un changement de feuille, la saisie gardée : dans la barre de formule, ou de retour dans sa cellule. */
	function placeHold(sheet: number) {
		const h = sh.hold;
		if (!h || sheet !== sh.placedSheet) return;
		if (sheet !== h.sheet) {
			// Autre feuille : la formule dans la barre de formule, curseur gardé.
			flushSync(() => {
				sh.fxText = h.text;
				sh.fxEditing = true;
			});
			sh.fxInput?.focus();
			sh.fxInput?.setSelectionRange(h.start, h.end);
			if (sh.fxInput) onFormulaInput(sh.fxInput, 'fx');
			sh.say(tr('grid.formula.pickHint'));
			return;
		}
		// Retour à la feuille de la formule.
		sh.hold = null;
		sh.lastRef = null;
		sh.fxEditing = false;
		sh.completion = sh.sig = null;
		sh.active = { row: h.row, col: h.col };
		sh.anchor = { ...sh.active };
		if (h.finish === 'cancel') {
			sh.refreshFx();
			sh.wrap.focus();
			sh.schedule();
			return;
		}
		if (h.where === 'fx' && !h.finish) {
			// Saisie commencée dans la barre de formule : elle y reprend.
			flushSync(() => {
				sh.fxText = h.text;
				sh.fxEditing = true;
			});
			sh.fxInput?.focus();
			sh.fxInput?.setSelectionRange(h.start, h.end);
			sh.schedule();
			return;
		}
		sh.startEdit(true, h.text);
		if (!sh.editing || !sh.input) return;
		sh.input.setSelectionRange(h.start, h.end);
		onFormulaInput(sh.input, 'cell');
		// Retour pour l'argument suivant : le focus que le changement de feuille
		// remet à la grille ne doit pas valider la formule.
		if (!h.finish) sh.keepEditorUntil = performance.now() + 400;
		// Entrée sur l'autre feuille : validée dans sa cellule (une faute laisse l'éditeur ouvert).
		if (h.finish === 'commit' && sh.commit()) sh.moveTo(h.row + 1, h.col);
		sh.schedule();
	}

	/** Entrée ou Échap sur l'autre feuille : retour à la feuille de la formule, qui est validée ou abandonnée. */
	function finishHold(finish: 'commit' | 'cancel') {
		if (!sh.hold) return;
		sh.hold = { ...(sh.fxInput ? updateHold(sh.hold, sh.fxText, sh.fxInput.selectionStart, sh.fxInput.selectionEnd) : sh.hold), finish };
		sh.completion = sh.sig = null;
		sh.onsheet?.(sh.hold.sheet);
	}

	/** Reprend la saisie d'une formule restée ouverte (clic sur l'onglet affiché). */
	function resumeFormula() {
		if (sh.editing) sh.input?.focus();
		else if (sh.fxEditing) sh.fxInput?.focus();
	}

	return {
		resumeFormula,
		onFormulaInput,
		moveSig,
		completionKey,
		keepFormula,
		placeHold,
		finishHold
	};
}
