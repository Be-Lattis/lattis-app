// Compte rendu d'un export .xlsx ou .ods (transfer.ExportReport) : une ligne
// pour la barre d'état, le détail par feuille pour la boîte « Détails ».
// Choix de Louis : à l'export, dire ce qui est exporté ou non.

import { count } from '#lib/sql/result.js';

export interface SheetReport {
	name: string;
	kind?: string;
	tables: number;
	translated: number;
	formulas: number;
	columns: number;
	functions?: string[] | null;
	pivots: number;
	charts: number;
	rules: number;
	notes: number;
	lists: number;
	buttons: number;
	queries: number;
	twoLevel: number;
	protected: boolean;
	clipped: number;
}

export interface ExportReport {
	sheets: SheetReport[] | null;
	scripts: number;
	clipped: number;
}

const sum = (r: ExportReport, f: (s: SheetReport) => number) => (r.sheets ?? []).reduce((n, s) => n + f(s), 0);

/** Fonctions en cause, sans doublon ; au-delà de max, « … ». */
function fnList(names: string[], max = 4): string {
	const u = [...new Set(names)].sort();
	return u.length > max ? `${u.slice(0, max).join(', ')}…` : u.join(', ');
}

/** Ce qui n'est pas exporté tel quel, en éléments courts (barre d'état). */
export function lostItems(r: ExportReport): string[] {
	const out: string[] = [];
	const add = (n: number, one: string, many: string) => {
		if (n > 0) out.push(count(n, one, many));
	};
	const sheets = r.sheets ?? [];
	add(sum(r, (s) => s.charts), 'graphique', 'graphiques');
	add(sum(r, (s) => s.pivots), 'TCD en valeurs', 'TCD en valeurs');
	const f = sum(r, (s) => s.formulas + s.columns);
	if (f > 0) {
		const fns = fnList(sheets.flatMap((s) => s.functions ?? []));
		out.push(count(f, 'formule en valeur', 'formules en valeurs') + (fns ? ` (${fns})` : ''));
	}
	add(sum(r, (s) => s.rules), 'règle de mise en forme conditionnelle', 'règles de mise en forme conditionnelle');
	add(sum(r, (s) => s.notes), 'note', 'notes');
	add(sum(r, (s) => s.lists), 'liste de validation', 'listes de validation');
	add(sum(r, (s) => s.buttons), 'bouton', 'boutons');
	add(sum(r, (s) => s.queries), 'requête (tableau en valeurs)', 'requêtes (tableaux en valeurs)');
	add(sheets.filter((s) => s.kind).length, 'onglet page ou document', 'onglets page ou document');
	add(r.scripts, 'script', 'scripts');
	add(sheets.filter((s) => s.protected).length, 'protection de feuille', 'protections de feuille');
	add(sum(r, (s) => s.twoLevel), "groupe d'en-têtes (dans les noms)", "groupes d'en-têtes (dans les noms)");
	add(r.clipped, 'cellule hors des limites', 'cellules hors des limites');
	return out;
}

/** Message de la barre d'état après l'export. */
export function exportNotice(path: string, r: ExportReport): string {
	const lost = lostItems(r);
	return `Classeur exporté : ${path} ; ` + (lost.length ? `non exporté : ${lost.join(', ')}` : 'tout a été exporté');
}

export interface SheetDetail {
	name: string;
	kept: string[];
	lost: string[];
}

/** Détail par feuille, en phrases (boîte « Détails »). */
export function exportDetails(r: ExportReport): SheetDetail[] {
	return (r.sheets ?? []).map((s) => {
		const kept: string[] = [];
		const lost: string[] = [];
		const put = (list: string[], n: number, one: string, many: string) => {
			if (n > 0) list.push(count(n, one, many));
		};
		if (s.kind) {
			lost.push(s.kind === 'doc' ? 'Onglet document : exporté comme une feuille vide' : 'Page HTML : exportée comme une feuille vide');
			return { name: s.name, kept, lost };
		}
		put(kept, s.tables, 'tableau', 'tableaux');
		put(kept, s.translated, 'formule exportée comme formule', 'formules exportées comme formules');
		put(lost, s.charts, 'graphique non exporté', 'graphiques non exportés');
		put(lost, s.pivots, 'tableau croisé exporté en valeurs, sans lien avec sa source', 'tableaux croisés exportés en valeurs, sans lien avec leur source');
		const fns = s.functions?.length ? ` (sans équivalent : ${fnList(s.functions, 8)})` : '';
		if (s.formulas > 0) lost.push(count(s.formulas, 'formule exportée par sa valeur', 'formules exportées par leur valeur') + (s.columns ? '' : fns));
		if (s.columns > 0) lost.push(count(s.columns, 'colonne calculée exportée par ses valeurs', 'colonnes calculées exportées par leurs valeurs') + fns);
		put(lost, s.rules, 'règle de mise en forme conditionnelle non exportée', 'règles de mise en forme conditionnelle non exportées');
		put(lost, s.notes, 'note non exportée', 'notes non exportées');
		put(lost, s.lists, 'liste de validation non exportée', 'listes de validation non exportées');
		put(lost, s.buttons, 'bouton non exporté', 'boutons non exportés');
		put(lost, s.queries, 'tableau chargé par une requête : valeurs exportées, sans la requête', 'tableaux chargés par une requête : valeurs exportées, sans la requête');
		put(lost, s.twoLevel, "tableau à groupes d'en-têtes : groupes exportés dans les noms (« Groupe.Sous »)", "tableaux à groupes d'en-têtes : groupes exportés dans les noms (« Groupe.Sous »)");
		if (s.protected) lost.push('Protection de la feuille non exportée');
		put(lost, s.clipped, 'cellule au-delà de 1 048 576 lignes × 16 384 colonnes non exportée', 'cellules au-delà de 1 048 576 lignes × 16 384 colonnes non exportées');
		return { name: s.name, kept, lost };
	});
}

/** Message après l'impression d'une feuille qui a des graphiques. */
export function printNotice(charts: number): string {
	return charts > 0 ? `${count(charts, 'graphique non imprimé', 'graphiques non imprimés')} : l'impression d'une feuille ne reprend que ses cellules` : '';
}
