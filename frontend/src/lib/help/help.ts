// Aide intégrée : pages (Markdown embarqué, lib/help/pages), catalogue des
// fonctions (internal/formula/docs.go, servi par DocumentService.FormulaHelp),
// référence Lua (script/reference.ts), raccourcis (shell/shortcuts.ts) et
// réglages (shell/settings.ts). Une cible d'aide est une chaîne : « sql »,
// « sql#ancre », « fonction:SOMME.SI », « lua:db.query ».

import { callAt, type FunctionDoc } from '#lib/grid/autocomplete.js';
import { categories as luaCategories, reference as luaReference } from '#lib/script/reference.js';
import { settingSections } from '#lib/shell/settings.js';
import { shortcutGroups } from '#lib/shell/shortcuts.js';
import { blockText, fold, parse, plain, slug, type Block } from './markdown';
import priseEnMain from './pages/prise-en-main.md?raw';
import formules from './pages/formules.md?raw';
import tcd from './pages/tcd.md?raw';
import graphiques from './pages/graphiques.md?raw';
import onglets from './pages/onglets.md?raw';
import controles from './pages/controles.md?raw';
import sql from './pages/sql.md?raw';
import lua from './pages/lua.md?raw';
import connexions from './pages/connexions.md?raw';
import assistant from './pages/assistant.md?raw';
import raccourcis from './pages/raccourcis.md?raw';
import parametres from './pages/parametres.md?raw';

export interface HelpPage {
	id: string;
	title: string;
	blocks: Block[];
}

function page(id: string, md: string): HelpPage {
	const blocks = parse(md);
	const h = blocks.find((b) => b.t === 'h' && b.level === 1);
	return { id, title: h && h.t === 'h' ? plain(h.text) : id, blocks };
}

/** Pages, dans l'ordre de la table des matières. */
export const pages: HelpPage[] = [
	page('prise-en-main', priseEnMain),
	page('formules', formules),
	page('tcd', tcd),
	page('graphiques', graphiques),
	page('onglets', onglets),
	page('controles', controles),
	page('sql', sql),
	page('lua', lua),
	page('connexions', connexions),
	page('assistant', assistant),
	page('raccourcis', raccourcis),
	page('parametres', parametres)
];

export const HOME = 'prise-en-main';

export const pageById = (id: string) => pages.find((p) => p.id === id);

/** Cible décodée : page et ancre, ou fonction, ou entrée de la référence Lua. */
export type Target = { page: string; anchor?: string } | { fn: string } | { lua: string };

export function parseTarget(t: string): Target {
	if (t.startsWith('fonction:')) return { fn: t.slice(9) };
	if (t.startsWith('lua:')) return { lua: t.slice(4) };
	const [p, anchor] = t.split('#');
	return { page: pageById(p) ? p : HOME, anchor: anchor || undefined };
}

/** Page de la table des matières à laquelle une cible appartient. */
export function targetPage(t: string): string {
	const x = parseTarget(t);
	return 'fn' in x ? 'formules' : 'lua' in x ? 'lua' : x.page;
}

const same = (a: string, b: string) => fold(a) === fold(b);

/** Fonction du catalogue par son nom français ou anglais. */
export function findFunction(functions: readonly FunctionDoc[], name: string): FunctionDoc | undefined {
	return functions.find((f) => same(f.name, name) || same(f.other, name));
}

/** Catégories des fonctions, dans l'ordre de la première apparition, et leurs fonctions. */
export function functionCategories(functions: readonly FunctionDoc[]): { name: string; functions: FunctionDoc[] }[] {
	const out: { name: string; functions: FunctionDoc[] }[] = [];
	for (const f of functions) {
		const c = f.category || 'Autres';
		let g = out.find((x) => x.name === c);
		if (!g) out.push((g = { name: c, functions: [] }));
		g.functions.push(f);
	}
	return out;
}

/**
 * Fonction d'une formule sous le curseur, pour F1 : le nom sur lequel est le
 * curseur (=SOM|ME(…), ou =SOMME| en cours de frappe), sinon la fonction dont
 * le curseur est dans les parenthèses. null hors d'une formule ou d'une
 * fonction connue.
 */
export function functionAt(text: string, caret: number, functions: readonly FunctionDoc[]): FunctionDoc | null {
	if (!/^\s*=/.test(text)) return null;
	for (const m of text.matchAll(/[\p{L}_][\p{L}\p{N}_.]*/gu)) {
		const a = m.index ?? 0;
		const b = a + m[0].length;
		if (a > caret) break;
		if (caret > b || text[a - 1] === '[') continue; // [Colonne]
		const f = findFunction(functions, m[0]);
		if (f) return f;
	}
	const c = callAt(text, caret);
	return (c && findFunction(functions, c.name)) ?? null;
}

