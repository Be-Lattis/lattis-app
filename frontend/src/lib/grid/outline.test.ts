import { describe, expect, it } from 'vitest';
import { LEVEL_W, bandSize, detailTarget, levelCenter, outlineDepth, outlineGroups, outlineHit, summaryLine, type OutlineLine } from './outline';
import { SUBTOTAL_FUNCS, subtotalDefaults } from './subtotal';
import { shortcutOf } from './shortcuts';

// Lignes 3 à 6 au niveau 1, 4 et 5 au niveau 2 (réduit), 9 seule au niveau 1 (réduite).
const lines: OutlineLine[] = [
	{ index: 6, level: 1 },
	{ index: 3, level: 1 },
	{ index: 4, level: 2 },
	{ index: 5, level: 2, folded: 2 },
	{ index: 9, level: 1, folded: 1 }
];

describe('plan', () => {
	it('groupes par niveau, état sur la ligne du côté de la synthèse', () => {
		expect(outlineGroups(lines)).toEqual([
			{ level: 1, from: 3, to: 6, folded: false },
			{ level: 1, from: 9, to: 9, folded: true },
			{ level: 2, from: 4, to: 5, folded: true }
		]);
		// Synthèse au-dessus : l'état est noté sur la première ligne.
		expect(outlineGroups([{ index: 2, level: 1, folded: 1 }, { index: 3, level: 1 }], true)).toEqual([{ level: 1, from: 2, to: 3, folded: true }]);
		expect(outlineGroups(null)).toEqual([]);
	});

	it('barre : une colonne par niveau et une pour tout afficher', () => {
		expect(outlineDepth(lines)).toBe(2);
		expect(bandSize(0)).toBe(0);
		expect(bandSize(2)).toBe(3 * LEVEL_W + 4);
		expect(levelCenter(1)).toBe(2 + LEVEL_W / 2);
		expect(summaryLine({ level: 1, from: 3, to: 6, folded: false })).toBe(7);
		expect(summaryLine({ level: 1, from: 3, to: 6, folded: false }, true)).toBe(2);
	});

	it('Afficher / Masquer le détail : le groupe de la ligne de synthèse, sinon le plus intérieur', () => {
		// Ligne 6 (synthèse du groupe 4-5, réduit) : Afficher rouvre ce groupe.
		expect(detailTarget(lines, false, 6, false)).toEqual({ level: 2, line: 4 });
		// Masquer depuis la ligne 6 : le groupe 3-6 (le 4-5 est déjà réduit).
		expect(detailTarget(lines, false, 6, true)).toEqual({ level: 1, line: 6 });
		// Ligne 10, synthèse du groupe réduit 9 : Afficher le rouvre ; Masquer n'a rien à faire.
		expect(detailTarget(lines, false, 10, false)).toEqual({ level: 1, line: 9 });
		expect(detailTarget(lines, false, 10, true)).toBeNull();
		expect(detailTarget(lines, false, 20, false)).toBeNull();
	});

	it('clic : boutons de niveau dans l’en-tête, [−]/[+] sur la ligne de synthèse', () => {
		const gs = outlineGroups(lines);
		const at = (i: number): [number, number] => [100 + i * 20, 20];
		expect(outlineHit(gs, 2, false, 10, levelCenter(3), 22, at)).toEqual({ kind: 'level', level: 3 });
		expect(outlineHit(gs, 2, false, 100 + 7 * 20 + 5, levelCenter(1), 22, at)).toEqual({ kind: 'group', group: gs[0] });
		expect(outlineHit(gs, 2, false, 100 + 6 * 20 + 5, levelCenter(2), 22, at)).toEqual({ kind: 'group', group: gs[2] });
		expect(outlineHit(gs, 2, false, 100 + 5 * 20 + 5, levelCenter(2), 22, at)).toBeNull();
		expect(outlineHit(gs, 2, false, 300, levelCenter(4), 22, at)).toBeNull();
	});

	it('Alt+Maj+→ groupe, Alt+Maj+← dissocie', () => {
		const k = (key: string, shiftKey = true) => shortcutOf({ key, code: '', ctrlKey: false, metaKey: false, shiftKey, altKey: true });
		expect(k('ArrowRight')).toBe('group');
		expect(k('ArrowLeft')).toBe('ungroup');
		expect(k('ArrowLeft', false)).toBeNull();
	});
});

describe('sous-total', () => {
	const tables = [
		{ name: 'Ventes', columns: ['Région', 'Mois', 'Montant', 'Qté'], numeric: [false, false, true, true] },
		{ name: 'Stock', columns: ['Code', 'Nb'], numeric: [false, true] }
	];
	it('choix de départ : colonne active (texte), Somme, dernière colonne de nombres', () => {
		expect(subtotalDefaults(tables)).toEqual({ table: 'Ventes', by: 'Région', func: 9, columns: ['Qté'] });
		expect(subtotalDefaults(tables, 'Ventes', 1)).toEqual({ table: 'Ventes', by: 'Mois', func: 9, columns: ['Qté'] });
		// Colonne active de nombres : la première colonne de texte.
		expect(subtotalDefaults(tables, 'Stock', 1)).toEqual({ table: 'Stock', by: 'Code', func: 9, columns: ['Nb'] });
		expect(subtotalDefaults([]).table).toBe('');
	});
	it('onze fonctions, codes de SOUS.TOTAL', () => {
		expect(SUBTOTAL_FUNCS.map((f) => f.code).sort((a, b) => a - b)).toEqual([1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11]);
		expect(SUBTOTAL_FUNCS[0].code).toBe(9);
	});
});
