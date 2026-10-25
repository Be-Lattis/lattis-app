// Choix du collage spécial (Ctrl+Alt+V), comme ceux du modèle
// (internal/canvas/pastespecial.go, PasteOptions).

export type PasteWhat = 'all' | 'values' | 'formulas' | 'formats' | 'valuesNumbers' | 'widths';
export type PasteOp = '' | 'add' | 'sub' | 'mul' | 'div';

export interface PasteOptions {
	what: PasteWhat;
	op: PasteOp;
	skipBlanks: boolean;
	transpose: boolean;
}

/** Entrées directes du menu Coller ▾ et du clic droit ; 'dialog' ouvre la boîte. */
export type PasteKind = 'dialog' | 'values' | 'formulas' | 'formats' | 'transpose';

export const PASTE_WHATS: PasteWhat[] = ['all', 'values', 'formulas', 'formats', 'valuesNumbers', 'widths'];
export const PASTE_OPS: PasteOp[] = ['', 'add', 'sub', 'mul', 'div'];

export function pasteOptions(kind: Exclude<PasteKind, 'dialog'>): PasteOptions {
	const o: PasteOptions = { what: 'all', op: '', skipBlanks: false, transpose: false };
	if (kind === 'transpose') o.transpose = true;
	else o.what = kind;
	return o;
}

/** Collage ordinaire (tout, sans option). */
export function plainPaste(o: PasteOptions): boolean {
	return o.what === 'all' && !o.op && !o.skipBlanks && !o.transpose;
}
