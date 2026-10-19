import { describe, expect, it } from 'vitest';
import { formulaTyping, fxLeave, holdFormula, insertRefAt, qualifyRef, sheetPrefix, updateHold } from './crossref';
import { checkFormula, formulaRefs, refRect, type RefTable } from './formulaEdit';

const at = { sheet: 2, row: 4, col: 1, where: 'cell' as const };

describe('holdFormula', () => {
	it('garde texte, curseur, cellule et feuille d’origine', () => {
		expect(holdFormula(at, '=REFERENCE(', 11, 11)).toEqual({ sheet: 2, row: 4, col: 1, where: 'cell', text: '=REFERENCE(', start: 11, end: 11 });
	});

	it('garde une sélection dans le texte', () => {
		expect(holdFormula(at, '=SOMME(A1)', 7, 9)).toMatchObject({ start: 7, end: 9 });
	});

	it('curseur inconnu : en fin de texte ; hors du texte : ramené dedans', () => {
		expect(holdFormula(at, '=SOMME(', null, null)).toMatchObject({ start: 7, end: 7 });
		expect(holdFormula(at, '=A', 9, 2)).toMatchObject({ start: 2, end: 2 });
	});

	it('pas une formule : rien n’est gardé', () => {
		expect(holdFormula(at, 'Paris', 5, 5)).toBeNull();
		expect(holdFormula(at, '', 0, 0)).toBeNull();
		expect(formulaTyping('  =')).toBe(true);
	});

	it('updateHold suit la saisie dans la barre de formule', () => {
		const h = holdFormula(at, '=REFERENCE(', 11, 11)!;
		const u = updateHold(h, '=REFERENCE(Clients[Ville];', 26, 26);
		expect(u).toMatchObject({ sheet: 2, row: 4, col: 1, where: 'cell', text: '=REFERENCE(Clients[Ville];', start: 26, end: 26 });
	});
});

describe('qualifyRef', () => {
	it('cellule libre : nom de la feuille devant', () => {
		expect(qualifyRef('Cell[B2]', 'Clients')).toBe('Clients!Cell[B2]');
		expect(qualifyRef('Cell[A1:C3]', 'Clients')).toBe('Clients!Cell[A1:C3]');
		expect(qualifyRef('Cell[B2]', 'Ma feuille')).toBe("'Ma feuille'!Cell[B2]");
		expect(qualifyRef('Cell[B2]', "L'été")).toBe("'L''été'!Cell[B2]");
		expect(qualifyRef('Cell[B2]', 'Feuille2')).toBe('Feuille2!Cell[B2]');
	});

	it('référence de tableau : telle quelle (noms uniques dans le classeur)', () => {
		expect(qualifyRef('Clients[Ville]', 'Clients')).toBe('Clients[Ville]');
		expect(qualifyRef('Clients[Ville][3]', 'Données')).toBe('Clients[Ville][3]');
	});

	it('sheetPrefix comme internal/formula', () => {
		expect(sheetPrefix('')).toBe('');
		expect(sheetPrefix('Données')).toBe('Données!');
		expect(sheetPrefix('2024')).toBe("'2024'!");
	});

	it('la formule obtenue passe la vérification de syntaxe', () => {
		expect(checkFormula('=Clients!Cell[B2]+1')).toBeNull();
		expect(checkFormula("='Ma feuille'!Cell[B2]*2")).toBeNull();
		expect(checkFormula('=SOMME(Clients!Cell[A1:C3])')).toBeNull();
		expect(checkFormula('=REFERENCE(Clients[Ville];"C001")')).toBeNull();
	});
});

