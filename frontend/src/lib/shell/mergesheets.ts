// Regrouper les onglets aux mêmes colonnes (docs/conception/
// regrouper-onglets-semblables.md, §4 et §5) : textes de la proposition
// (puce sous les onglets, bulle de la barre d'état), choix du groupe à
// proposer, couleurs des traits sous les onglets, plan de départ du volet
// Regrouper. Sans composant, testé par vitest.

import type { MergePlan, MergePreview, SheetGroup } from '#bindings/internal/canvas/models.js';

/** Couleurs des traits sous les onglets semblables : un groupe, une couleur (la première est l'accent). */
export const GROUP_COLORS = ['#217346', '#0f6cbd', '#b4009e', '#ca5010', '#5c2e91', '#038387'];

/** Moments où la proposition se fait d'elle-même (§4) ; « menu » : à la demande. */
export type OfferMoment = 'import' | 'open' | 'copy';

const nf = (n: number) => n.toLocaleString('fr-FR');

/** « 12 onglets semblables » (puce au bout de la série). */
export function chipLabel(g: SheetGroup): string {
	return `${nf(g.tables?.length ?? 0)} onglets semblables`;
}

/** « Janvier à Décembre » : le premier et le dernier onglet du groupe. */
export function sheetRange(g: SheetGroup): string {
	const t = g.tables ?? [];
	if (!t.length) return '';
	if (t.length === 1) return t[0].sheetName;
	if (t.length === 2) return `${t[0].sheetName} et ${t[1].sheetName}`;
	return `${t[0].sheetName} à ${t[t.length - 1].sheetName}`;
}

/** Nom proposé pour le tableau commun : celui du groupe, sinon le nom du fichier (sans extension), sinon « Regroupement ». */
export function proposedName(g: SheetGroup, fileTitle = ''): string {
	if (g.name) return g.name;
	const base = fileTitle
		.split(/[\\/]/)
		.pop()!
		.replace(/\.[^.]+$/, '')
		.replace(/[^\p{L}\p{N}_]/gu, '');
	return base && !/^\d/.test(base) ? base : 'Regroupement';
}

/** Libellé du futur onglet : « Ventes · Mois ▾ ». */
export function futureTab(g: SheetGroup, fileTitle = ''): string {
	const cat = g.category?.column;
	return cat ? `${proposedName(g, fileTitle)} · ${cat} ▾` : proposedName(g, fileTitle);
}

/** Texte de la bulle (§4.1, 2) : « 12 onglets (Janvier à Décembre) ont les mêmes colonnes. Les regrouper… ? » */
export function offerText(g: SheetGroup, fileTitle = ''): string {
	const n = g.tables?.length ?? 0;
	return `${nf(n)} onglets (${sheetRange(g)}) ont les mêmes colonnes. Les regrouper en un seul onglet « ${futureTab(g, fileTitle)} » ?`;
}

/**
 * Groupe à proposer dans la bulle : le plus grand (la détection les rend
 * dans cet ordre) assez sûr, jamais refusé, pas déjà proposé dans la
 * session (offered : identifiants). À la copie d'un onglet, seulement le
 * groupe de la nouvelle feuille, dès trois onglets (§4, 3).
 */
export function pickOffer(groups: SheetGroup[], offered: Set<string>, moment: OfferMoment, sheet?: number): SheetGroup | null {
	for (const g of groups) {
		if (!g.propose || g.dismissed || offered.has(g.id)) continue;
		if (moment === 'copy' && (sheet === undefined || (g.tables?.length ?? 0) < 3 || !g.tables?.some((t) => t.sheet === sheet))) continue;
		return g;
	}
	return null;
}

/** Groupes montrés sous les onglets : ceux qui sont proposés (pas refusés). */
export function shownGroups(groups: SheetGroup[]): SheetGroup[] {
	return groups.filter((g) => g.propose && !g.dismissed);
}

