import { describe, expect, it } from 'vitest';
import type { MergePreview, SheetGroup } from '#bindings/internal/canvas/models.js';
import { cellText, chipLabel, futureTab, initialPlan, offerText, oldHint, pickOffer, planForSheets, previewWarnings, proposedName, refCounts, refLine, sheetRange, tabMarks, withTables } from './mergesheets';

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
