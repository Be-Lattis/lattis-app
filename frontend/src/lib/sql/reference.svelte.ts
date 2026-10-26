// Référence SQL de l'onglet SQL (panneau « Référence », SqlReference.svelte).
//
// Les entrées sont dans ref/entries.json : section, mots-clés (F1, recherche)
// et requête d'exemple, écrite pour le classeur d'exemple (Commandes :
// Client, Montant, Payé, Échéance ; Clients : Client, Ville ; TCD1). Le test
// Go internal/canvas/sqlref_test.go exécute chacune sur ce classeur. Titre
// et phrase de chaque entrée sont dans ref/<langue>.json, chargés à la
// demande (le français tout de suite).
//
// À l'affichage, l'exemple prend les noms du classeur ouvert quand il a un
// tableau qui convient (une colonne texte, une colonne de nombres…) ; sinon
// il garde ceux du classeur d'exemple.

import entries from './ref/entries.json';
import fr from './ref/fr.json';
import { lang, lower, t, type Lang } from '#lib/i18n/index.js';
import { quote, tokenize, type SqlColumn, type SqlObject } from './names';
import { isReadOnly } from './examples';

export type Section = 'syntax' | 'window' | 'write' | 'functions' | 'dates' | 'lattis';
export const SECTIONS: Section[] = ['syntax', 'window', 'write', 'functions', 'dates', 'lattis'];

export interface RefEntry {
	id: string;
	section: Section;
	/** Mots-clés et fonctions que l'entrée explique (F1 sur l'un d'eux l'ouvre). */
	keys: string[];
	/** Exemple, sur le classeur d'exemple. */
	sql: string;
	/** "" : lecture ; "write" : INSERT, UPDATE, DELETE ; "rename" : ALTER TABLE … RENAME TO. */
	kind?: 'write' | 'rename';
}

export const reference = entries as RefEntry[];

type Texts = Record<string, { t: string; d: string }>;
const files = import.meta.glob<Texts>(['./ref/*.json', '!./ref/entries.json', '!./ref/fr.json'], { import: 'default' });
let texts = $state.raw<Partial<Record<Lang, Texts>>>({ fr });
const loading = new Map<Lang, Promise<void>>();

function textsOf(l: Lang): Texts {
	const m = texts[l];
	if (m) return m;
	if (!loading.has(l)) {
		const imp = files[`./ref/${l}.json`];
		loading.set(
			l,
			imp
				? imp().then((x) => {
						texts = { ...texts, [l]: x };
					})
				: Promise.resolve()
		);
	}
	return fr; // le temps du chargement
}

/** Titre de l'entrée dans la langue de l'interface. */
export const title = (e: RefEntry) => textsOf(lang())[e.id]?.t ?? fr[e.id as keyof typeof fr]?.t ?? e.id;
/** Phrase de l'entrée dans la langue de l'interface. */
export const summary = (e: RefEntry) => textsOf(lang())[e.id]?.d ?? fr[e.id as keyof typeof fr]?.d ?? '';
export const sectionLabel = (s: Section) => t(`sql.ref.section.${s}`);

/** Entrées qui contiennent tous les mots de la recherche (titre, phrase, mots-clés, exemple). */
export function search(query: string): RefEntry[] {
	const words = lower(query).split(/\s+/).filter(Boolean);
	if (words.length === 0) return reference;
	return reference.filter((e) => {
		const hay = lower([title(e), summary(e), e.keys.join(' '), e.sql].join(' '));
		return words.every((w) => hay.includes(w));
	});
}

/** Entrée qui explique un mot de l'éditeur (F1) : mot-clé ou fonction. */
export function entryFor(word: string): RefEntry | undefined {
	const w = word.trim().toUpperCase();
	if (!w) return undefined;
	return reference.find((e) => e.keys.includes(w));
}

// --- Noms du classeur ouvert ----------------------------------------------------

/** Noms du classeur d'exemple, et le rôle de chacun. */
const GENERIC: Record<string, Role> = {
	commandes: 'T',
	clients: 'T2',
	tcd1: 'P',
	client: 'A',
	montant: 'N',
	payé: 'B',
	échéance: 'D',
	ville: 'X'
};
type Role = 'T' | 'T2' | 'P' | 'A' | 'N' | 'B' | 'D' | 'X';

