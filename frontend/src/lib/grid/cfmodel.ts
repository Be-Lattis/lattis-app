// Gestionnaire des règles de mise en forme conditionnelle : les règles sont
// enregistrées par emplacement (colonne de tableau, ligne ou colonne
// entière, cellule libre, voir internal/canvas/rules.go) ; le gestionnaire
// les regroupe par règle (même identifiant : « S'applique à » d'Excel), dans
// l'ordre de priorité, puis réécrit les listes des emplacements changés.

import type { Rule } from './rules';
import { t } from '#lib/i18n/index.js';

/** Emplacement d'une règle. */
export type Loc = { k: 'tbl'; t: number; c: number } | { k: 'row'; i: number } | { k: 'col'; i: number } | { k: 'cell'; r: number; c: number };

export interface RuleTable {
	id: number;
	name: string;
	row: number; // ligne d'en-tête
	col: number;
	rows: number; // lignes de données
	columns: string[];
	rules: (Rule[] | undefined)[];
}

/** Règles d'une feuille, par emplacement. */
export interface SheetRules {
	tables: RuleTable[];
	rows: Map<number, Rule[]>;
	cols: Map<number, Rule[]>;
	/** Cellules libres : « ligne,colonne ». */
	cells: Map<string, Rule[]>;
}

/** Une règle et sa plage. */
export interface Managed {
	key: string;
	rule: Rule;
	locs: Loc[];
}

/** Changements à envoyer au classeur (canvas.RuleChanges). */
export interface RuleChanges {
	columns: { table: string; col: number; rules: Rule[] }[];
	lines: { rows: boolean; i0: number; i1: number; rules: Rule[] }[];
	cells: { r0: number; c0: number; r1: number; c1: number; rules: Rule[] }[];
}

export const locKey = (l: Loc) => (l.k === 'tbl' ? `t${l.t}:${l.c}` : l.k === 'cell' ? `x${l.r},${l.c}` : `${l.k}${l.i}`);

/** Règle sans ce que le classeur recalcule (formule analysée). */
export function bare(r: Rule): Rule {
	// eslint-disable-next-line @typescript-eslint/no-unused-vars
	const { f, ...rest } = r;
	return rest;
}

const scopeOf = (l: Loc) => (l.k === 'tbl' ? `t${l.t}` : l.k);

/** Clé d'une règle : son identifiant, sinon son contenu dans sa portée (règles d'avant les identifiants). */
export function groupKey(r: Rule, l: Loc): string {
	return r.id ? 'id:' + r.id : scopeOf(l) + ':' + JSON.stringify(bare(r));
}

/** Toutes les listes de règles d'une feuille, avec leur emplacement. */
export function* placed(sr: SheetRules): Generator<[Loc, Rule[]]> {
	for (const t of sr.tables) for (let c = 0; c < t.rules.length; c++) if (t.rules[c]?.length) yield [{ k: 'tbl', t: t.id, c }, t.rules[c]!];
	for (const [i, rs] of sr.rows) if (rs.length) yield [{ k: 'row', i }, rs];
	for (const [i, rs] of sr.cols) if (rs.length) yield [{ k: 'col', i }, rs];
	for (const [key, rs] of sr.cells) {
		if (!rs.length) continue;
		const [r, c] = key.split(',').map(Number);
		yield [{ k: 'cell', r, c }, rs];
	}
}

/** Règles de la feuille regroupées, de la plus prioritaire à la moins prioritaire. */
export function buildManaged(sr: SheetRules): Managed[] {
	const by = new Map<string, Managed & { rank: number; seen: number }>();
	let seen = 0;
	for (const [loc, rs] of placed(sr)) {
		rs.forEach((r, k) => {
			const key = groupKey(r, loc);
			let m = by.get(key);
			if (!m) {
				m = { key, rule: bare(r), locs: [], rank: k, seen: seen++ };
				by.set(key, m);
			}
			m.rank = Math.min(m.rank, k);
			m.locs.push(loc);
		});
	}
	return [...by.values()].sort((a, b) => a.rank - b.rank || a.seen - b.seen).map(({ key, rule, locs }) => ({ key, rule, locs }));
}

