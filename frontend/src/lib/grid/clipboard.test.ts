import { describe, expect, it } from 'vitest';
import { contains, followClip, overlaps, parseTSV, pasteArea, richClip, sameClipText, tileBlock, tiles, toTSV, type Clip } from './clipboard';

describe('presse-papiers', () => {
	it('lit le texte copié depuis Excel', () => {
		expect(parseTSV('Ville\tHabitants\r\nParis\t2 100 000\r\nLyon\t\r\n')).toEqual([
			['Ville', 'Habitants'],
			['Paris', '2 100 000'],
			['Lyon', '']
		]);
	});

	it('gère les champs entre guillemets (tabulation, saut de ligne, guillemet)', () => {
		expect(parseTSV('"a\tb"\t"ligne 1\nligne 2"\t"dit ""oui"""\n')).toEqual([['a\tb', 'ligne 1\nligne 2', 'dit "oui"']]);
	});

	it('accepte un texte sans saut de ligne final', () => {
		expect(parseTSV('x\ty')).toEqual([['x', 'y']]);
	});

	it('fait l’aller-retour', () => {
		const rows = [['a', 'b\tc'], ['"q"', '']];
		expect(parseTSV(toTSV(rows))).toEqual(rows);
	});
});

describe('collage à la Excel', () => {
	const sel = (r0: number, c0: number, r1: number, c1: number) => ({ r0, c0, r1, c1 });

	it('une cellule copiée remplit toute la sélection', () => {
		expect(tiles(1, 1, sel(0, 0, 2, 1))).toEqual([3, 2]);
		expect(tileBlock([['x']], sel(0, 0, 1, 2))).toEqual([
			['x', 'x', 'x'],
			['x', 'x', 'x']
		]);
	});

	it('répète un bloc dont la taille divise la sélection', () => {
		expect(tileBlock([['a'], ['b']], sel(5, 5, 8, 6))).toEqual([
			['a', 'a'],
			['b', 'b'],
			['a', 'a'],
			['b', 'b']
		]);
		expect(pasteArea(2, 1, sel(5, 5, 8, 6))).toEqual(sel(5, 5, 8, 6));
	});

	it('colle une fois un bloc qui ne divise pas la sélection', () => {
		expect(tiles(2, 2, sel(0, 0, 2, 3))).toEqual([1, 1]);
		expect(tileBlock([['a', 'b']], sel(0, 0, 0, 2))).toEqual([['a', 'b']]);
		expect(pasteArea(3, 2, sel(4, 4, 4, 4))).toEqual(sel(4, 4, 6, 5));
	});

	it('complète les lignes courtes', () => {
		expect(tileBlock([['a', 'b'], ['c']], sel(0, 0, 1, 3))).toEqual([
			['a', 'b', 'a', 'b'],
			['c', '', 'c', '']
		]);
	});

	it('reconnaît le texte de la dernière copie', () => {
		expect(sameClipText('a\tb\r\nc\r\n', 'a\tb\nc')).toBe(true);
		expect(sameClipText('a', 'b')).toBe(false);
		const clip: Clip = { ...sel(0, 0, 0, 1), sheet: 1, cut: false, text: 'a\tb\r\n', values: [['a', 'b']], plain: false };
		expect(richClip(clip, 'a\tb\n')).toBe(clip);
		expect(richClip(clip, 'autre')).toBeNull();
		expect(richClip({ ...clip, plain: true }, 'a\tb')).toBeNull();
		expect(richClip(null, 'a')).toBeNull();
	});

	it('compare des zones', () => {
		expect(overlaps(sel(0, 0, 2, 2), sel(2, 2, 3, 3))).toBe(true);
		expect(overlaps(sel(0, 0, 2, 2), sel(3, 0, 3, 0))).toBe(false);
		expect(contains(sel(0, 0, 5, 5), sel(1, 1, 2, 2))).toBe(true);
		expect(contains(sel(1, 1, 2, 2), sel(0, 0, 5, 5))).toBe(false);
	});
});

describe('cadre de copie après un changement de la feuille', () => {
	const box = (id: number, r0: number, c0: number, r1: number, c1: number) => ({ id, r0, c0, r1, c1 });
	const clip = { r0: 0, c0: 0, r1: 4, c1: 2 };
	it('suit le tableau copié poussé en cascade par le collage', () => {
		// Copie du tableau 1 (A1:C5), collé en B3 : le nouveau tableau 2 reste, le 1 part à droite.
		expect(followClip(clip, [box(1, 0, 0, 4, 2)], [box(1, 0, 6, 4, 8), box(2, 2, 1, 6, 3)])).toEqual({ r0: 0, c0: 6, r1: 4, c1: 8 });
	});
	it('reste quand rien ne l’a touché', () => {
		const before = [box(1, 0, 0, 4, 2), box(3, 0, 10, 4, 12)];
		expect(followClip(clip, before, [box(1, 0, 0, 4, 2), box(3, 0, 14, 4, 16)])).toBe(clip);
	});
	it('disparaît quand la zone ne désigne plus ce qui a été copié', () => {
		// Tableau agrandi, supprimé, ou un autre arrivé dans la zone.
		expect(followClip(clip, [box(1, 0, 0, 4, 2)], [box(1, 0, 0, 5, 2)])).toBeNull();
		expect(followClip(clip, [box(1, 0, 0, 4, 2)], [])).toBeNull();
		expect(followClip({ r0: 0, c0: 0, r1: 4, c1: 1 }, [], [box(2, 1, 1, 3, 3)])).toBeNull();
		// Cellules libres et tableau copiés ensemble, le tableau poussé : les cellules libres, elles, sont restées.
		expect(followClip({ r0: 0, c0: 0, r1: 6, c1: 2 }, [box(1, 0, 0, 4, 2)], [box(1, 0, 5, 4, 7)])).toBeNull();
	});
});