/** Trait sous chaque onglet d'un groupe montré : feuille → { couleur, rang du groupe }. */
export function tabMarks(groups: SheetGroup[]): Map<number, { color: string; group: number }> {
	const out = new Map<number, { color: string; group: number }>();
	groups.forEach((g, i) => {
		for (const t of g.tables ?? []) {
			if (!out.has(t.sheet)) out.set(t.sheet, { color: GROUP_COLORS[i % GROUP_COLORS.length], group: i });
		}
	});
	return out;
}

/** Plan de départ du volet : les tableaux du groupe, sa correspondance de colonnes, sa catégorie, le nom proposé. */
export function initialPlan(g: SheetGroup, fileTitle = ''): MergePlan {
	const c = g.category;
	return {
		tables: (g.tables ?? []).map((t) => t.table),
		columns: g.columns ?? [],
		category: c ? { name: c.column, type: c.type, values: [...(c.labels ?? [])], format: c.format ?? '' } : null,
		name: proposedName(g, fileTitle)
	};
}

/** Plan pour une sélection d'onglets (clic droit, menu Données) : le groupe qui les contient tous, sinon leurs seuls tableaux (le moteur déduit le reste). */
export function planForSheets(groups: SheetGroup[], sheets: number[], tablesOf: (sheet: number) => string[], fileTitle = ''): MergePlan | null {
	if (sheets.length === 1) {
		const g = groups.find((x) => x.tables?.some((t) => t.sheet === sheets[0]));
		return g ? initialPlan(g, fileTitle) : null;
	}
	const g = groups.find((x) => sheets.every((s) => x.tables?.some((t) => t.sheet === s)));
	if (g) {
		const keep = (g.tables ?? []).map((t, i) => (sheets.includes(t.sheet) ? i : -1)).filter((i) => i >= 0);
		const p = initialPlan(g, fileTitle);
		if (keep.length === (g.tables ?? []).length) return p;
		return withTables(p, keep);
	}
	const tables = sheets.flatMap(tablesOf);
	return tables.length >= 2 ? { tables, name: '' } : null;
}

/**
 * Le plan restreint à certains tableaux (rangs keep) : la correspondance
 * des colonnes et les valeurs de la catégorie suivent ; une colonne qui
 * n'est plus remplie par aucun tableau part.
 */
export function withTables(p: MergePlan, keep: number[]): MergePlan {
	const pick = <T>(xs: T[] | null | undefined) => keep.map((i) => (xs ?? [])[i]);
	const columns = (p.columns ?? [])
		.map((c) => ({ ...c, from: pick(c.from).map((x) => x ?? ''), paired: pick(c.paired).map((x) => !!x) }))
		.filter((c) => c.from.some((x) => x));
	return {
		...p,
		tables: pick(p.tables),
		columns,
		category: p.category ? { ...p.category, values: pick(p.category.values).map((x) => x ?? '') } : p.category
	};
}

/** Avertissements du volet (en orange) : clé retirée, formules à reprendre, refus. */
export function previewWarnings(p: MergePreview): string[] {
	const out: string[] = [];
	if (p.refused) out.push(`Impossible de regrouper : ${p.refused}`);
	if (p.keyNote) out.push(p.keyNote);
	const n = p.pending?.length ?? 0;
	if (n) out.push(`${n === 1 ? 'Une formule ou un objet cite' : `${nf(n)} formules ou objets citent`} les anciens tableaux : à reprendre à la main${p.old === 'hide' ? ' (les anciens onglets sont gardés masqués)' : ''}`);
	return out;
}

/** Ce que deviennent les anciens onglets. */
export function oldLabel(old: string): string {
	return old === 'hide' ? 'Garder une copie masquée (tableaux renommés …_ancien)' : 'Supprimer les onglets (Ctrl+Z les rend)';
}

/** Valeur d'une case de l'aperçu. */
export function cellText(v: unknown): string {
	if (v === null || v === undefined) return '';
	if (typeof v === 'number') return v.toLocaleString('fr-FR', { maximumFractionDigits: 10 });
	if (typeof v === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(v)) return `${v.slice(8, 10)}/${v.slice(5, 7)}/${v.slice(0, 4)}`;
	return String(v);
}
