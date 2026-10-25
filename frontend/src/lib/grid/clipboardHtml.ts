// Presse-papiers HTML : un tableur met, à côté du texte tabulé, un tableau
// HTML avec la mise en forme des cellules (styles en ligne ou classes CSS
// d'un bloc <style>). parseClipHtml le lit en valeurs et en mises en forme
// de Lattis (style.ts) ; toClipHtml écrit la sélection copiée de la même
// façon, pour qu'un collage dans un tableur garde couleurs et formats.
//
// Lecture tolérante, sans DOM (aussi dans les tests) : seules les balises
// du tableau (table, tr, td, th, br) et le bloc <style> comptent. Les règles
// d'élément (td, table…) donnent la police par défaut de la source : elles
// ne font pas de mise en forme. L'alignement vertical en bas, celui d'office
// des tableurs, n'est pas repris (Lattis centre verticalement).

import type { CellStyle } from './style';

/** Mise en forme d'une cellule collée : celle de Lattis, et des propriétés inconnues tolérées (« format »). */
export type ClipStyle = Partial<CellStyle> & { format?: string; mr?: number; mc?: number };

export interface HtmlClip {
	rows: string[][];
	/** Mise en forme de chaque cellule (null : aucune) ; fusions en mr, mc sur la cellule en haut à gauche. */
	styles: (ClipStyle | null)[][];
}

type Decls = Record<string, string>;

const ENTITIES: Record<string, string> = { amp: '&', lt: '<', gt: '>', quot: '"', apos: "'", nbsp: ' ', euro: '€', eacute: 'é', egrave: 'è', agrave: 'à', ccedil: 'ç' };

function decode(s: string): string {
	return s.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (m, e: string) => {
		if (e[0] === '#') {
			const n = e[1] === 'x' || e[1] === 'X' ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10);
			if (n === 160) return ' ';
			return Number.isFinite(n) ? String.fromCodePoint(n) : m;
		}
		return ENTITIES[e.toLowerCase()] ?? m;
	});
}

/** Déclarations CSS « a:b; c:"d;e" » (points-virgules entre guillemets compris). */
export function parseDecls(text: string): Decls {
	const out: Decls = {};
	let cur = '';
	let q = '';
	const flush = () => {
		const i = cur.indexOf(':');
		if (i > 0) out[cur.slice(0, i).trim().toLowerCase()] = cur.slice(i + 1).trim();
		cur = '';
	};
	for (let i = 0; i < text.length; i++) {
		const c = text[i];
		if (q) {
			if (c === '\\' && i + 1 < text.length) {
				cur += c + text[++i];
				continue;
			}
			if (c === q) q = '';
		} else if (c === '"' || c === "'") q = c;
		else if (c === ';') {
			flush();
			continue;
		}
		cur += c;
	}
	flush();
	return out;
}

/** Règles d'un bloc <style> : sélecteur (« .xl65 », « td ») → déclarations. */
function parseCss(css: string): Map<string, Decls> {
	const rules = new Map<string, Decls>();
	const text = css.replace(/<!--|-->/g, '').replace(/\/\*[\s\S]*?\*\//g, '');
	const re = /([^{}]+)\{([^}]*)\}/g;
	let m: RegExpExecArray | null;
	while ((m = re.exec(text))) {
		const decls = parseDecls(m[2]);
		for (const sel of m[1].split(',')) {
			const k = sel.trim().toLowerCase();
			if (k) rules.set(k, { ...(rules.get(k) ?? {}), ...decls });
		}
	}
	return rules;
}