/** Liste des règles de chaque emplacement d'après la liste gérée. */
function listsOf(list: Managed[]): Map<string, { loc: Loc; rules: Rule[] }> {
	const out = new Map<string, { loc: Loc; rules: Rule[] }>();
	for (const m of list)
		for (const l of m.locs) {
			const k = locKey(l);
			let e = out.get(k);
			if (!e) out.set(k, (e = { loc: l, rules: [] }));
			if (!e.rules.includes(m.rule)) e.rules.push(m.rule);
		}
	return out;
}

const same = (a: Rule[], b: Rule[]) => JSON.stringify(a.map(bare)) === JSON.stringify(b.map(bare));

/** Rectangles qui couvrent exactement des cellules (« ligne,colonne »). */
export function rects(cells: Iterable<[number, number]>): { r0: number; c0: number; r1: number; c1: number }[] {
	const set = new Set<string>();
	for (const [r, c] of cells) set.add(`${r},${c}`);
	const keys = [...set].map((k) => k.split(',').map(Number) as [number, number]).sort((a, b) => a[0] - b[0] || a[1] - b[1]);
	const out: { r0: number; c0: number; r1: number; c1: number }[] = [];
	for (const [r, c] of keys) {
		if (!set.has(`${r},${c}`)) continue;
		let c1 = c;
		while (set.has(`${r},${c1 + 1}`)) c1++;
		let r1 = r;
		const full = (rr: number) => {
			for (let x = c; x <= c1; x++) if (!set.has(`${rr},${x}`)) return false;
			return true;
		};
		while (full(r1 + 1)) r1++;
		for (let rr = r; rr <= r1; rr++) for (let x = c; x <= c1; x++) set.delete(`${rr},${x}`);
		out.push({ r0: r, c0: c, r1, c1 });
	}
	return out;
}

/** Listes actuelles de la feuille, par emplacement. */
function current(sr: SheetRules): Map<string, { loc: Loc; rules: Rule[] }> {
	const old = new Map<string, { loc: Loc; rules: Rule[] }>();
	for (const [loc, rules] of placed(sr)) old.set(locKey(loc), { loc, rules });
	return old;
}

/** Changements qui font passer la feuille de sr à la liste gérée list. */
export function diffRules(sr: SheetRules, list: Managed[]): RuleChanges {
	const neu = listsOf(list);
	const old = current(sr);
	for (const [k, e] of old) if (!neu.has(k)) neu.set(k, { loc: e.loc, rules: [] });
	return emit(sr, [...neu.values()], old);
}

/** Nouvelle règle en tête des listes de ses emplacements (les autres ne changent pas). */
export function prependRule(sr: SheetRules, rule: Rule, locs: Loc[]): RuleChanges {
	const old = current(sr);
	return emit(
		sr,
		locs.map((loc) => ({
			loc,
			rules: [rule, ...(old.get(locKey(loc))?.rules ?? [])]
		})),
		old
	);
}

/** Retire toutes les règles des emplacements qui vérifient drop. */
export function clearWhere(sr: SheetRules, drop: (l: Loc) => boolean): RuleChanges {
	const old = current(sr);
	return emit(
		sr,
		[...old.values()].filter((e) => drop(e.loc)).map((e) => ({ loc: e.loc, rules: [] })),
		old
	);
}