describe('insertRefAt', () => {
	it('au curseur, dans les arguments d’une fonction', () => {
		expect(insertRefAt('=REFERENCE()', 11, 11, 'Clients[Ville]')).toEqual({ value: '=REFERENCE(Clients[Ville])', start: 11, end: 25 });
		expect(insertRefAt('=REFERENCE(', 11, 11, 'Clients[Ville]')).toEqual({ value: '=REFERENCE(Clients[Ville]', start: 11, end: 25 });
	});

	it('au milieu du texte : la suite est gardée', () => {
		const v = '=SOMME(;B1)';
		expect(insertRefAt(v, 7, 7, 'Clients!Cell[A1]')?.value).toBe('=SOMME(Clients!Cell[A1];B1)');
	});

	it('après « = », un séparateur ou un opérateur seulement', () => {
		expect(insertRefAt('=', 1, 1, 'Cell[A1]')?.value).toBe('=Cell[A1]');
		expect(insertRefAt('=1+', 3, 3, 'Cell[A1]')?.value).toBe('=1+Cell[A1]');
		expect(insertRefAt('=SOMME(A1; ', 11, 11, 'Cell[B1]')?.value).toBe('=SOMME(A1; Cell[B1]');
		expect(insertRefAt('=SOMME(A1', 9, 9, 'Cell[B1]')).toBeNull();
		// Curseur ramené après « = » : la référence n'irait pas au bon endroit.
		expect(insertRefAt('=REFERENCE(', 1, 1, 'Clients[Ville]')?.value).toBe('=Clients[Ville]REFERENCE(');
	});

	it('remplace la sélection', () => {
		expect(insertRefAt('=SOMME(A1:A3)', 7, 12, 'Clients[Ville]')?.value).toBe('=SOMME(Clients[Ville])');
	});

	it('un nouveau clic remplace la référence insérée au clic précédent', () => {
		const first = insertRefAt('=REFERENCE(', 11, 11, 'Clients[Ville]')!;
		const last = { start: first.start, end: first.end, value: first.value };
		expect(insertRefAt(first.value, first.end, first.end, 'Clients[Nom]', last)).toEqual({ value: '=REFERENCE(Clients[Nom]', start: 11, end: 23 });
		// Texte changé depuis : pas de remplacement.
		expect(insertRefAt(first.value + ';', first.end + 1, first.end + 1, 'Cell[A1]', last)?.value).toBe('=REFERENCE(Clients[Ville];Cell[A1]');
	});
});

describe('refRect sur une autre feuille', () => {
	const tables: RefTable[] = [{ name: 'Clients', row: 0, col: 0, columns: ['Code', 'Ville'], rows: 5 }];

	it('les références d’ici sont encadrées, pas les adresses de la feuille de la formule', () => {
		const refs = formulaRefs('=REFERENCE(Clients[Ville];Cell[A1])+Clients!Cell[D2]').map((f) => f.text);
		expect(refs).toEqual(['Clients[Ville]', 'Cell[A1]', 'Clients!Cell[D2]']);
		expect(refRect('Clients[Ville]', tables, 'Clients', undefined, true)).toEqual({ r0: 0, c0: 1, r1: 5, c1: 1 });
		expect(refRect('Cell[A1]', tables, 'Clients', undefined, true)).toBeNull();
		expect(refRect('A1', tables, 'Clients', undefined, true)).toBeNull();
		expect(refRect('Clients!Cell[D2]', tables, 'Clients', undefined, true)).toEqual({ r0: 1, c0: 3, r1: 1, c1: 3 });
		expect(refRect('Analyse!Cell[D2]', tables, 'Clients', undefined, true)).toBeNull();
	});

	it('sur la feuille de la formule, rien ne change', () => {
		expect(refRect('Cell[A1]', tables, 'Clients')).toEqual({ r0: 0, c0: 0, r1: 0, c1: 0 });
	});
});

describe('fxLeave (barre de formule quittée)', () => {
	const base = { editing: true, stillFocused: false, alertOpen: false, pointing: false, text: 'Bonjour' };

	it('clic dans la grille, le ruban ou la zone de nom : la saisie est validée (demande de Louis)', () => {
		expect(fxLeave(base)).toBe('commit');
		expect(fxLeave({ ...base, text: '=1+2' })).toBe('commit');
	});

	it('Échap ou Entrée déjà passés : la barre se referme sans rien valider', () => {
		expect(fxLeave({ ...base, editing: false })).toBe('close');
	});

	it('fenêtre quittée (Alt+Tab) ou boîte « nom invalide » : la saisie reste', () => {
		expect(fxLeave({ ...base, stillFocused: true })).toBe('keep');
		expect(fxLeave({ ...base, alertOpen: true })).toBe('keep');
	});

	it('onglet de feuille : une formule est gardée pour pointer, un texte est validé', () => {
		expect(fxLeave({ ...base, pointing: true, text: '=SOMME(' })).toBe('keep');
		expect(fxLeave({ ...base, pointing: true })).toBe('commit');
	});
});

describe('clic dans la grille pendant une formule : pointage ou validation', () => {
	// Le clic insère la référence quand le curseur en attend une (insertRefAt non nul), sinon il valide.
	const points = (v: string, caret = v.length) => formulaTyping(v) && insertRefAt(v, caret, caret, '') !== null;

	it('après « = », un opérateur, « ( », « ; » ou « , » : pointage', () => {
		for (const v of ['=', '=1+', '=SOMME(', '=SI(A1;', '=MAX(1,']) expect(points(v)).toBe(true);
	});

	it('après une valeur ou une référence complète, ou un texte : validation', () => {
		for (const v of ['=1+2', '=SOMME(A1)', '=SOMME(A1', 'Bonjour', '12+']) expect(points(v)).toBe(false);
	});
});
