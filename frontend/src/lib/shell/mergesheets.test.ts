import { describe, expect, it } from 'vitest';
import type { MergePreview, SheetGroup } from '#bindings/internal/canvas/models.js';
import { cellText, chipLabel, familyLine, familyPlan, followLabel, futureTab, groupSheets, initialPlan, layoutLines, leftTables, modeLabel, offerText, oldHint, pickOffer, planForSheets, previewWarnings, proposedName, refCounts, refLine, sheetRange, sourceLabel, tabMarks, withTables } from './mergesheets';

const months = ['Janvier', 'Février', 'Mars'];

function group(over: Partial<SheetGroup> = {}, names = months, first = 1): SheetGroup {
	return {
		id: 'g1',
		tables: names.map((n, i) => ({ table: n, sheet: first + i, sheetName: n, rows: 10, key: '', hidden: false })),
		columns: [
			{ name: 'Client', type: 'text', from: [...names.map(() => 'Client')], paired: names.map(() => false), text: false },
			{ name: 'Montant', type: 'decimal', from: names.map((_, i) => (i === 2 ? 'Montant €' : 'Montant')), paired: names.map((_, i) => i === 2), text: false }
		],
		category: { kind: 'month', score: 40, common: '', column: 'Mois', type: 'text', values: [...names], labels: [...names] },
		key: '',
		name: 'Ventes',
		score: 90,
		propose: true,
		identical: false,
		dismissed: false,
		summary: [],
		notes: [],
		...over
	} as SheetGroup;
}

describe('proposition de regroupement', () => {
	it('écrit la puce et la bulle', () => {
		const g = group();
		expect(chipLabel(g)).toBe('3 onglets semblables');
		expect(sheetRange(g)).toBe('Janvier à Mars');
		expect(futureTab(g)).toBe('Ventes · Mois ▾');
		expect(offerText(g)).toBe('3 onglets (Janvier à Mars) ont les mêmes colonnes. Les regrouper en un seul onglet « Ventes · Mois ▾ » ?');
		expect(sheetRange(group({}, ['2024', '2025']))).toBe('2024 et 2025');
	});

	it('prend le nom du fichier quand le groupe n’en propose pas', () => {
		expect(proposedName(group({ name: '' }), 'C:\\Docs\\Suivi ventes.xlsx')).toBe('Suiviventes');
		expect(proposedName(group({ name: '' }), '2025.xlsx')).toBe('Regroupement');
		expect(proposedName(group({ name: '' }))).toBe('Regroupement');
	});

	it('ne propose qu’une fois, jamais un groupe refusé ou peu sûr', () => {
		const a = group();
		const b = group({ id: 'g2', propose: false });
		const c = group({ id: 'g3', dismissed: true });
		expect(pickOffer([b, c, a], new Set(), 'import')).toBe(a);
		expect(pickOffer([a], new Set(['g1']), 'open')).toBeNull();
		expect(pickOffer([b, c], new Set(), 'open')).toBeNull();
	});

	it('à la copie : seulement le groupe de la nouvelle feuille, dès trois onglets', () => {
		const g = group();
		expect(pickOffer([g], new Set(), 'copy', 3)).toBe(g);
		expect(pickOffer([g], new Set(), 'copy', 9)).toBeNull();
		expect(pickOffer([group({}, ['2024', '2025'])], new Set(), 'copy', 2)).toBeNull();
	});

	it('trace un trait de couleur par groupe', () => {
		const m = tabMarks([group(), group({ id: 'g2' }, ['Paris', 'Lyon', 'Nice'], 10)]);
		expect(m.get(1)?.group).toBe(0);
		expect(m.get(11)?.group).toBe(1);
		expect(m.get(1)?.color).not.toBe(m.get(11)?.color);
		expect(m.has(5)).toBe(false);
	});
});

