// Mise en forme d'une cellule écrite en CSS (impression, export PDF) : la
// même lecture du style que la grille (style.ts, textwrap.ts, colors.ts),
// pour que la page imprimée ressemble à l'écran.
import type { CellStyle } from '#lib/grid/style.js';
import { paint } from '#lib/grid/colors.js';
import { fontFamily as gridFamily, fontPx, lineHeight } from '#lib/grid/textwrap.js';
import { edgeCss, LINE_STYLES, parseEdge } from '#lib/grid/borders.js';

/** Couleur CSS d'une couleur de style (référence au thème résolue), ou undefined. */
export function cssColor(v: unknown): string | undefined {
	if (typeof v !== 'string' || !v) return undefined;
	const c = paint(v);
	return typeof c === 'string' && /^(#[0-9a-f]{3,8}|rgba?\([\d\s.,%]+\)|[a-z]+)$/i.test(c) ? c : undefined;
}

const WIDTHS: Record<string, string> = { hair: '0.5px', thin: '1px', medium: '2px', thick: '3px' };
const STYLES = new Set(['solid', 'dashed', 'dotted', 'double', 'groove', 'ridge', 'inset', 'outset']);

/**
 * Trait CSS d'un bord : vrai (trait fin, comme la grille), ou une valeur
 * plus riche d'une version plus récente du style (« medium dashed #c00 »,
 * « thick », « 2px dotted », { style, color, width }) ; color : couleur des
 * bordures (bc), sinon noir. undefined : pas de bord.
 */
export function borderCss(v: unknown, color?: unknown): string | undefined {
	if (!v) return undefined;
	const fallback = cssColor(color) ?? '#000';
	if (v === true) return `1px solid ${fallback}`;
	// Bord de la grille (borders.ts) : « style couleur » (« double #c00000 »).
	if (typeof v === 'string') {
		const [a, b, more] = v.trim().split(/\s+/);
		if ((LINE_STYLES as string[]).includes(a) && !more && (!b || !(STYLES.has(b.toLowerCase()) || WIDTHS[b.toLowerCase()]))) {
			const e = parseEdge(v);
			if (e) return edgeCss(e, cssColor(e.color) ?? fallback);
		}
	}
	if (typeof v === 'string' && /^\s*none\s*$/i.test(v)) return undefined;
	let width = '';
	let style = '';
	let col = '';
	const take = (tok: string) => {
		const t = tok.trim();
		if (!t) return;
		const low = t.toLowerCase();
		if (WIDTHS[low]) width = WIDTHS[low];
		else if (/^\d+(\.\d+)?(px|pt)$/.test(low)) width = low;
		else if (STYLES.has(low)) style = low;
		else if (/^(mediumdashed|dashdot|mediumdashdot|dashdotdot|slantdashdot)$/.test(low)) {
			// Noms d'un fichier .xlsx : tirets, plus épais quand « medium ».
			style = 'dashed';
			if (low.startsWith('medium')) width ||= '2px';
		} else if (low === 'hairline') width = '0.5px';
		else if (cssColor(t)) col = cssColor(t)!;
	};
	if (typeof v === 'string') v.split(/\s+/).forEach(take);
	else if (typeof v === 'object') {
		const o = v as Record<string, unknown>;
		for (const k of ['width', 'w', 'style', 's', 'color', 'c']) if (typeof o[k] === 'string') take(o[k] as string);
	} else return `1px solid ${fallback}`;
	return `${width || '1px'} ${style || 'solid'} ${col || fallback}`;
}

const SIDES = [
	['t', 'top'],
	['b', 'bottom'],
	['l', 'left'],
	['r', 'right']
] as const;

/** Bords d'une cellule (bt, bb, bl, br et leur couleur bc ; border_top… s'ils existent). */
export function bordersCss(st: CellStyle | undefined): string[] {
	if (!st) return [];
	const s = st as Record<string, unknown>;
	const out: string[] = [];
	for (const [k, side] of SIDES) {
		const v = s['b' + k] ?? s['border_' + side];
		const b = borderCss(v, s['bc' + k] ?? s.bc);
		if (b) out.push(`border-${side}:${b}`);
	}
	return out;
}

/** Police d'une cellule : la famille choisie (ff, ou font), sinon celle de la grille. */
export function fontFamily(st: CellStyle | undefined): string {
	const s = st as Record<string, unknown> | undefined;
	const f = typeof s?.ff === 'string' && s.ff ? s.ff : typeof s?.font === 'string' && s.font ? s.font : '';
	return gridFamily(f ? f.replace(/["\\;{}<>]/g, '') : undefined);
}

/** Présentation d'une cellule imprimée, en plus de son style. */
export interface CellLook {
	align?: 'left' | 'center' | 'right';
	/** Couleur du texte sans couleur choisie (en-tête, erreur). */
	color?: string;
	/** Fond sans remplissage choisi (en-tête de tableau, tableau croisé). */
	fill?: string;
	/** Gras par défaut (en-têtes). */
	bold?: boolean;
}

/**
 * Déclarations CSS d'une cellule : fond, texte (couleur, police, taille,
 * gras, italique, souligné, barré), alignements, renvoi à la ligne,
 * bordures. Vide : la cellule par défaut.
 */
export function cellCss(st: CellStyle | undefined, look: CellLook = {}): string {
	const out: string[] = [];
	const bg = cssColor(st?.bg) ?? look.fill;
	if (bg) out.push(`background:${bg}`);
	// Lien hypertexte : bleu souligné, comme à l'écran (une couleur choisie l'emporte).
	const link = typeof st?.ln === 'string' && /^(https?:\/\/|mailto:|#)/i.test(st.ln);
	const fg = cssColor(st?.fg) ?? (link ? '#0563c1' : look.color);
	if (fg) out.push(`color:${fg}`);
	const s = st as Record<string, unknown> | undefined;
	if (st?.ff || s?.font) out.push(`font-family:${fontFamily(st)}`);
	if (st?.fs) {
		const px = fontPx(st.fs);
		out.push(`font-size:${px}px`, `line-height:${lineHeight(px)}px`);
	}
	if (st?.b ?? look.bold) out.push('font-weight:600');
	if (st?.i) out.push('font-style:italic');
	const deco = [(st?.u || link) && 'underline', st?.x && 'line-through'].filter(Boolean);
	if (deco.length) out.push(`text-decoration:${deco.join(' ')}`);
	const al = st?.al ?? look.align;
	if (al && al !== 'left') out.push(`text-align:${al}`);
	if (st?.va === 'top' || st?.va === 'bottom') out.push(`vertical-align:${st.va}`);
	if (st?.wr) out.push('white-space:pre-wrap', 'overflow-wrap:anywhere');
	out.push(...bordersCss(st));
	return out.join(';');
}
