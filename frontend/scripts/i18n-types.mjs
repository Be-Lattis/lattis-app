// Génère src/lib/i18n/keys.d.ts depuis les catalogues français
// (src/lib/i18n/messages/fr/*.json) : une entrée par clé avec ses
// paramètres, pour que svelte-check refuse une clé inconnue ou un paramètre
// manquant dans t(). Lancé par npm run i18n:types et npm run check ; le test
// i18n.test.ts vérifie que le fichier est à jour.
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', 'src', 'lib', 'i18n');
const dir = join(root, 'messages', 'fr');

/** Clés à plat d'un catalogue imbriqué. */
function flatten(tree, prefix, out) {
	for (const [k, v] of Object.entries(tree)) {
		const key = `${prefix}.${k}`;
		if (typeof v === 'string') out[key] = v;
		else flatten(v, key, out);
	}
	return out;
}

/** Noms des paramètres d'un texte : {nom} et variables de pluriel (voir core.ts, shape). */
function params(s) {
	const out = new Set();
	// Groupes {…} de premier niveau de t : [début, fin].
	const groups = (t) => {
		const gs = [];
		for (let i = 0; i < t.length; i++) {
			if (t[i] !== '{') continue;
			let depth = 0;
			let j = i;
			for (; j < t.length; j++) {
				if (t[j] === '{') depth++;
				else if (t[j] === '}' && --depth === 0) break;
			}
			gs.push(t.slice(i + 1, j));
			i = j;
		}
		return gs;
	};
	const walk = (t) => {
		for (const inner of groups(t)) {
			const parts = inner.split(',');
			if (parts.length >= 3 && parts[1].trim() === 'plural') {
				out.add(parts[0].trim());
				for (const branch of groups(parts.slice(2).join(','))) walk(branch);
			} else out.add(inner.trim());
		}
	};
	walk(s);
	return [...out].sort();
}

const all = {};
for (const f of readdirSync(dir).filter((f) => f.endsWith('.json')).sort()) {
	flatten(JSON.parse(readFileSync(join(dir, f), 'utf8')), f.slice(0, -5), all);
}
const lines = [
	'// Généré par scripts/i18n-types.mjs (npm run i18n:types) depuis',
	'// messages/fr/*.json : ne pas modifier à la main.',
	'',
	'/** Paramètres de chaque message (clé : espace de noms puis chemin). */',
	'export interface MessageParams {'
];
for (const key of Object.keys(all).sort()) {
	const ps = params(all[key]);
	lines.push(`\t'${key}': ${ps.length ? `{ ${ps.map((p) => `${p}: number | string`).join('; ')} }` : 'Record<never, never>'};`);
}
lines.push('}', '', '/** Clé d’un message du catalogue. */', 'export type MessageKey = keyof MessageParams;', '');
writeFileSync(join(root, 'keys.d.ts'), lines.join('\r\n'));
