import { describe, expect, it } from 'vitest';
import { apply, argKindAt, callAt, needsNextArg, toNextArg, signature, signatureParts, suggest, type Catalog, type FunctionDoc } from './autocomplete';

const cat: Catalog = {
	functions: [
		{ name: 'SI', other: 'IF', arity: 'de 2 à 3 arguments' },
		{ name: 'SIERREUR', other: 'IFERROR', arity: '2 argument(s)' },
		{ name: 'SOMME', other: 'SUM', arity: 'au moins 1 argument(s)' },
		{ name: 'REFERENCE', other: 'REFERENCE', arity: '3 argument(s)' }
	],
	tables: [
		{ name: 'Ventes', columns: ['Produit', 'Prix', 'Qté'] },
		{ name: 'Clients', columns: ['Id', 'Nom'] }
	]
};

describe('autocomplétion des formules', () => {
	it('propose les fonctions et tableaux qui commencent par le préfixe', () => {
		const c = suggest('=so', 3, cat)!;
		expect(c.from).toBe(1);
		expect(c.items.map((i) => i.label)).toEqual(['SOMME']);
		expect(suggest('=SOMME(v', 8, cat)!.items[0]).toMatchObject({ label: 'Ventes', insert: 'Ventes[' });
	});

	it('propose les colonnes après « [ » (le @ d’Excel reste accepté)', () => {
		expect(suggest('=SOMME(Ventes[p', 15, cat)!.items.map((i) => i.insert)).toEqual(['Produit]', 'Prix]']);
		expect(suggest('=[@q', 4, cat, 'Ventes')!.items[0].insert).toBe('Qté]');
		expect(suggest('=[@q', 4, cat)).toBeNull(); // pas de tableau courant
	});

	it('dans une ligne du tableau, [Colonne] est la valeur de la ligne', () => {
		expect(suggest('=[Prix]*[q', 10, cat, 'Ventes')!.items.map((i) => i.insert)).toEqual(['Qté]']);
	});

	it('ignore le texte hors formule et les chaînes', () => {
		expect(suggest('so', 2, cat)).toBeNull();
		expect(suggest('="so', 4, cat)).toBeNull();
	});

	it('applique une suggestion', () => {
		const c = suggest('=1+so', 5, cat)!;
		expect(apply('=1+so', 5, c.from, c.items[0])).toEqual({ text: '=1+SOMME(', caret: 9 });
	});

	it('donne la fonction en cours et le rang de l’argument', () => {
		const at = (t: string) => {
			const s = signature(t, t.length, cat);
			return s && `${s.fn.name}:${s.index}`;
		};
		expect(at('=SI(A>1;SOMME(1')).toBe('SOMME:0');
		expect(at('=SI(A>1;SOMME(1);')).toBe('SI:2');
		expect(at('=IF(')).toBe('SI:0');
		expect(at('=1+2')).toBeNull();
	});

	it('compte les séparateurs au niveau de l’appel en cours', () => {
		expect(callAt('=SOMME.SI(Ventes[Région]; "Nord"; ', 33)).toEqual({
			name: 'SOMME.SI',
			index: 2
		});
		// Chaînes, appels imbriqués et regroupements ignorés.
		expect(callAt('=SI(";;(";SOMME(1;2);(1;2', 25)).toEqual({
			name: 'SI',
			index: 2
		});
		expect(callAt('=SI(A;"a;b', 10)).toEqual({ name: 'SI', index: 1 }); // curseur dans la chaîne
		// Noms de colonnes et adresses : texte littéral.
		expect(callAt('=SOMME([Prix (€; HT)];Cell[A1];', 31)).toEqual({
			name: 'SOMME',
			index: 2
		});
		expect(callAt('=SOMME(Ventes[Pri', 17)).toEqual({
			name: 'SOMME',
			index: 0
		});
		expect(callAt("=SI('Ma (feuille)'!Cell[A1];", 28)).toEqual({
			name: 'SI',
			index: 1
		});
		// La clé de Tableau[Colonne][clé] est une expression.
		expect(callAt('=SI(1;Ventes[Prix][GAUCHE(x;', 28)).toEqual({
			name: 'GAUCHE',
			index: 1
		});
		expect(callAt('=SI(1;Ventes[Prix][GAUCHE(x;1)];', 32)).toEqual({
			name: 'SI',
			index: 2
		});
		// Hors des parenthèses, ou parenthèses refermées.
		expect(callAt('=SOMME(1;2)', 11)).toBeNull();
		expect(callAt('SOMME(1', 7)).toBeNull();
	});

	it('met en gras l’argument en cours dans la signature', () => {
		const sumif: FunctionDoc = {
			name: 'SOMME.SI',
			other: 'SUMIF',
			arity: '',
			args: [
				{ name: 'plage', desc: 'p' },
				{ name: 'critère', desc: 'c' },
				{ name: 'plage_somme', desc: 's', optional: true }
			]
		};
		const show = (f: FunctionDoc, i: number) =>
			signatureParts(f, i)
				.parts.map((p) => (p.on ? `**${p.text}**` : p.text))
				.join('');
		expect(show(sumif, 1)).toBe('SOMME.SI(plage; **critère**; [plage_somme])');
		expect(show(sumif, 2)).toBe('SOMME.SI(plage; critère; **[plage_somme]**)');
		expect(signatureParts(sumif, 1).arg?.desc).toBe('c');
		expect(show(sumif, 5)).toBe('SOMME.SI(plage; critère; [plage_somme])'); // argument en trop
		expect(signatureParts(sumif, 5).arg).toBeNull();

		const sum: FunctionDoc = {
			name: 'SOMME',
			other: 'SUM',
			arity: '',
			repeat: 1,
			args: [{ name: 'valeur1', desc: 'v' }]
		};
		expect(show(sum, 0)).toBe('SOMME(**valeur1**; [valeur2]; …)');
		expect(show(sum, 1)).toBe('SOMME(valeur1; [**valeur2**]; …)');
		expect(show(sum, 4)).toBe('SOMME(valeur1; …; [**valeur5**]; …)');
		expect(signatureParts(sum, 4).arg?.name).toBe('valeur5');

		const ifs: FunctionDoc = {
			name: 'SOMME.SI.ENS',
			other: 'SUMIFS',
			arity: '',
			repeat: 2,
			args: [
				{ name: 'plage_somme', desc: '' },
				{ name: 'plage_critères1', desc: '' },
				{ name: 'critère1', desc: '' }
			]
		};
		expect(show(ifs, 2)).toBe('SOMME.SI.ENS(plage_somme; plage_critères1; **critère1**; [plage_critères2; critère2]; …)');
		expect(show(ifs, 4)).toBe('SOMME.SI.ENS(plage_somme; plage_critères1; critère1; [plage_critères2; **critère2**]; …)');
		expect(show(ifs, 5)).toBe('SOMME.SI.ENS(plage_somme; plage_critères1; critère1; …; [**plage_critères3**; critère3]; …)');
		expect(show({ name: 'PI', other: 'PI', arity: '', args: [] }, 0)).toBe('PI()');

		// LET : paires nom; valeur répétées, puis le calcul.
		const let_: FunctionDoc = {
			name: 'LET',
			other: 'LET',
			arity: '',
			args: [
				{ name: 'nom1', desc: 'n' },
				{ name: 'valeur1', desc: 'v' },
				{ name: 'calcul', desc: 'Le calcul.' }
			]
		};
		expect(show(let_, 0)).toBe('LET(**nom1**; valeur1; [nom2; valeur2]; …; calcul)');
		expect(show(let_, 1)).toBe('LET(nom1; **valeur1**; [nom2; valeur2]; …; calcul)');
		expect(show(let_, 2)).toBe('LET(nom1; valeur1; [**nom2**; valeur2]; …; **calcul**)');
		expect(signatureParts(let_, 2).arg?.name).toBe('calcul ou nom2');
		expect(show(let_, 3)).toBe('LET(nom1; valeur1; [nom2; **valeur2**]; …; calcul)');
		expect(show(let_, 4)).toBe('LET(nom1; valeur1; …; [**nom3**; valeur3]; …; **calcul**)');
		expect(signatureParts(let_, 5).arg?.name).toBe('valeur3');
	});

	it('ne compte pas les « ; » d’une constante matricielle', () => {
		const at = (s: string) => callAt(s, s.length);
		expect(at('=FRACTIONNER.TEXTE(Cell[A1]; {",";";"')).toEqual({ name: 'FRACTIONNER.TEXTE', index: 1 });
		expect(at('=FRACTIONNER.TEXTE(Cell[A1]; {",";";"}; ')).toEqual({ name: 'FRACTIONNER.TEXTE', index: 2 });
		expect(at('=SOMME({1;2;3}; ')).toEqual({ name: 'SOMME', index: 1 });
	});
});