describe('plan du volet Regrouper', () => {
	it('part du groupe détecté', () => {
		const p = initialPlan(group());
		expect(p.tables).toEqual(months);
		expect(p.category).toEqual({ name: 'Mois', type: 'text', values: months, format: '' });
		expect(p.name).toBe('Ventes');
		expect(p.columns?.[1].from).toEqual(['Montant', 'Montant', 'Montant €']);
	});

	it('suit les onglets décochés', () => {
		const p = withTables(initialPlan(group()), [0, 2]);
		expect(p.tables).toEqual(['Janvier', 'Mars']);
		expect(p.category?.values).toEqual(['Janvier', 'Mars']);
		expect(p.columns?.[1].from).toEqual(['Montant', 'Montant €']);
		expect(p.columns?.[1].paired).toEqual([false, true]);
	});

	it('part d’une sélection d’onglets', () => {
		const g = group();
		const tablesOf = (s: number) => [`T${s}`];
		expect(planForSheets([g], [1, 3], tablesOf)?.tables).toEqual(['Janvier', 'Mars']);
		expect(planForSheets([g], [2], tablesOf)?.tables).toEqual(months);
		expect(planForSheets([g], [7, 8], tablesOf)?.tables).toEqual(['T7', 'T8']);
		expect(planForSheets([], [7], tablesOf)).toBeNull();
	});

	it('avertit en orange', () => {
		const p = { keyNote: 'Pas de clé : F-001 est dans Janvier et dans Février', pending: [{ kind: 'cell' }, { kind: 'cell' }], old: 'hide', refused: '' } as unknown as MergePreview;
		expect(previewWarnings(p)).toEqual([
			'Pas de clé : F-001 est dans Janvier et dans Février',
			'2 formules ou objets citent les anciens tableaux sans équivalent exact : à reprendre à la main (les anciens onglets sont gardés masqués)'
		]);
		expect(previewWarnings({ refused: 'feuille protégée', pending: [] } as unknown as MergePreview)).toEqual(['Impossible de regrouper : feuille protégée']);
		const kept = '« Janvier » gardée : elle contient aussi le tableau Remises (à supprimer à la main si besoin)';
		expect(previewWarnings({ refused: '', pending: [], kept: [kept] } as unknown as MergePreview)).toEqual([kept]);
	});

	it('fait le bilan des formules reprises et à reprendre (§8)', () => {
		const ref = (n: number) => Array.from({ length: n }, () => ({ kind: 'cell' }));
		const counts = (done: number, left: number) => refCounts({ rewritten: ref(done), pending: ref(left) } as unknown as MergePreview);
		expect(counts(14, 2)).toBe('14 formules reprises, 2 à reprendre à la main');
		expect(counts(1, 1)).toBe('Une formule reprise, une à reprendre à la main');
		expect(counts(3, 0)).toBe('3 formules reprises');
		expect(counts(0, 2)).toBe('2 formules à reprendre à la main');
		expect(counts(0, 0)).toBe('');
		expect(refCounts({ rewritten: null, pending: null } as unknown as MergePreview)).toBe('');
		expect(refCounts({ rewritten: ref(1200), pending: [] } as unknown as MergePreview)).toBe('1\u202f200 formules reprises');
	});

	it('écrit chaque formule reprise (avant → après) ou à reprendre (raison)', () => {
		expect(refLine({ kind: 'cell', where: 'Synthèse A1', formula: '=SOMME(Janvier[Montant])', after: '=SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")' } as never)).toBe(
			'Synthèse A1 : =SOMME(Janvier[Montant]) → =SOMME.SI.ENS(Ventes[Montant]; Ventes[Mois]; "=Janvier")'
		);
		expect(refLine({ kind: 'cell', where: 'Synthèse A2', formula: '=Janvier[Montant][3]', why: 'une ligne précise' } as never)).toBe('Synthèse A2 : =Janvier[Montant][3] (une ligne précise)');
	});

	it('explique ce que deviennent les anciens onglets (décision 12.1)', () => {
		const p = (n: number) => ({ pending: Array.from({ length: n }, () => ({ kind: 'cell' })), old: n ? 'hide' : 'delete' }) as unknown as MergePreview;
		expect(oldHint(p(0), 'delete')).toBe('');
		expect(oldHint(null, 'hide')).toBe('');
		expect(oldHint(p(2), 'hide')).toBe('Gardés masqués : 2 formules ou objets les citent encore');
		expect(oldHint(p(1), 'delete')).toBe('Supprimés quand même : une formule ou un objet les cite encore, qui ne trouvera plus son tableau');
	});

	it('affiche les cases de l’aperçu', () => {
		expect(cellText('2025-01-01')).toBe('01/01/2025');
		expect(cellText(null)).toBe('');
		expect(cellText(12.5)).toBe('12,5');
		expect(cellText('Janvier')).toBe('Janvier');
	});
});

