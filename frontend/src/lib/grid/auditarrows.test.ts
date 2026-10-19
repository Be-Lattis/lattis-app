import { describe, expect, it } from 'vitest';
import { arrowNotes, arrowSpan, spanBox, spanPoint, type ArrowGeometry } from './auditarrows';

// Grille régulière : colonnes de 100 px, lignes de 20 px.
const geo: ArrowGeometry = { x: (c) => c * 100, w: () => 100, y: (r) => r * 20, h: (r0, r1) => (r1 - r0 + 1) * 20 };
const own = { name: 'Suividesnids', row: 2 };

describe('arrowSpan', () => {
	it('une ligne précise vise sa cellule, pas l’en-tête (O8 = Suividesnids[NID suivi][1])', () => {
		const s = arrowSpan({ kind: 'row', table: 'Suividesnids', row: 3, col: 4 }, { row: 7 }, null);
		expect(s).toEqual({ row: 3, rows: 1, col: 4 });
		expect(spanPoint(s, geo)).toEqual({ x: 450, y: 70 });
	});

	it('une plage est encadrée et la flèche part du centre du cadre', () => {
		const s = arrowSpan({ kind: 'row', row: 4, rows: 3, col: 1 }, { row: 0 }, null);
		expect(spanBox(s, geo)).toEqual({ x: 100, y: 80, w: 100, h: 60 });
		expect(spanPoint(s, geo)).toEqual({ x: 150, y: 110 });
	});

	it('une colonne entière d’un autre tableau vise son en-tête', () => {
		expect(arrowSpan({ kind: 'column', table: 'Autre', row: 0, col: 1 }, { row: 5 }, own)).toEqual({ row: 0, rows: 1, col: 1 });
	});

	it('une colonne du même tableau, depuis une ligne de données, est lue sur cette ligne', () => {
		expect(arrowSpan({ kind: 'column', table: 'Suividesnids', row: 2, col: 1 }, { row: 5 }, own)).toEqual({ row: 5, rows: 1, col: 1 });
		// Depuis l'en-tête : la colonne.
		expect(arrowSpan({ kind: 'column', table: 'Suividesnids', row: 2, col: 1 }, { row: 2 }, own)).toEqual({ row: 2, rows: 1, col: 1 });
	});

	it('une ligne sans place (masquée, absente) vise la colonne, même dans le même tableau', () => {
		const d = { kind: 'column', table: 'Suividesnids', row: 2, col: 1, note: 'Suividesnids[X] : la ligne 2 est masquée (filtre ou onglet)' };
		expect(arrowSpan(d, { row: 5 }, own)).toEqual({ row: 2, rows: 1, col: 1 });
		expect(arrowNotes([d, d, { kind: 'cell', row: 0, col: 0 }])).toEqual([d.note]);
	});
});