/** Changements des emplacements dont la liste diffère de l'actuelle. */
function emit(sr: SheetRules, entries: { loc: Loc; rules: Rule[] }[], old: Map<string, { loc: Loc; rules: Rule[] }>): RuleChanges {
	const ch: RuleChanges = { columns: [], lines: [], cells: [] };
	const lines: { rows: boolean; i: number; rules: Rule[] }[] = [];
	const cells = new Map<string, { rules: Rule[]; at: [number, number][] }>();
	for (const { loc, rules } of entries) {
		if (same(old.get(locKey(loc))?.rules ?? [], rules)) continue;
		const clean = rules.map(bare);
		if (loc.k === 'tbl') {
			const t = sr.tables.find((x) => x.id === loc.t);
			if (t) ch.columns.push({ table: t.name, col: loc.c, rules: clean });
		} else if (loc.k === 'cell') {
			const j = JSON.stringify(clean);
			let g = cells.get(j);
			if (!g) cells.set(j, (g = { rules: clean, at: [] }));
			g.at.push([loc.r, loc.c]);
		} else lines.push({ rows: loc.k === 'row', i: loc.i, rules: clean });
	}
	// Lignes (ou colonnes) consécutives aux mêmes règles : une seule plage.
	lines.sort((a, b) => Number(b.rows) - Number(a.rows) || a.i - b.i);
	for (const l of lines) {
		const last = ch.lines[ch.lines.length - 1];
		if (last && last.rows === l.rows && last.i1 === l.i - 1 && JSON.stringify(last.rules) === JSON.stringify(l.rules)) last.i1 = l.i;
		else ch.lines.push({ rows: l.rows, i0: l.i, i1: l.i, rules: l.rules });
	}
	for (const g of cells.values()) for (const r of rects(g.at)) ch.cells.push({ ...r, rules: g.rules });
	return ch;
}

export const emptyChanges = (ch: RuleChanges) => !ch.columns.length && !ch.lines.length && !ch.cells.length;

// --- Adresses ---------------------------------------------------------------