/** Page d'aide de ce qui est affiché, pour F1 hors d'une formule. */
export function viewTarget(view: string, page: '' | 'html' | 'doc' = ''): string {
	if (view === 'sql') return 'sql';
	if (view === 'script') return 'lua';
	return page ? 'onglets' : HOME;
}

// --- Recherche ----------------------------------------------------------

/** Entrée de l'index : une section de page, une fonction, une entrée Lua, un raccourci, un réglage. */
export interface HelpEntry {
	target: string;
	title: string;
	/** Où l'entrée se trouve : « Formules · Texte », « Prise en main ». */
	where: string;
	/** Autres noms (nom anglais, catégorie, signature). */
	keys: string;
	text: string;
}

export interface HelpHit extends HelpEntry {
	score: number;
	snippet: string;
}

/** Sections d'une page (titres ##), la première étant l'introduction. */
export function sections(p: HelpPage): { anchor: string; title: string; text: string }[] {
	const out = [{ anchor: '', title: p.title, text: '' }];
	for (const b of p.blocks) {
		if (b.t === 'h' && b.level === 1) continue;
		if (b.t === 'h' && b.level === 2) out.push({ anchor: b.id, title: plain(b.text), text: '' });
		else out[out.length - 1].text += ' ' + blockText(b);
	}
	return out.filter((s) => s.text.trim() || s.anchor === '');
}

/** Index de recherche de toute l'aide. */
export function buildIndex(functions: readonly FunctionDoc[]): HelpEntry[] {
	const out: HelpEntry[] = [];
	for (const p of pages)
		for (const s of sections(p))
			out.push({ target: s.anchor ? `${p.id}#${s.anchor}` : p.id, title: s.title, where: s.anchor ? p.title : 'Page', keys: s.anchor ? '' : p.title, text: s.text.trim() });
	for (const f of functions)
		out.push({
			target: `fonction:${f.name}`,
			title: f.name,
			where: `Fonction · ${f.category ?? ''}`,
			keys: `${f.other} ${f.category ?? ''}`,
			text: [f.description, f.example, ...(f.args ?? []).map((a) => `${a.name} ${a.desc}`)].filter(Boolean).join(' ')
		});
	for (const e of luaReference)
		out.push({
			target: `lua:${e.name}`,
			title: e.name,
			where: `Lua · ${luaCategories.find((c) => c.id === e.category)?.label ?? e.category}`,
			keys: e.signature,
			text: [e.summary, e.details].filter(Boolean).join(' ')
		});
	for (const g of shortcutGroups)
		for (const s of g.items) out.push({ target: `raccourcis#${slug(g.title)}`, title: s.keys, where: `Raccourci · ${g.title}`, keys: s.keys, text: s.label });
	for (const sec of settingSections)
		for (const s of sec.settings) out.push({ target: 'parametres#reglages-disponibles', title: s.label, where: `Paramètres · ${sec.title}`, keys: s.path, text: s.help ?? '' });
	return out;
}

/** Extrait du texte autour du premier mot trouvé. */
function snippet(text: string, words: string[], len = 140): string {
	const f = fold(text);
	let at = -1;
	for (const w of words) {
		const i = f.indexOf(w);
		if (i >= 0 && (at < 0 || i < at)) at = i;
	}
	if (at < 0 || text.length <= len) return text.length > len ? text.slice(0, len).trimEnd() + '…' : text;
	// Début du texte si le mot y figure ; sinon un peu avant le mot, au début d'un mot.
	let start = at < len - 40 ? 0 : at - 40;
	if (start > 0) start = text.indexOf(' ', start) + 1 || start;
	const s = text.slice(start, start + len).trim();
	return (start > 0 ? '…' : '') + s + (start + len < text.length ? '…' : '');
}

/**
 * Recherche dans l'index : chaque mot doit se trouver dans le titre, les
 * autres noms ou le texte (sans tenir compte des accents ni de la casse) ;
 * le titre compte plus que le reste, un titre identique à la recherche
 * passe en tête.
 */
export function searchHelp(index: readonly HelpEntry[], query: string, max = 50): HelpHit[] {
	const q = fold(query.trim());
	const words = q.split(/\s+/).filter(Boolean);
	if (!words.length) return [];
	const hits: HelpHit[] = [];
	for (const e of index) {
		const title = fold(e.title);
		const keys = fold(e.keys);
		const text = fold(e.text);
		let score = 0;
		let ok = true;
		for (const w of words) {
			const s = title.startsWith(w) ? 30 : title.includes(w) ? 20 : keys.includes(w) ? 10 : text.includes(w) ? 2 : 0;
			if (!s) {
				ok = false;
				break;
			}
			score += s;
		}
		if (!ok) continue;
		if (title === q || fold(e.keys.split(' ')[0]) === q) score += 100;
		hits.push({ ...e, score, snippet: snippet(e.text, words) });
	}
	hits.sort((a, b) => b.score - a.score || a.title.length - b.title.length || a.title.localeCompare(b.title, 'fr'));
	return hits.slice(0, max);
}
