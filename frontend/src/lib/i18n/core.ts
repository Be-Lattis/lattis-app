// Logique pure de la traduction (sans état ni Svelte) : langues, mise à plat
// des catalogues, remplacement des paramètres et pluriels ICU minimaux. La
// même syntaxe que le catalogue Go (internal/msg) :
//   « {nom} » existe déjà
//   {n, plural, =0 {aucune ligne} one {# ligne} other {# lignes}}
// (# : le nombre, écrit dans la langue). Pas d'échappement par apostrophe :
// l'apostrophe française s'écrit telle quelle.

/** Langues de l'interface ; le français est la référence. */
export const LANGS = ['fr', 'en', 'de', 'es', 'zh'] as const;
export type Lang = (typeof LANGS)[number];

/** Réglage ui.language : une langue, ou auto (système / navigateur). */
export type LanguageSetting = Lang | 'auto';

/** Nom de chaque langue dans sa propre langue (choix des paramètres). */
export const LANG_NAMES: Record<Lang, string> = { fr: 'Français', en: 'English', de: 'Deutsch', es: 'Español', zh: '中文' };

/** Locale Intl de chaque langue (nombres, dates, tri). */
export const LOCALES: Record<Lang, string> = { fr: 'fr-FR', en: 'en-US', de: 'de-DE', es: 'es-ES', zh: 'zh-CN' };

/** Langue d'un système dont la langue n'est pas proposée (comme internal/msg.Fallback). */
export const FALLBACK: Lang = 'en';

/** Langue lue dans un code (« en-US », « zh-Hans-CN », « de_DE.UTF-8 ») ; null si elle n'est pas proposée. */
export function parseLang(tag: string | null | undefined): Lang | null {
	const t = (tag ?? '').trim().toLowerCase().split(/[-_.@]/)[0];
	return (LANGS as readonly string[]).includes(t) ? (t as Lang) : null;
}

/**
 * Langue de la session : le réglage s'il nomme une langue, sinon (auto) la
 * première langue proposée parmi celles du système ou du navigateur, sinon
 * FALLBACK (internal/msg.Resolve fait de même).
 */
export function resolveLang(setting: unknown, system: readonly (string | null | undefined)[]): Lang {
	const s = typeof setting === 'string' ? parseLang(setting) : null;
	if (s) return s;
	for (const tag of system) {
		const l = parseLang(tag);
		if (l) return l;
	}
	return FALLBACK;
}

/** Catalogue d'un espace de noms : objets imbriqués, textes aux feuilles. */
export interface Tree {
	[k: string]: string | Tree;
}

/** Met à plat un catalogue : { a: { b: 'x' } } → { 'ns.a.b': 'x' }. */
export function flatten(tree: Tree, prefix: string, out: Record<string, string> = {}): Record<string, string> {
	for (const [k, v] of Object.entries(tree)) {
		const key = prefix ? `${prefix}.${k}` : k;
		if (typeof v === 'string') out[key] = v;
		else flatten(v, key, out);
	}
	return out;
}

/** Espace de noms d'un fichier de messages (« ./messages/fr/grid.json » → grid). */
export function namespaceOf(path: string): { lang: string; ns: string } {
	const m = /messages\/([^/]+)\/([^/]+)\.json$/.exec(path);
	return { lang: m?.[1] ?? '', ns: m?.[2] ?? '' };
}

/** Paramètres d'un message : textes ou nombres. */
export type Params = Record<string, string | number>;

/** Position de l'accolade qui ferme celle de s[i] (-1 sans). */
function closing(s: string, i: number): number {
	let depth = 0;
	for (let k = i; k < s.length; k++) {
		if (s[k] === '{') depth++;
		else if (s[k] === '}' && --depth === 0) return k;
	}
	return -1;
}

/** « one {…} other {…} » en sélecteur → texte. */
function branches(s: string): Map<string, string> {
	const out = new Map<string, string>();
	let i = 0;
	while (i < s.length) {
		const open = s.indexOf('{', i);
		if (open < 0) break;
		const end = closing(s, open);
		if (end < 0) break;
		out.set(s.slice(i, open).trim(), s.slice(open + 1, end));
		i = end + 1;
	}
	return out;
}

/** Ce que render demande à la langue : catégorie de pluriel et écriture d'un nombre. */
export interface Speaker {
	plural(n: number): string;
	number(n: number): string;
}

/** Remplace les {paramètres} et choisit les pluriels de s. */
export function render(s: string, params: Params | undefined, sp: Speaker, hash?: string): string {
	let out = '';
	for (let i = 0; i < s.length; ) {
		const c = s[i];
		if (c === '#' && hash !== undefined) {
			out += hash;
			i++;
			continue;
		}
		if (c !== '{') {
			out += c;
			i++;
			continue;
		}
		const j = closing(s, i);
		if (j < 0) return out + s.slice(i);
		out += placeholder(s.slice(i + 1, j), params, sp, s.slice(i, j + 1));
		i = j + 1;
	}
	return out;
}

function placeholder(inner: string, params: Params | undefined, sp: Speaker, raw: string): string {
	const comma = inner.indexOf(',');
	const name = (comma < 0 ? inner : inner.slice(0, comma)).trim();
	const v = params?.[name];
	if (comma < 0) return v === undefined ? raw : typeof v === 'number' ? sp.number(v) : v;
	const rest = inner.slice(comma + 1);
	const c2 = rest.indexOf(',');
	if (c2 < 0 || rest.slice(0, c2).trim() !== 'plural') return raw;
	const n = typeof v === 'number' ? v : Number(v ?? 0) || 0;
	const cases = branches(rest.slice(c2 + 1));
	const text = cases.get(`=${n}`) ?? cases.get(sp.plural(n)) ?? cases.get('other') ?? '';
	return render(text, params, sp, sp.number(n));
}

/** Forme d'un message, pour comparer deux langues : paramètres et variables de pluriel. */
export function shape(s: string): { params: string[]; plurals: string[] } {
	const params = new Set<string>();
	const plurals = new Set<string>();
	const walk = (t: string) => {
		for (let i = 0; i < t.length; i++) {
			if (t[i] !== '{') continue;
			const j = closing(t, i);
			if (j < 0) return;
			const inner = t.slice(i + 1, j);
			const parts = inner.split(',');
			if (parts.length >= 3 && parts[1].trim() === 'plural') {
				plurals.add(parts[0].trim());
				for (const b of branches(parts.slice(2).join(',')).values()) walk(b);
			} else params.add(inner.trim());
			i = j;
		}
	};
	walk(s);
	return { params: [...params].sort(), plurals: [...plurals].sort() };
}