export function colLetters(c: number): string {
	let s = '';
	for (let n = c + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s;
	return s;
}

function colIndex(letters: string): number {
	let c = 0;
	for (const ch of letters.toUpperCase()) c = c * 26 + ch.charCodeAt(0) - 64;
	return c - 1;
}

function runs(xs: number[]): [number, number][] {
	const s = [...new Set(xs)].sort((a, b) => a - b);
	const out: [number, number][] = [];
	for (const x of s) {
		const last = out[out.length - 1];
		if (last && last[1] === x - 1) last[1] = x;
		else out.push([x, x]);
	}
	return out;
}

/** « S'applique à » d'une plage : =Ventes[Montant];$C:$D;$2:$4;$F$1:$F$10. */
export function formatLocs(locs: Loc[], tables: RuleTable[]): string {
	const parts: string[] = [];
	for (const l of locs)
		if (l.k === 'tbl') {
			const t = tables.find((x) => x.id === l.t);
			const p = t ? `${t.name}[${t.columns[l.c] ?? l.c}]` : '?';
			if (!parts.includes(p)) parts.push(p);
		}
	for (const [a, b] of runs(locs.flatMap((l) => (l.k === 'col' ? [l.i] : [])))) parts.push(`$${colLetters(a)}:$${colLetters(b)}`);
	for (const [a, b] of runs(locs.flatMap((l) => (l.k === 'row' ? [l.i] : [])))) parts.push(`$${a + 1}:$${b + 1}`);
	for (const r of rects(locs.flatMap((l) => (l.k === 'cell' ? [[l.r, l.c] as [number, number]] : [])))) {
		const a = `$${colLetters(r.c0)}$${r.r0 + 1}`;
		parts.push(r.r0 === r.r1 && r.c0 === r.c1 ? a : `${a}:$${colLetters(r.c1)}$${r.r1 + 1}`);
	}
	return parts.length ? '=' + parts.join(';') : '';
}

/** Plafond des cellules libres d'une plage (comme le classeur). */
export const MAX_RULE_CELLS = 20000;

/**
 * Emplacements d'un rectangle de la feuille : dans un tableau, ses colonnes
 * (toutes leurs lignes) ; ailleurs, les cellules libres. Erreur au-delà du
 * plafond de cellules.
 */
export function rectLocs(r0: number, c0: number, r1: number, c1: number, tables: RuleTable[]): Loc[] | string {
	const out: Loc[] = [];
	const seen = new Set<string>();
	let free = 0;
	for (let c = c0; c <= c1; c++) {
		const covering = tables.filter((t) => c >= t.col && c < t.col + t.columns.length && t.row <= r1 && t.row + t.rows >= r0);
		for (const t of covering) {
			const key = `t${t.id}:${c - t.col}`;
			if (!seen.has(key)) {
				seen.add(key);
				out.push({ k: 'tbl', t: t.id, c: c - t.col });
			}
		}
		for (let r = r0; r <= r1; r++) {
			if (covering.some((t) => r >= t.row && r <= t.row + t.rows)) continue;
			if (++free > MAX_RULE_CELLS) return t('grid.cf.range.tooBig', { max: MAX_RULE_CELLS });
			out.push({ k: 'cell', r, c });
		}
	}
	return out;
}

/** Lit un « S'applique à » (références séparées par ; ou ,). */
export function parseLocs(text: string, tables: RuleTable[]): Loc[] | string {
	const out: Loc[] = [];
	const items = text
		.trim()
		.replace(/^=/, '')
		.split(/[;,]/)
		.map((s) => s.trim())
		.filter(Boolean);
	if (!items.length) return t('grid.cf.range.missing');
	for (const it of items) {
		let m = /^(.+?)\[(.+)\]$/.exec(it);
		if (m) {
			const tb = tables.find((x) => x.name.toLowerCase() === m![1].trim().toLowerCase());
			const c = tb?.columns.findIndex((x) => x.toLowerCase() === m![2].trim().toLowerCase()) ?? -1;
			if (!tb || c < 0) return t('grid.cf.range.noColumn', { ref: it });
			out.push({ k: 'tbl', t: tb.id, c });
			continue;
		}
		const tb = tables.find((x) => x.name.toLowerCase() === it.toLowerCase());
		if (tb) {
			tb.columns.forEach((_, c) => out.push({ k: 'tbl', t: tb.id, c }));
			continue;
		}
		if ((m = /^\$?([A-Z]{1,3}):\$?([A-Z]{1,3})$/i.exec(it))) {
			const [a, b] = [colIndex(m[1]), colIndex(m[2])].sort((x, y) => x - y);
			for (let i = a; i <= b; i++) out.push({ k: 'col', i });
			continue;
		}
		if ((m = /^\$?(\d+):\$?(\d+)$/.exec(it))) {
			const [a, b] = [+m[1] - 1, +m[2] - 1].sort((x, y) => x - y);
			if (a < 0) return t('grid.cf.range.invalid', { ref: it });
			for (let i = a; i <= b; i++) out.push({ k: 'row', i });
			continue;
		}
		if ((m = /^\$?([A-Z]{1,3})\$?(\d+)(?::\$?([A-Z]{1,3})\$?(\d+))?$/i.exec(it))) {
			const ra = +m[2] - 1;
			const ca = colIndex(m[1]);
			const rb = m[3] ? +m[4] - 1 : ra;
			const cb = m[3] ? colIndex(m[3]) : ca;
			if (ra < 0 || rb < 0) return t('grid.cf.range.invalid', { ref: it });
			const got = rectLocs(Math.min(ra, rb), Math.min(ca, cb), Math.max(ra, rb), Math.max(ca, cb), tables);
			if (typeof got === 'string') return got;
			out.push(...got);
			continue;
		}
		return t('grid.cf.range.unknown', { ref: it });
	}
	const seen = new Set<string>();
	return out.filter((l) => !seen.has(locKey(l)) && !!seen.add(locKey(l)));
}

/** L'emplacement touche-t-il le rectangle ? */
export function locIn(l: Loc, r0: number, c0: number, r1: number, c1: number, tables: RuleTable[]): boolean {
	switch (l.k) {
		case 'cell':
			return l.r >= r0 && l.r <= r1 && l.c >= c0 && l.c <= c1;
		case 'row':
			return l.i >= r0 && l.i <= r1;
		case 'col':
			return l.i >= c0 && l.i <= c1;
		case 'tbl': {
			const t = tables.find((x) => x.id === l.t);
			if (!t) return false;
			const c = t.col + l.c;
			return c >= c0 && c <= c1 && t.row <= r1 && t.row + t.rows >= r0;
		}
	}
}