const IDENT = /(^|\s)(n°|no|num|id|code|réf|ref)(?!\p{L})|_id$/iu;
const isText = (c: SqlColumn) => c.type === 'text';
const isNumber = (c: SqlColumn) => c.type === 'integer' || c.type === 'decimal';
const isBool = (c: SqlColumn) => c.type === 'boolean';
const isDate = (c: SqlColumn) => c.type === 'date';
const COLUMN: Partial<Record<Role, (c: SqlColumn) => boolean>> = { A: isText, N: isNumber, B: isBool, D: isDate };

/** Colonnes de o pour les rôles demandés (A, N, B, D), ou null s'il en manque. */
function columnsFor(o: SqlObject, roles: Set<Role>, fixed: Partial<Record<Role, string>> = {}): Partial<Record<Role, string>> | null {
	const out: Partial<Record<Role, string>> = { ...fixed };
	for (const r of ['A', 'N', 'B', 'D'] as Role[]) {
		if (!roles.has(r) || out[r]) continue;
		// Montant : une colonne décimale plutôt qu'un numéro entier.
		const c = (r === 'N' && o.columns.find((x) => x.type === 'decimal')) || o.columns.find(COLUMN[r]!);
		if (!c) return null;
		out[r] = c.name;
	}
	return out;
}

/** Noms du classeur pour chaque rôle utilisé, ou null : l'exemple garde ses noms. */
function bind(roles: Set<Role>, objects: SqlObject[]): Partial<Record<Role, string>> | null {
	// Les tableaux qui ont une colonne décimale (des montants) d'abord.
	const tables = objects.filter((o) => !isReadOnly(o));
	tables.sort((a, b) => Number(!a.columns.some((c) => c.type === 'decimal')) - Number(!b.columns.some((c) => c.type === 'decimal')));
	const out: Partial<Record<Role, string>> = {};
	if (roles.has('P')) {
		const p = objects.find(isReadOnly);
		if (!p) return null;
		out.P = p.name;
	}
	if (!roles.has('T') && !roles.has('T2')) return roles.has('P') && roles.size === 1 ? out : null;
	for (const t of tables) {
		if (roles.has('T2')) {
			// Deux tableaux reliés par une colonne de même nom (la colonne Client).
			for (const t2 of tables) {
				if (t2 === t) continue;
				// La clé de l'un des deux d'abord, puis un identifiant (N° client,
				// Code…), sinon la première colonne commune.
				const common = t.columns.filter((c) => t2.columns.some((d) => lower(d.name) === lower(c.name)));
				const keys = [t.columns[t.key ?? -1]?.name, t2.columns[t2.key ?? -1]?.name].filter(Boolean).map((n) => lower(n!));
				const j = common.find((c) => keys.includes(lower(c.name))) ?? common.find((c) => IDENT.test(c.name)) ?? common[0];
				if (!j) continue;
				const x = t2.columns.find((c) => lower(c.name) !== lower(j.name));
				if (roles.has('X') && !x) continue;
				const cols = columnsFor(t, roles, { A: j.name });
				if (!cols) continue;
				return { ...out, ...cols, T: t.name, T2: t2.name, X: x?.name };
			}
			continue;
		}
		const cols = columnsFor(t, roles);
		if (cols) return { ...out, ...cols, T: t.name };
	}
	return null;
}

/**
 * Exemple de l'entrée avec les noms du classeur ouvert quand un tableau
 * convient ; sinon tel quel. Un nom écrit entre guillemets le reste.
 */
export function adapt(e: RefEntry, objects: SqlObject[]): string {
	const tokens = tokenize(e.sql);
	const roleOf = (tk: (typeof tokens)[number]) => ((tk.kind === 'word' || tk.kind === 'quoted') && tk.text[0] !== '[' ? GENERIC[lower(tk.value)] : undefined);
	const roles = new Set(tokens.map(roleOf).filter((r): r is Role => !!r));
	if (roles.size === 0) return e.sql;
	const names = bind(roles, objects);
	if (!names) return e.sql;
	return tokens
		.map((tk) => {
			const r = roleOf(tk);
			const name = r && names[r];
			if (!name) return tk.text;
			return tk.kind === 'quoted' ? `"${name.replaceAll('"', '""')}"` : quote(name);
		})
		.join('');
}
