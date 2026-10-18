// Document : passage entre la mise en forme de Quill (gras,
// police, titre, liste…) et celle du ruban (CellStyle des groupes Police et
// Alignement, plus l'état du groupe Paragraphe).

import type { CellStyle } from '#lib/grid/style.js';

/** Police et taille du texte sans mise en forme (celles du ruban quand rien n'est imposé). */
export const DOC_FONT = 'Segoe UI';
export const DOC_PT = 11;

/** Mise en forme Quill à la sélection (quill.getFormat()). */
export type QuillFormat = Record<string, unknown>;

/** Bloc du paragraphe courant (groupe Paragraphe du ruban). */
export type DocBlock = 'normal' | 'h1' | 'h2' | 'h3' | 'bullet' | 'ordered' | 'quote' | 'code';

/** État de la sélection pour le ruban : style de caractère et bloc. */
export interface DocFormat {
	style: CellStyle;
	block: DocBlock;
	align: 'left' | 'center' | 'right' | 'justify';
	strike: boolean;
	link: boolean;
}

/** Taille Quill (« 14pt ») en points ; undefined : taille par défaut. */
export function sizePt(v: unknown): number | undefined {
	if (typeof v !== 'string') return undefined;
	const m = /^(\d+(?:\.\d+)?)pt$/.exec(v);
	return m ? Number(m[1]) : undefined;
}

/** Valeur unique d'un format (Quill renvoie un tableau si la sélection en mêle plusieurs). */
const one = (v: unknown) => (Array.isArray(v) ? undefined : v);

export function docFormat(f: QuillFormat): DocFormat {
	const style: CellStyle = {};
	if (f.bold === true) style.b = true;
	if (f.italic === true) style.i = true;
	if (f.underline === true) style.u = true;
	if (f.strike === true) style.x = true;
	const ff = one(f.font);
	if (typeof ff === 'string') style.ff = ff;
	const fs = sizePt(one(f.size));
	style.fs = fs ?? DOC_PT;
	const fg = one(f.color);
	if (typeof fg === 'string') style.fg = fg;
	const bg = one(f.background);
	if (typeof bg === 'string') style.bg = bg;
	const al = one(f.align);
	const align = al === 'center' || al === 'right' || al === 'justify' ? al : 'left';
	if (align !== 'justify') style.al = align;
	let block: DocBlock = 'normal';
	const h = one(f.header);
	if (h === 1 || h === 2 || h === 3) block = `h${h}` as DocBlock;
	else if (one(f.list) === 'bullet') block = 'bullet';
	else if (one(f.list) === 'ordered') block = 'ordered';
	else if (f.blockquote === true) block = 'quote';
	else if (f['code-block']) block = 'code';
	return { style, block, align, strike: !!style.x, link: typeof one(f.link) === 'string' };
}

/**
 * Formats Quill d'un correctif du ruban (onstyle : { b: 'toggle' },
 * { ff: 'Arial' }, { fs: 14 }, { fg: '#c00000' }, { al: 'center' },
 * effacement) ; cur est la mise en forme actuelle, pour les bascules.
 * Les clés sans équivalent (bordures, format de nombre…) sont ignorées.
 */
export function quillFormats(patch: Record<string, unknown>, cur: CellStyle): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	const flag = (key: 'b' | 'i' | 'u' | 'x', name: string) => {
		if (!(key in patch)) return;
		const v = patch[key];
		out[name] = v === 'toggle' ? !cur[key] : !!v;
	};
	flag('b', 'bold');
	flag('i', 'italic');
	flag('u', 'underline');
	flag('x', 'strike');
	if ('ff' in patch) out.font = patch.ff ?? false;
	if ('fs' in patch) out.size = patch.fs == null || patch.fs === DOC_PT ? false : `${patch.fs}pt`;
	if ('fg' in patch) out.color = patch.fg ?? false;
	if ('bg' in patch) out.background = patch.bg ?? false;
	if ('al' in patch) out.align = patch.al == null || patch.al === 'left' ? false : patch.al;
	return out;
}

/** Formats de bloc Quill pour un choix du groupe Paragraphe (une liste ou un titre déjà en place se retire). */
export function blockFormats(block: DocBlock, cur: DocBlock): Record<string, unknown> {
	const target = block === cur && block !== 'normal' ? 'normal' : block;
	return {
		header: target === 'h1' ? 1 : target === 'h2' ? 2 : target === 'h3' ? 3 : false,
		list: target === 'bullet' ? 'bullet' : target === 'ordered' ? 'ordered' : false,
		blockquote: target === 'quote',
		'code-block': target === 'code'
	};
}

/** Contenu enregistré d'un onglet Document (Delta Quill en JSON) ; un texte qui n'en est pas un devient un paragraphe. */
export function parseDelta(source: string): { ops: Record<string, unknown>[] } {
	try {
		const d = JSON.parse(source);
		if (d && Array.isArray(d.ops)) return d;
	} catch {
		// texte brut
	}
	return { ops: [{ insert: source.endsWith('\n') ? source : source + '\n' }] };
}
