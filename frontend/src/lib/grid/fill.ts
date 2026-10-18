// Poignée de recopie : prolonge une série à partir des cellules sources, comme
// Excel. Nombres (pas constant), dates (pas en jours), texte terminé par un
// nombre (« Article 1 »), mois et jours de la semaine ; sinon recopie.

const MONTHS = ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre'];
const MONTHS_SHORT = ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.'];
const DAYS = ['lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi', 'dimanche'];
const DAYS_SHORT = ['lun.', 'mar.', 'mer.', 'jeu.', 'ven.', 'sam.', 'dim.'];
const LISTS = [MONTHS, MONTHS_SHORT, DAYS, DAYS_SHORT];

const fmt = new Intl.NumberFormat('fr-FR', { maximumFractionDigits: 10, useGrouping: false });

function num(s: string): number | null {
	const t = s.trim().replace(/[\s  ]/g, '').replace(',', '.');
	if (!/^[-+]?(\d+\.?\d*|\.\d+)$/.test(t)) return null;
	return Number(t);
}

const DATE = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/;
function date(s: string): number | null {
	const m = DATE.exec(s.trim());
	if (!m) return null;
	const d = Date.UTC(+m[3], +m[2] - 1, +m[1]);
	return Number.isNaN(d) ? null : d / 86400000;
}
function showDate(days: number): string {
	const d = new Date(days * 86400000);
	return `${String(d.getUTCDate()).padStart(2, '0')}/${String(d.getUTCMonth() + 1).padStart(2, '0')}/${d.getUTCFullYear()}`;
}

function sameCase(model: string, word: string): string {
	if (model === model.toUpperCase()) return word.toUpperCase();
	if (model[0] === model[0].toUpperCase()) return word[0].toUpperCase() + word.slice(1);
	return word;
}

/** Pas constant d'une suite de nombres (null si irrégulière). */
function step(xs: number[]): number | null {
	if (xs.length < 2) return null;
	const d = xs[1] - xs[0];
	for (let i = 2; i < xs.length; i++) if (Math.abs(xs[i] - xs[i - 1] - d) > 1e-9) return null;
	return d;
}

/** Les n valeurs qui suivent src (vers le bas ou la droite). */
export function fillSeries(src: string[], n: number): string[] {
	const out: string[] = [];
	if (n <= 0 || src.length === 0) return out;
	const filled = src.filter((s) => s !== '');
	if (filled.length === src.length && !src.some((s) => s.trim().startsWith('='))) {
		// Nombres : pas constant (une seule valeur : recopie, comme Excel).
		const xs = src.map(num);
		if (xs.every((x) => x !== null)) {
			const d = step(xs as number[]) ?? 0;
			const last = xs[xs.length - 1]!;
			for (let k = 1; k <= n; k++) out.push(fmt.format(last + d * k));
			return out;
		}
		// Dates : pas en jours (une seule date : jour suivant).
		const ds = src.map(date);
		if (ds.every((x) => x !== null)) {
			const d = step(ds as number[]) ?? 1;
			const last = ds[ds.length - 1]!;
			for (let k = 1; k <= n; k++) out.push(showDate(last + d * k));
			return out;
		}
		// Mois, jours.
		for (const list of LISTS) {
			const idx = src.map((s) => list.indexOf(s.trim().toLowerCase()));
			if (idx.every((i) => i >= 0)) {
				const d = idx.length > 1 ? (((idx[1] - idx[0]) % list.length) + list.length) % list.length || 1 : 1;
				const last = idx[idx.length - 1];
				for (let k = 1; k <= n; k++) out.push(sameCase(src[src.length - 1].trim(), list[(last + d * k) % list.length]));
				return out;
			}
		}
		// Texte terminé par un nombre : « Article 1 », « T3 ».
		const parts = src.map((s) => /^(.*?)(\d+)$/.exec(s));
		if (parts.every((p) => p !== null) && parts.every((p) => p![1] === parts[0]![1])) {
			const xs2 = parts.map((p) => Number(p![2]));
			const d = step(xs2) ?? 1;
			const width = parts[parts.length - 1]![2].length;
			const last = xs2[xs2.length - 1];
			for (let k = 1; k <= n; k++) {
				const v = last + d * k;
				out.push(parts[0]![1] + (v >= 0 ? String(v).padStart(width, '0') : String(v)));
			}
			return out;
		}
	}
	// Recopie, en boucle sur les sources.
	for (let k = 0; k < n; k++) out.push(src[k % src.length]);
	return out;
}

/**
 * Recopie d'une formule vers la droite ou la gauche (choix de Louis) : une
 * référence à une colonne de tableau (Ventes[Prix], Ventes[Prix][2]) passe à
 * la colonne voisine du même tableau, dc colonnes plus loin, comme les
 * références structurées d'Excel. Les adresses Cell[…] restent absolues ; une
 * colonne qui sortirait du tableau ne bouge pas.
 */
export function shiftTableColumns(f: string, dc: number, tables: readonly { name: string; columns: readonly string[] }[]): string {
	if (!dc || !f.trim().startsWith('=')) return f;
	const byName = new Map(tables.map((t) => [t.name.toLowerCase(), t]));
	let out = '';
	let i = 0;
	while (i < f.length) {
		const ch = f[i];
		if (ch === '"') {
			// Chaîne : recopiée telle quelle ("" : guillemet doublé).
			let j = i + 1;
			while (j < f.length && !(f[j] === '"' && f[j + 1] !== '"')) j += f[j] === '"' ? 2 : 1;
			out += f.slice(i, j + 1);
			i = j + 1;
			continue;
		}
		const m = /^([\p{L}_][\p{L}\p{N}_.]*)\[([^\[\]]+)\]/u.exec(f.slice(i));
		if (m && !/[\p{L}\p{N}_.]/u.test(f[i - 1] ?? '')) {
			const t = byName.get(m[1].toLowerCase());
			const k = t ? t.columns.findIndex((c) => c.toLowerCase() === m[2].trim().toLowerCase()) : -1;
			const to = k + dc;
			out += t && k >= 0 && to >= 0 && to < t.columns.length ? `${m[1]}[${t.columns[to]}]` : m[0];
			i += m[0].length;
			continue;
		}
		out += ch;
		i++;
	}
	return out;
}