function attrs(s: string): Record<string, string> {
	const out: Record<string, string> = {};
	const re = /([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*(?:=\s*("[^"]*"|'[^']*'|[^\s"'>]+))?/g;
	let m: RegExpExecArray | null;
	while ((m = re.exec(s))) {
		let v = m[2] ?? '';
		if (v.startsWith('"') || v.startsWith("'")) v = v.slice(1, -1);
		out[m[1].toLowerCase()] = decode(v);
	}
	return out;
}

const NAMED: Record<string, string> = {
	black: '#000000',
	windowtext: '#000000',
	white: '#ffffff',
	window: '#ffffff',
	red: '#ff0000',
	green: '#008000',
	lime: '#00ff00',
	blue: '#0000ff',
	yellow: '#ffff00',
	orange: '#ffa500',
	gray: '#808080',
	grey: '#808080',
	silver: '#c0c0c0',
	maroon: '#800000',
	navy: '#000080',
	olive: '#808000',
	purple: '#800080',
	teal: '#008080',
	aqua: '#00ffff',
	cyan: '#00ffff',
	fuchsia: '#ff00ff',
	magenta: '#ff00ff'
};

/** Couleur CSS en #rrggbb (null : aucune, transparente ou automatique). */
export function cssColor(v: string | undefined): string | null {
	if (!v) return null;
	const s = v.trim().toLowerCase();
	let m = /#([0-9a-f]{6}|[0-9a-f]{3})\b/.exec(s);
	if (m) {
		const h = m[1].length === 3 ? [...m[1]].map((c) => c + c).join('') : m[1];
		return '#' + h;
	}
	m = /rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*([\d.]+)\s*)?\)/.exec(s);
	if (m) {
		if (m[4] !== undefined && +m[4] === 0) return null;
		return '#' + [m[1], m[2], m[3]].map((x) => Math.min(255, +x).toString(16).padStart(2, '0')).join('');
	}
	for (const w of s.split(/[\s,]+/)) if (NAMED[w]) return NAMED[w];
	return null;
}

/** Taille de police en points (12px = 9pt). */
function fontSize(v: string | undefined): number | null {
	const m = v && /([\d.]+)\s*(pt|px)?/.exec(v);
	if (!m) return null;
	const n = m[2] === 'px' ? +m[1] * 0.75 : +m[1];
	return Number.isFinite(n) && n > 0 ? Math.round(n * 2) / 2 : null;
}

function fontFamily(v: string | undefined): string | null {
	if (!v) return null;
	const f = v.split(',')[0].trim().replace(/^["']|["']$/g, '');
	return f && !/^(sans-serif|serif|monospace|inherit|initial)$/i.test(f) ? f : null;
}

/** Une bordure tracée (« .5pt solid windowtext », pas « none » ni « 0 »). */
function edge(v: string | undefined): { on: boolean; color: string | null } | null {
	if (v === undefined) return null;
	const s = v.trim().toLowerCase();
	if (!s || /\b(none|hidden)\b/.test(s) || /^0(\.0+)?(pt|px)?$/.test(s)) return { on: false, color: null };
	return { on: true, color: cssColor(s) };
}

/** Code de format d'un tableur (mso-number-format, sans guillemets ni échappements). */
function unescapeCode(v: string): string {
	let s = v.trim();
	if ((s.startsWith('"') && s.endsWith('"')) || (s.startsWith("'") && s.endsWith("'"))) s = s.slice(1, -1);
	// Échappements CSS (\0022 : guillemet), puis ceux du code (\. \#).
	s = s.replace(/\\00([0-9a-f]{2})/gi, (_, h: string) => String.fromCharCode(parseInt(h, 16)));
	let prev = '';
	while (prev !== s) {
		prev = s;
		s = s.replace(/\\(.)/g, '$1');
	}
	return s;
}

/**
 * Format de nombre de Lattis (nf, dp, ng, df) d'un code de format de
 * tableur ; un code que Lattis n'a pas reste dans « format ».
 */
export function numberFormatOf(code: string): ClipStyle {
	const c = code.trim();
	const low = c.toLowerCase();
	if (!c) return {};
	switch (low) {
		case 'general':
		case 'general number':
			return {};
		case '@':
		case 'text':
			return { nf: 'text' };
		case 'percent':
			return { nf: '%', dp: 2 };
		case 'fixed':
			return { nf: '0,00', ng: true };
		case 'standard':
			return { nf: '0,00' };
		case 'short date':
		case 'medium date':
			return { nf: 'date' };
		case 'short time':
			return { nf: 'time', df: 'hh:mm' };
		case 'long time':
			return { nf: 'time', df: 'hh:mm:ss' };
	}
	// Première section seulement (positif ; « ;» sépare négatif et zéro).
	const sec = c.split(';')[0].replace(/\[[^\]]*\]/g, (m) => (m.includes('€') ? '€' : ''));
	const plain = sec.replace(/"[^"]*"/g, (m) => (m.includes('€') ? '€' : ''));
	const decimals = /0\.(0+)/.exec(plain)?.[1].length ?? 0;
	if (/[dy]/i.test(plain) || /m{3,}/i.test(plain)) {
		const df = plain
			.toLowerCase()
			.replace(/y+/g, (y) => (y.length > 2 ? 'aaaa' : 'aa'))
			.replace(/d+/g, (d) => 'j'.repeat(d.length));
		const time = /h/.test(df);
		const known = ['jj/mm/aaaa', 'jj/mm/aa', 'aaaa-mm-jj', 'mm/aaaa', 'jj/mm', 'jj mmm aaaa', 'jj mmmm aaaa', 'jjjj jj mmmm aaaa', 'mmmm aaaa', 'jj/mm/aaaa hh:mm'];
		if (time) return { nf: 'time', df: known.includes(df) ? df : 'jj/mm/aaaa hh:mm' };
		return known.includes(df) ? { nf: 'date', df } : { nf: 'date' };
	}
	if (/^\[?h+\]?:mm(:ss)?$/i.test(plain.trim())) return { nf: 'time', df: /ss/.test(plain) ? 'hh:mm:ss' : 'hh:mm' };
	if (!/[0#?]/.test(plain)) return { format: c };
	const grouped = /[#0],[#0]/.test(plain);
	if (plain.includes('%')) return decimals ? { nf: '%', dp: decimals } : { nf: '%', dp: 0 };
	if (plain.includes('€')) return decimals === 2 ? { nf: '€' } : { nf: '€', dp: decimals };
	// Autre symbole monétaire ($, £…) : format inconnu de Lattis, gardé tel quel.
	if (/[$£¥]/.test(plain)) return { format: c };
	const base: ClipStyle = decimals === 0 ? { nf: '0' } : decimals === 2 ? { nf: '0,00' } : { nf: '0,00', dp: decimals };
	if (!grouped) base.ng = true;
	return base;
}

/** Mise en forme de Lattis de déclarations CSS ; base : celles des règles d'élément (police par défaut). */
export function styleOfDecls(d: Decls, base: Decls = {}): ClipStyle | null {
	const s: ClipStyle = {};
	const weight = d['font-weight']?.toLowerCase();
	if (weight && (weight === 'bold' || weight === 'bolder' || +weight >= 600)) s.b = true;
	if (/italic|oblique/i.test(d['font-style'] ?? '')) s.i = true;
	const deco = (d['text-decoration'] ?? '') + ' ' + (d['text-decoration-line'] ?? '');
	if (/underline/i.test(deco) || (d['text-underline-style'] && !/none/i.test(d['text-underline-style']))) s.u = true;
	if (/line-through/i.test(deco) || d['text-line-through'] && !/none/i.test(d['text-line-through'])) s.x = true;
	const fg = cssColor(d['color']);
	if (fg && fg !== '#000000') s.fg = fg;
	const bgSrc = d['background-color'] ?? d['background'];
	const bg = bgSrc && /none|transparent|auto/i.test(bgSrc) ? null : cssColor(bgSrc);
	if (bg && bg !== '#ffffff') s.bg = bg;
	const fs = fontSize(d['font-size']);
	if (fs && fs !== fontSize(base['font-size'])) s.fs = fs;
	const ff = fontFamily(d['font-family']);
	if (ff && ff.toLowerCase() !== (fontFamily(base['font-family']) ?? '').toLowerCase()) s.ff = ff;
	const al = (d['text-align'] ?? '').toLowerCase();
	if (al === 'left' || al === 'center' || al === 'right') s.al = al;
	else if (al === 'centre') s.al = 'center';
	const va = (d['vertical-align'] ?? '').toLowerCase();
	if (va === 'top') s.va = 'top';
	const ws = (d['white-space'] ?? '').toLowerCase();
	if (ws === 'normal' || ws === 'pre-wrap' || ws === 'break-spaces' || /break-word/i.test(d['word-wrap'] ?? '') && ws !== 'nowrap') s.wr = true;
	const all = edge(d['border']);
	let bc: string | null = null;
	for (const [k, side] of [
		['bt', 'top'],
		['bb', 'bottom'],
		['bl', 'left'],
		['br', 'right']
	] as const) {
		const e = edge(d['border-' + side]) ?? all;
		if (e?.on) {
			s[k] = true;
			bc ??= e.color;
		}
	}
	if (bc && bc !== '#000000') s.bc = bc;
	const code = d['mso-number-format'];
	if (code) Object.assign(s, numberFormatOf(unescapeCode(code)));
	return Object.keys(s).length ? s : null;
}

/** Marque d'un <br> dans le texte d'une cellule, avant la réduction des blancs. */
const BR = '\u0001';

interface Cell {
	text: string;
	decls: Decls;
	cols: number;
	rows: number;
	/** Lien <a href> de la cellule (le classeur vérifie l'adresse). */
	href?: string;
}

/**
 * Lit le tableau HTML d'un presse-papiers (le premier, ou le seul) ; null
 * sans tableau.
 */
export function parseClipHtml(html: string): HtmlClip | null {
	if (!html || !/<t[dh][\s>]/i.test(html)) return null;
	const css = new Map<string, Decls>();
	for (const m of html.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gi)) for (const [k, v] of parseCss(m[1])) css.set(k, { ...(css.get(k) ?? {}), ...v });
	const body = html.replace(/<style[^>]*>[\s\S]*?<\/style>/gi, '').replace(/<!--[\s\S]*?-->/g, '');
	// Police par défaut de la source : règles d'élément et style du tableau.
	let base: Decls = { ...(css.get('table') ?? {}), ...(css.get('td') ?? {}) };
	const lines: Cell[][] = [];
	let line: Cell[] | null = null;
	let cell: Cell | null = null;
	let depth = 0; // tableaux imbriqués : seul le premier compte
	let done = false;
	const re = /<(\/?)([a-zA-Z][a-zA-Z0-9:-]*)([^>]*)>|([^<]+)/g;
	let m: RegExpExecArray | null;
	const close = () => {
		if (cell && line) line.push(cell);
		cell = null;
	};
	while (!done && (m = re.exec(body))) {
		if (m[4] !== undefined) {
			if (cell && depth === 1) cell.text += m[4];
			continue;
		}
		const end = m[1] === '/';
		const tag = m[2].toLowerCase();
		if (tag === 'table') {
			if (!end) {
				depth++;
				if (depth === 1) base = { ...base, ...parseDecls(attrs(m[3]).style ?? '') };
			} else if (--depth === 0) {
				close();
				done = true;
			}
			continue;
		}
		if (depth !== 1) continue;
		if (tag === 'tr') {
			close();
			if (!end) lines.push((line = []));
			else line = null;
		} else if (tag === 'td' || tag === 'th') {
			close();
			if (end) continue;
			if (!line) lines.push((line = []));
			const a = attrs(m[3]);
			let decls: Decls = tag === 'th' ? { 'font-weight': 'bold' } : {};
			for (const k of (a.class ?? '').split(/\s+/)) if (k) decls = { ...decls, ...(css.get('.' + k.toLowerCase()) ?? css.get('td.' + k.toLowerCase()) ?? {}) };
			if (a.align) decls['text-align'] = a.align;
			if (a.valign) decls['vertical-align'] = a.valign;
			if (a.bgcolor) decls['background-color'] = a.bgcolor;
			decls = { ...decls, ...parseDecls(a.style ?? '') };
			// Format d'un tableur en ligne (data-sheets-numberformat : {"2":"0.00"}).
			if (a['data-sheets-numberformat'] && !decls['mso-number-format']) {
				try {
					const f = JSON.parse(a['data-sheets-numberformat']) as Record<string, unknown>;
					if (typeof f['2'] === 'string') decls['mso-number-format'] = f['2'];
				} catch {
					/* format illisible : ignoré */
				}
			}
			cell = { text: '', decls, cols: Math.max(1, parseInt(a.colspan ?? '1', 10) || 1), rows: Math.max(1, parseInt(a.rowspan ?? '1', 10) || 1) };
		} else if (tag === 'br') {
			if (cell) cell.text += BR;
		} else if (cell && !end) {
			// Mise en forme posée par une balise autour du texte (<b>, <i>, <font color>).
			const a = attrs(m[3]);
			// Lien de la cellule : une adresse web ou e-mail (les autres sont laissées).
			if (tag === 'a' && a.href && /^(https?:\/\/|mailto:)/i.test(decode(a.href).trim())) cell.href ??= decode(a.href).trim();
			if (tag === 'b' || tag === 'strong') cell.decls['font-weight'] ??= 'bold';
			else if (tag === 'i' || tag === 'em') cell.decls['font-style'] ??= 'italic';
			else if (tag === 'u') cell.decls['text-decoration'] ??= 'underline';
			else if (tag === 's' || tag === 'strike') cell.decls['text-decoration'] ??= 'line-through';
			else if (tag === 'font' && a.color) cell.decls['color'] ??= a.color;
			else if (tag === 'span' && a.style) {
				const d = parseDecls(a.style);
				for (const k of ['color', 'font-weight', 'font-style', 'text-decoration']) if (d[k] && !cell.decls[k]) cell.decls[k] = d[k];
			}
		}
	}
	close();
	if (!lines.some((l) => l.length)) return null;
	// Grille : les cellules fusionnées (colspan, rowspan) occupent leur place.
	const rows: string[][] = [];
	const styles: (ClipStyle | null)[][] = [];
	const taken = new Set<string>();
	let width = 0;
	lines.forEach((l, r) => {
		rows[r] ??= [];
		styles[r] ??= [];
		let c = 0;
		for (const x of l) {
			while (taken.has(`${r},${c}`)) c++;
			// Blancs du HTML réduits à une espace ; <br> : saut de ligne dans la cellule.
			const text = decode(x.text.replace(/[ \t\r\n]+/g, ' ').replace(/ ?\u0001 ?/g, '\n')).trim();
			const st0 = styleOfDecls(x.decls, base);
			const st = x.href ? { ...(st0 ?? {}), ln: x.href } : st0;
			const merged = x.rows > 1 || x.cols > 1 ? { ...(st ?? {}), mr: x.rows, mc: x.cols } : st;
			for (let i = 0; i < x.rows; i++)
				for (let j = 0; j < x.cols; j++) {
					taken.add(`${r + i},${c + j}`);
					rows[r + i] ??= [];
					styles[r + i] ??= [];
					rows[r + i][c + j] = i === 0 && j === 0 ? text : '';
					// Les cellules couvertes par une fusion gardent la mise en forme (bordures, fond).
					styles[r + i][c + j] = i === 0 && j === 0 ? merged : st;
				}
			c += x.cols;
			width = Math.max(width, c);
		}
	});
	for (let r = 0; r < rows.length; r++) {
		rows[r] ??= [];
		styles[r] ??= [];
		for (let c = 0; c < width; c++) {
			rows[r][c] ??= '';
			styles[r][c] ??= null;
		}
	}
	return { rows, styles };
}

/** Texte des cellules d'un collage HTML, ou celui du texte tabulé quand il a la même forme (valeurs plus sûres). */
export function clipValues(html: HtmlClip, tsv: string[][]): string[][] {
	const same = tsv.length === html.rows.length && tsv.every((r, i) => r.length <= html.rows[i].length);
	if (!same) return html.rows;
	return html.rows.map((r, i) => r.map((v, j) => tsv[i][j] ?? v));
}

/** Une mise en forme collée porte-t-elle quelque chose ? */
export function hasClipStyles(c: HtmlClip | null): boolean {
	return !!c && c.styles.some((r) => r.some((s) => s && Object.keys(s).length));
}

// --- Écriture ------------------------------------------------------------------

const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

/** Code de format de tableur (mso-number-format) d'une mise en forme de Lattis. */
function msoFormat(st: ClipStyle): string | null {
	const dec = (n: number | undefined, d: number) => {
		const k = n ?? d;
		return k > 0 ? '0.' + '0'.repeat(k) : '0';
	};
	switch (st.nf) {
		case 'text':
			return '@';
		case '%':
			return dec(st.dp, 0) + '%';
		case '€':
			return '#,##0' + dec(st.dp, 2).slice(1) + ' "€"';
		case '0':
		case '0,00': {
			const d = dec(st.dp, st.nf === '0' ? 0 : 2);
			return st.ng ? d : '#,##' + d;
		}
		case 'date':
			return (st.df ?? 'jj/mm/aaaa').replace(/a/g, 'y').replace(/j/g, 'd');
		case 'time':
			return (st.df ?? 'hh:mm').replace(/a/g, 'y').replace(/j/g, 'd');
	}
	return st.format ?? null;
}

/** Déclarations CSS en ligne d'une mise en forme de Lattis. */
export function cssOfStyle(st: ClipStyle | null | undefined): string {
	if (!st) return '';
	const d: string[] = [];
	if (st.b) d.push('font-weight:bold');
	if (st.i) d.push('font-style:italic');
	const deco = [st.u && 'underline', st.x && 'line-through'].filter(Boolean).join(' ');
	if (deco) d.push('text-decoration:' + deco);
	if (st.fg) d.push('color:' + st.fg);
	if (st.bg) d.push('background:' + st.bg);
	if (st.fs) d.push(`font-size:${st.fs}pt`);
	if (st.ff) d.push(`font-family:"${st.ff.replace(/"/g, '')}"`);
	if (st.al) d.push('text-align:' + st.al);
	d.push('vertical-align:' + (st.va === 'top' ? 'top' : st.va === 'bottom' ? 'bottom' : 'middle'));
	if (st.wr) d.push('white-space:normal');
	const line = `.5pt solid ${st.bc ?? 'windowtext'}`;
	if (st.bt) d.push('border-top:' + line);
	if (st.bb) d.push('border-bottom:' + line);
	if (st.bl) d.push('border-left:' + line);
	if (st.br) d.push('border-right:' + line);
	const f = msoFormat(st);
	if (f) d.push(`mso-number-format:"${f.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`);
	return d.join(';');
}

/**
 * Tableau HTML d'une sélection copiée (valeurs affichées et mise en forme
 * de chaque cellule ; fusions en colspan et rowspan).
 */
export function toClipHtml(values: string[][], styleAt: (r: number, c: number) => ClipStyle | null | undefined): string {
	const covered = new Set<string>();
	const out: string[] = ['<html><head><meta charset="utf-8"><style>br{mso-data-placement:same-cell}</style></head><body><table style="border-collapse:collapse">'];
	values.forEach((row, r) => {
		out.push('<tr>');
		row.forEach((v, c) => {
			if (covered.has(`${r},${c}`)) return;
			const st = styleAt(r, c);
			const rs = Math.min(st?.mr ?? 1, values.length - r);
			const cs = Math.min(st?.mc ?? 1, row.length - c);
			for (let i = 0; i < rs; i++) for (let j = 0; j < cs; j++) if (i || j) covered.add(`${r + i},${c + j}`);
			const css = cssOfStyle(st);
			const span = (rs > 1 ? ` rowspan="${rs}"` : '') + (cs > 1 ? ` colspan="${cs}"` : '');
			const text = esc(v).replace(/\r?\n/g, '<br>');
			// Lien web ou e-mail : un <a href>, que les autres applications reprennent.
			const href = st?.ln && /^(https?:\/\/|mailto:)/i.test(st.ln) ? st.ln : '';
			out.push(`<td${span}${css ? ` style="${esc(css)}"` : ''}>${href ? `<a href="${esc(href)}">${text}</a>` : text}</td>`);
		});
		out.push('</tr>');
	});
	out.push('</table></body></html>');
	return out.join('');
}
