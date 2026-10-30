// Compte rendu d'un export .xlsx ou .ods (transfer.ExportReport) : une ligne
// pour la barre d'état, le détail par feuille pour la boîte « Détails ».
// Choix de Louis : à l'export, dire ce qui est exporté ou non.

import { t, tAny } from '#lib/i18n/index.js';

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
	favorites?: number;
	scenarios?: number;
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
	const add = (n: number, key: string) => {
		if (n > 0) out.push(tAny(`shell.exportReport.lost.${key}`, { n }));
	};
	const sheets = r.sheets ?? [];
	add(sum(r, (s) => s.charts), 'charts');
	add(sum(r, (s) => s.pivots), 'pivots');
	const f = sum(r, (s) => s.formulas + s.columns);
	if (f > 0) {
		const fns = fnList(sheets.flatMap((s) => s.functions ?? []));
		const text = t('shell.exportReport.lost.formulas', { n: f });
		out.push(fns ? t('shell.exportReport.withFunctions', { text, functions: fns }) : text);
	}
	add(sum(r, (s) => s.rules), 'rules');
	add(sum(r, (s) => s.notes), 'notes');
	add(sum(r, (s) => s.lists), 'lists');
	add(sum(r, (s) => s.buttons), 'buttons');
	add(sum(r, (s) => s.queries), 'queries');
	add(sheets.filter((s) => s.kind).length, 'pages');
	add(r.scripts, 'scripts');
	add(r.favorites ?? 0, 'favorites');
	add(r.scenarios ?? 0, 'scenarios');
	add(sheets.filter((s) => s.protected).length, 'protections');
	add(sum(r, (s) => s.twoLevel), 'twoLevel');
	add(r.clipped, 'clipped');
	return out;
}

/** Message de la barre d'état après l'export. */
export function exportNotice(path: string, r: ExportReport): string {
	const lost = lostItems(r);
	return lost.length ? t('shell.exportReport.noticeLost', { path, lost: lost.join(', ') }) : t('shell.exportReport.noticeAll', { path });
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
		const put = (list: string[], n: number, key: string) => {
			if (n > 0) list.push(tAny(`shell.exportReport.detail.${key}`, { n }));
		};
		if (s.kind) {
			lost.push(s.kind === 'doc' ? t('shell.exportReport.detail.docTab') : t('shell.exportReport.detail.htmlTab'));
			return { name: s.name, kept, lost };
		}
		put(kept, s.tables, 'tables');
		put(kept, s.translated, 'translated');
		put(lost, s.charts, 'charts');
		put(lost, s.pivots, 'pivots');
		const fns = s.functions?.length ? fnList(s.functions, 8) : '';
		const withFns = (text: string) => (fns ? t('shell.exportReport.detail.noEquivalent', { text, functions: fns }) : text);
		if (s.formulas > 0) {
			const text = t('shell.exportReport.detail.formulas', { n: s.formulas });
			lost.push(s.columns ? text : withFns(text));
		}
		if (s.columns > 0) lost.push(withFns(t('shell.exportReport.detail.columns', { n: s.columns })));
		put(lost, s.rules, 'rules');
		put(lost, s.notes, 'notes');
		put(lost, s.lists, 'lists');
		put(lost, s.buttons, 'buttons');
		put(lost, s.queries, 'queries');
		put(lost, s.twoLevel, 'twoLevel');
		if (s.protected) lost.push(t('shell.exportReport.detail.protected'));
		put(lost, s.clipped, 'clipped');
		return { name: s.name, kept, lost };
	});
}