describe('argKindAt', () => {
	const kinds = {
		functions: [
			{ name: 'REFERENCE', other: 'REFERENCE', arity: '2', args: [{ name: 'colonne', desc: '', kind: 'column' }, { name: 'clé', desc: '' }] },
			{ name: 'EXISTE', other: 'EXISTS', arity: '2', args: [{ name: 'tableau', desc: '', kind: 'table' }, { name: 'clé', desc: '' }] },
			{ name: 'SOMME', other: 'SUM', arity: '1+', args: [{ name: 'valeur1', desc: '', kind: 'column' }], repeat: 1 }
		],
		tables: []
	} as unknown as Catalog;
	it("donne ce qu'attend l'argument en cours", () => {
		expect(argKindAt('=REFERENCE(', 11, kinds)).toBe('column');
		expect(argKindAt('=REFERENCE(Clients[Ville]; ', 27, kinds)).toBe('');
		expect(argKindAt('=EXISTE(', 8, kinds)).toBe('table');
		expect(argKindAt('=SOMME(A; ', 10, kinds)).toBe('column');
		expect(argKindAt('=1+', 3, kinds)).toBe('');
	});
});

describe('argument suivant après un pointage', () => {
	const cat = {
		functions: [
			{ name: 'REFERENCE', other: 'REFERENCE', arity: '2', args: [{ name: 'colonne', desc: '' }, { name: 'clé', desc: '' }] },
			{ name: 'SI', other: 'IF', arity: '2-3', args: [{ name: 'test', desc: '' }, { name: 'si_vrai', desc: '' }, { name: 'si_faux', desc: '', optional: true }] },
			{ name: 'SOMME', other: 'SUM', arity: '1+', args: [{ name: 'valeur1', desc: '' }], repeat: 1 },
			{ name: 'SOMME.SI.ENS', other: 'SUMIFS', arity: '3+', args: [{ name: 'plage_somme', desc: '' }, { name: 'plage1', desc: '' }, { name: 'critère1', desc: '' }], repeat: 2 }
		],
		tables: []
	} as unknown as Catalog;
	it('passe à un argument obligatoire, jamais après le dernier', () => {
		expect(needsNextArg('=REFERENCE(T[Col]', 17, cat)).toBe(true);
		expect(needsNextArg('=REFERENCE(T[Col]; [Id]', 23, cat)).toBe(false);
		expect(needsNextArg('=SI([A]', 7, cat)).toBe(true);
		expect(needsNextArg('=SI([A]; [B]', 12, cat)).toBe(false); // si_faux est facultatif
		expect(needsNextArg('=SOMME(Cell[A1]', 15, cat)).toBe(false);
		expect(needsNextArg('=SOMME.SI.ENS(a; b; c; d', 24, cat)).toBe(true); // critère2 suit plage2
		expect(needsNextArg('=SOMME.SI.ENS(a; b; c; d; e', 27, cat)).toBe(false);
		// LET : après une valeur vient un nom ou le calcul ; le calcul peut être le dernier.
		const withLet = {
			...cat,
			functions: [...cat.functions, { name: 'LET', other: 'LET', arity: '', args: [{ name: 'nom1', desc: '' }, { name: 'valeur1', desc: '' }, { name: 'calcul', desc: '' }] }]
		} as unknown as Catalog;
		expect(needsNextArg('=LET(x; Cell[A1]', 16, withLet)).toBe(true);
		expect(needsNextArg('=LET(x; 1; Cell[A1]', 19, withLet)).toBe(false);
		expect(needsNextArg('=LET(x; 1; y; Cell[A1]', 22, withLet)).toBe(true);
	});
	it('saute un « ; » déjà là, sinon en ajoute un', () => {
		expect(toNextArg('=SI([A])', 7)).toEqual({ text: '=SI([A]; )', caret: 9 });
		expect(toNextArg('=SI([A]; 1)', 7)).toEqual({ text: '=SI([A]; 1)', caret: 9 });
	});
});
