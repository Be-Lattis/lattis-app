import { describe, expect, it } from 'vitest';
import { checkFormula, closeParens, editorWidth, formulaRefs, highlightParts, rangeRef, refRect, serverErrorPos, type RefTable } from './formulaEdit';

describe('closeParens', () => {
	it('ferme les parenthèses oubliées', () => {
		expect(closeParens('=SOMME(1;2')).toBe('=SOMME(1;2)');
		expect(closeParens('=SI(A1>0;ABS(2')).toBe('=SI(A1>0;ABS(2))');
		expect(closeParens('=SOMME(1;2)')).toBe('=SOMME(1;2)');
		expect(closeParens('=")("&(1')).toBe('=")("&(1)');
		expect(closeParens('=T[a(b][1')).toBe('=T[a(b][1');
		expect(closeParens('texte (')).toBe('texte (');
	});
});

describe('checkFormula', () => {
	it('accepte les formules correctes', () => {
		for (const f of ['=1+2', '=SOMME(1;2)', '=SOMME(Cell[A1:B3])', '=Ventes[Prix][1:3]', '=[Prix]*2', '=SOMME(1;)', "='Ma feuille'!Cell[A1]", '=$C$14*2', '=A1:B3', '=-2^2%', '=Ventes[Prix][Cell[B1]+1]'])
			expect(checkFormula(f), f).toBeNull();
	});
	it('explique les fautes en français ordinaire, à leur place', () => {
		expect(checkFormula('=1+')).toEqual({ pos: 2, end: 3, msg: 'il manque une valeur après « + »' });
		expect(checkFormula('=SOMME(1;')?.msg).toBe('il manque une valeur après « ; »');
		expect(checkFormula('=SOMME(1;2')?.msg).toBe('il manque une parenthèse fermante « ) » à SOMME');
		expect(checkFormula('=1+2)')).toEqual({ pos: 4, end: 5, msg: 'parenthèse fermante « ) » en trop' });
		expect(checkFormula('=1 2')?.msg).toBe('il manque un opérateur avant « 2 »');
		expect(checkFormula('=*2')?.msg).toBe('il manque une valeur avant « * »');
		expect(checkFormula('="abc')?.msg).toBe('guillemet non fermé');
		expect(checkFormula('=Cell[1A]')?.msg).toBe('adresse de cellule invalide : Cell[1A]');
		expect(checkFormula('=1 ? 2')?.msg).toBe('caractère inattendu « ? »');
	});
	it('situe une erreur renvoyée par le serveur', () => {
		expect(serverErrorPos('=SOMME(1 2)', 'formule : position 9 : il manque…')).toBe(9);
		expect(serverErrorPos('  =1+', 'formule : position 3 : …')).toBe(5);
		expect(serverErrorPos('=1', 'autre')).toBeNull();
	});
});

describe('références', () => {
	it('repère et colore les références', () => {
		const refs = formulaRefs('=SOMME(Cell[A1:A3])+Ventes[Prix][2]*[Qté]+cell[a1:a3]+C14+SOMME(1)');
		expect(refs.map((r) => r.text)).toEqual(['Cell[A1:A3]', 'Ventes[Prix][2]', '[Qté]', 'cell[a1:a3]', 'C14']);
		expect(refs[0].color).toBe(refs[3].color);
		expect(refs[0].color).not.toBe(refs[1].color);
		expect(formulaRefs('texte')).toEqual([]);
		// En cours de frappe : chaîne non fermée.
		expect(formulaRefs('=Cell[A1]&"ab').map((r) => r.text)).toEqual(['Cell[A1]']);
	});
	it('découpe le texte pour l’affichage coloré', () => {
		const text = '=Cell[A1]+';
		const parts = highlightParts(text, formulaRefs(text), { pos: 9, end: 10 });
		expect(parts).toEqual([{ text: '=' }, { text: 'Cell[A1]', color: expect.any(String) }, { text: '+', err: true }]);
	});
	const ventes: RefTable = { name: 'Ventes', row: 4, col: 1, columns: ['Prix', 'Qté'], rows: 3 };
	const clients: RefTable = { name: 'Clients', row: 4, col: 5, columns: ['Id', 'Nom'], rows: 2, keyed: true, keyRow: (k) => (k === 'C2' ? 1 : undefined) };
	it('situe les références sur la grille', () => {
		expect(refRect('Cell[B2:A1]', [], 'F1')).toEqual({ r0: 0, c0: 0, r1: 1, c1: 1 });
		expect(refRect('F1!Cell[A1]', [], 'F1')).toEqual({ r0: 0, c0: 0, r1: 0, c1: 0 });
		expect(refRect('F2!Cell[A1]', [], 'F1')).toBeNull();
		expect(refRect('$C$14', [], 'F1')).toEqual({ r0: 13, c0: 2, r1: 13, c1: 2 });
		expect(refRect('Ventes[Qté]', [ventes], 'F1')).toEqual({ r0: 4, c0: 2, r1: 7, c1: 2 });
		expect(refRect('Ventes[Prix][2]', [ventes], 'F1')).toEqual({ r0: 6, c0: 1, r1: 6, c1: 1 });
		expect(refRect('Ventes[Prix][2:9]', [ventes], 'F1')).toEqual({ r0: 6, c0: 1, r1: 7, c1: 1 });
		expect(refRect('[Qté]', [ventes], 'F1', { table: ventes, row: 5 })).toEqual({ r0: 5, c0: 2, r1: 5, c1: 2 });
		expect(refRect('Clients[Nom]["C2"]', [clients], 'F1')).toEqual({ r0: 6, c0: 6, r1: 6, c1: 6 });
		expect(refRect('Clients[Nom][2]', [clients], 'F1')).toBeNull();
	});
	it('écrit la référence d’une plage glissée', () => {
		expect(rangeRef({ r0: 13, c0: 2, r1: 15, c1: 2 }, [ventes], undefined)).toBe('Cell[C14:C16]');
		expect(rangeRef({ r0: 0, c0: 0, r1: 1, c1: 1 }, [], undefined)).toBe('Cell[A1:B2]');
		expect(rangeRef({ r0: 5, c0: 1, r1: 7, c1: 1 }, [ventes], undefined)).toBe('Ventes[Prix]');
		expect(rangeRef({ r0: 4, c0: 1, r1: 7, c1: 1 }, [ventes], undefined)).toBe('Ventes[Prix]');
		expect(rangeRef({ r0: 6, c0: 1, r1: 5, c1: 1 }, [ventes], undefined)).toBe('Ventes[Prix][1:2]');
		expect(rangeRef({ r0: 5, c0: 1, r1: 6, c1: 1 }, [ventes], ventes)).toBe('[Prix][1:2]');
		expect(rangeRef({ r0: 5, c0: 1, r1: 6, c1: 2 }, [ventes], undefined)).toBeNull();
	});
});

describe('editorWidth', () => {
	const w = () => 50;
	it('s’élargit sur les cellules vides, puis s’arrête', () => {
		expect(editorWidth(40, 0, w, () => true, 500)).toBe(50);
		expect(editorWidth(120, 0, w, () => true, 500)).toBe(150);
		expect(editorWidth(120, 0, w, (c) => c < 2, 500)).toBe(100);
		expect(editorWidth(900, 0, w, () => true, 320)).toBe(320);
	});
});
