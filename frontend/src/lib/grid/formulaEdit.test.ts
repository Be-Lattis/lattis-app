import { describe, expect, it } from 'vitest';
import { cellRefText, checkFormula, closeParens, movedRef, pointedRef, previewShift, editorWidth, formulaRefs, highlightParts, rangeRef, refRect, serverErrorPos, type RefTable } from './formulaEdit';

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
		// Index de ligne calculé : chaque référence garde sa couleur, l'expression n'en fait pas partie.
		const nested = formulaRefs('=Commandes[Produit][1+Commandes[Quantité][1]]');
		expect(nested.map((r) => r.text)).toEqual(['Commandes[Produit]', 'Commandes[Quantité][1]']);
		expect(nested[0].color).not.toBe(nested[1].color);
		expect(formulaRefs('=[Prix][[Rang]-1]+1').map((r) => r.text)).toEqual(['[Prix]', '[Rang]']);
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

describe('previewShift : saisie de la barre montrée dans sa cellule', () => {
	it('texte qui tient dans la cellule : pas de défilement', () => {
		expect(previewShift(30, 60, 76)).toBe(0);
		expect(previewShift(0, 0, 76)).toBe(0);
	});

	it('curseur à la fin d\'un texte plus long : la fin est visible (demande de Louis)', () => {
		expect(previewShift(300, 300, 76)).toBe(224);
	});

	it('curseur au début ou dans la partie visible : pas de défilement', () => {
		expect(previewShift(0, 300, 76)).toBe(0);
		expect(previewShift(70, 300, 76)).toBe(0);
	});

	it('curseur au milieu : il touche le bord droit, sans dépasser la fin du texte', () => {
		expect(previewShift(150, 300, 76)).toBe(74);
		expect(previewShift(400, 300, 76)).toBe(224);
	});

	it('cellule sans largeur (colonne masquée) : rien', () => {
		expect(previewShift(150, 300, 0)).toBe(0);
		expect(previewShift(150, 300, -12)).toBe(0);
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

describe('références relatives et marqueurs déplacés', () => {
	const t: RefTable = { name: 'Ventes', row: 0, col: 0, columns: ['Qté', 'Prix'], rows: 5 };
	const here = { table: t, row: 3 };
	it('situe [Col][-1] et [Col][+1] par rapport à la ligne de la formule', () => {
		expect(refRect('[Prix][-1]', [t], '', here)).toEqual({ r0: 2, c0: 1, r1: 2, c1: 1 });
		expect(refRect('[Prix][+2]', [t], '', here)).toEqual({ r0: 5, c0: 1, r1: 5, c1: 1 });
		expect(refRect('[Prix][-3]', [t], '', here)).toBeNull(); // l'en-tête
	});
	it('écrit la référence d’une cellule comme un clic', () => {
		expect(cellRefText(3, 1, [t], here)).toBe('[Prix]');
		expect(cellRefText(2, 1, [t], here)).toBe('[Prix][-1]');
		expect(cellRefText(4, 0, [t], here)).toBe('[Qté][+1]');
		expect(cellRefText(0, 1, [t], here)).toBe('Ventes[Prix]');
		expect(cellRefText(7, 3, [t], here)).toBe('Cell[D8]');
		expect(cellRefText(2, 1, [t])).toBe('Ventes[Prix][2]');
	});
	it('réécrit un marqueur déplacé', () => {
		// [Prix] (ligne 3) monté d'une ligne : [Prix][-1].
		expect(movedRef({ r0: 3, c0: 1, r1: 3, c1: 1 }, -1, 0, [t], here)).toBe('[Prix][-1]');
		// Toute la colonne Qté décalée vers Prix.
		expect(movedRef({ r0: 0, c0: 0, r1: 5, c1: 0 }, 0, 1, [t], here)).toBe('Ventes[Prix]');
		// Attrapée plus bas et déposée sur une autre ligne : toujours toute la colonne.
		expect(movedRef({ r0: 0, c0: 0, r1: 5, c1: 0 }, 2, 1, [t], here, { row: 4, col: 1 })).toBe('Ventes[Prix]');
		// Hors du tableau : la même hauteur, sur les mêmes lignes.
		expect(movedRef({ r0: 0, c0: 0, r1: 5, c1: 0 }, 3, 4, [t], here, { row: 3, col: 4 })).toBe('Cell[E1:E6]');
		expect(movedRef({ r0: 0, c0: 0, r1: 0, c1: 0 }, -1, 0, [t], here)).toBeNull();
	});
});

describe('pointedRef : référence pointée au clic', () => {
	// Bilan (TEST clé 1/2/3, année) en A1:B4.
	const bilan: RefTable = { name: 'Bilan', row: 0, col: 0, columns: ['TEST', 'année'], rows: 3, keyed: false, keyRow: (k) => Number(k) - 1 };
	const tables = [bilan];
	it('depuis une cellule libre : la valeur d’une cellule, pas toute la colonne', () => {
		expect(pointedRef(2, 0, tables, { row: 1, col: 4 })).toBe('Bilan[TEST][2]');
		expect(pointedRef(1, 1, tables, { row: 6, col: 0 })).toBe('Bilan[année][1]');
		// Formule d'une autre feuille (pointage) : de même.
		expect(pointedRef(3, 0, tables, null)).toBe('Bilan[TEST][3]');
	});
	it('l’en-tête donne toute la colonne', () => {
		expect(pointedRef(0, 0, tables, { row: 1, col: 4 })).toBe('Bilan[TEST]');
	});
	it('dans le tableau, ou juste à côté sur une de ses lignes : relatif à la ligne', () => {
		expect(pointedRef(2, 0, tables, { row: 2, col: 1 })).toBe('[TEST]');
		expect(pointedRef(3, 0, tables, { row: 2, col: 2 })).toBe('[TEST][+1]');
		expect(pointedRef(1, 1, tables, { row: 2, col: 2 })).toBe('[année][-1]');
		// À côté de l'en-tête ou sous le tableau : cellule libre.
		expect(pointedRef(2, 0, tables, { row: 0, col: 2 })).toBe('Bilan[TEST][2]');
		expect(pointedRef(2, 0, tables, { row: 4, col: 2 })).toBe('Bilan[TEST][2]');
	});
	it('une cellule libre pointée : Cell[A1]', () => {
		expect(pointedRef(5, 3, tables, { row: 1, col: 4 })).toBe('Cell[D6]');
	});
	it('plusieurs cellules glissées : une plage, toute la colonne si elle est entière', () => {
		expect(rangeRef({ r0: 1, c0: 0, r1: 2, c1: 0 }, tables)).toBe('Bilan[TEST][1:2]');
		expect(rangeRef({ r0: 1, c0: 0, r1: 3, c1: 0 }, tables)).toBe('Bilan[TEST]');
	});
});

describe('« != » pour « <> »', () => {
	it('est accepté comme comparaison', () => {
		expect(checkFormula('=SI(A1!=B1;1;0)')).toBeNull();
	});
});