// Lot 4 : feuilles à plusieurs tableaux (§6.5).
function families(): SheetGroup['families'] {
	const t = (name: string, i: number) => ({ table: `${name}${i + 1}`, sheet: i + 1, sheetName: months[i], rows: 2, key: '', hidden: false });
	return [
		{ tables: months.map((_, i) => t('Tarifs', i)), columns: [], key: '', name: 'Tarifs', same: true, mode: 'once' },
		{ tables: months.map((_, i) => t('Notes', i)), columns: [{ name: 'Note', type: 'text', from: ['Note', 'Note', 'Note'], paired: [false, false, false], text: false }], key: '', name: 'Notes', same: false, mode: 'merge' },
		{ tables: [t('Remises', 0)], columns: [], key: '', name: 'Remises', same: false, mode: 'leave' }
	] as SheetGroup['families'];
}

describe('feuilles à plusieurs tableaux (lot 4)', () => {
	it('compte les onglets, ou les tableaux empilés', () => {
		const g = group({ sheets: months.map((n, i) => ({ sheet: i + 1, name: n, hidden: false, copied: false })), families: families() });
		expect(groupSheets(g).map((s) => s.sheet)).toEqual([1, 2, 3]);
		expect(chipLabel(g)).toBe('3 onglets semblables');
		expect(offerText(g)).toBe('3 onglets (Janvier à Mars) ont les mêmes tableaux. Les regrouper en un seul onglet « Ventes · Mois ▾ » ?');
		const e = group({
			stacked: true,
			tables: ['T1', 'T2', 'T3'].map((t) => ({ table: t, sheet: 1, sheetName: 'Villes', rows: 2, key: '', hidden: false })),
			category: { kind: 'city', score: 20, common: '', column: 'Ville', type: 'text', values: ['Paris', 'Lyon', 'Nice'], labels: ['Paris', 'Lyon', 'Nice'] }
		} as Partial<SheetGroup>);
		expect(chipLabel(e)).toBe('3 tableaux semblables');
		expect(groupSheets(e)).toEqual([{ sheet: 1, name: 'Villes' }]);
		expect(offerText(e)).toBe('3 tableaux de « Villes » (Paris à Nice) ont les mêmes colonnes. Les regrouper en un seul tableau « Ventes · Ville ▾ » ?');
	});

	it('à la troisième copie, propose même un groupe peu sûr d’onglets copiés', () => {
		const sheets = ['Janvier', 'Janvier (2)', 'Janvier (3)'].map((n, i) => ({ sheet: i + 1, name: n, hidden: false, copied: true }));
		const g = group({ propose: false, copies: 3, sheets });
		expect(pickOffer([g], new Set(), 'copy', 3)).toBe(g);
		expect(pickOffer([g], new Set(), 'open')).toBeNull();
		expect(pickOffer([group({ propose: false, copies: 0 })], new Set(), 'copy', 3)).toBeNull();
	});

	it('part des familles proposées, de la source de la catégorie et du cas F', () => {
		const p = initialPlan(group({ families: families(), categoryFrom: 'cell:A1' }));
		expect(p.categoryFrom).toBe('cell:A1');
		expect(p.families?.map((f) => [f.tables?.[0], f.mode])).toEqual([
			['Tarifs1', 'once'],
			['Notes1', 'merge'],
			['Remises1', 'leave']
		]);
		expect(p.families?.[1].columns?.length).toBe(1);
		expect(initialPlan(group({ dataColumn: 'Date' })).noCategory).toBe(true);
		expect(initialPlan(group({ dataColumn: 'Date', families: families() })).noCategory).toBeUndefined();
		expect(initialPlan(group()).categoryFrom).toBeUndefined();
	});

	it('restreint les familles aux onglets cochés, avec le mode choisi', () => {
		const f = familyPlan(families() ?? [], ['merge', 'leave', 'merge'], new Set([1, 3]));
		expect(f.map((x) => [x.tables, x.mode])).toEqual([
			[['Tarifs1', 'Tarifs3'], 'merge'],
			[['Notes1', 'Notes3'], 'leave'],
			[['Remises1'], 'merge']
		]);
		// Restreinte, la correspondance des colonnes se déduit de nouveau.
		expect(f[1].columns).toBeUndefined();
		expect(familyPlan(families() ?? [], [], new Set([2])).map((x) => x.tables)).toEqual([['Tarifs2'], ['Notes2']]);
		const g = group({ families: families() });
		expect(planForSheets([g], [1, 2], () => [])?.families?.map((x) => x.tables)).toEqual([
			['Tarifs1', 'Tarifs2'],
			['Notes1', 'Notes2'],
			['Remises1']
		]);
	});

	it('écrit les familles, les modes et la source', () => {
		const [tarifs, , remises] = families() ?? [];
		expect(familyLine(tarifs)).toBe('Tarifs · 3 onglets · mêmes données partout');
		expect(familyLine(remises)).toBe('Remises · seulement dans Janvier');
		expect(modeLabel('merge', 'Mois')).toBe('Les regrouper : suit le mois');
		expect(modeLabel('merge', 'Lot', true)).toBe('Le regrouper : suit « Lot »');
		expect(modeLabel('once', 'Mois')).toBe('En garder un seul exemplaire (le dernier)');
		expect(modeLabel('leave', 'Mois', true)).toBe('Le laisser sur son onglet');
		expect(followLabel('Année')).toBe("l'année");
		expect(sourceLabel('cell:A1')).toBe('cellule A1');
		expect(sourceLabel('sheet')).toBe("nom de l'onglet");
		expect(sourceLabel('title', [{ source: 'title', label: 'titre au-dessus de chaque tableau' } as never])).toBe('titre au-dessus de chaque tableau');
	});

	it('montre la disposition de la feuille commune', () => {
		const p = {
			plan: { category: { name: 'Mois' } },
			layout: [
				{ kind: 'table', name: 'Ventes', row: 2, col: 0, rows: 6, cols: 3, follows: true, perValue: false },
				{ kind: 'cell', name: 'Mars 2025', row: 0, col: 1, rows: 0, cols: 1, follows: false, perValue: true },
				{ kind: 'table', name: 'Tarifs', row: 2, col: 27, rows: 1, cols: 2, follows: false, perValue: false }
			],
			cells: 5,
			perValue: 2,
			lost: 1,
			families: [{ name: '', mode: 'leave', tables: [{ table: 'Remises1', sheetName: 'Janvier' }] }]
		} as unknown as MergePreview;
		expect(layoutLines(p)).toEqual([
			'Ventes en A3 · 6 lignes · suit le choix',
			'Tarifs en AB3 · 1 ligne',
			'5 cellules libres reprises, dont 2 par mois',
			'Une cellule libre tombe sur un tableau commun : non reprise'
		]);
		expect(leftTables(p.families ?? [])).toEqual(['Remises1 (Janvier)']);
	});
});
