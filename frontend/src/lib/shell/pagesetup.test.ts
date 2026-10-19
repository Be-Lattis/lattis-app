import { describe, expect, it } from 'vitest';
import { copySetup, emptySetup, pageAction, hfLabel, marginPreset, MARGINS, parseRange, parseSpan, presetMargins, rangeA1, renderHF, scaleLabel, spanA1, withBreak, withFit, withoutBreak, withoutBreaks, withScale } from './pagesetup';

describe('mise en page', () => {
	it('reconnaît les marges prédéfinies', () => {
		expect(marginPreset(emptySetup())).toBe('normal');
		expect(marginPreset({ ...emptySetup(), margins: presetMargins('wide') })).toBe('wide');
		expect(marginPreset({ ...emptySetup(), margins: { ...MARGINS.narrow } })).toBe('narrow');
		expect(marginPreset({ ...emptySetup(), margins: { ...MARGINS.narrow, top: 3 } })).toBe('custom');
		expect(presetMargins('normal')).toBeNull();
		// Arrondies au centième (boîte Mise en page) : toujours Normales.
		expect(marginPreset({ ...emptySetup(), margins: { top: 1.91, bottom: 1.91, left: 1.78, right: 1.78, header: 0.76, footer: 0.76 } })).toBe('normal');
	});

	it('applique les commandes du ruban', () => {
		const here = { row: 4, col: 2, sel: { r0: 1, c0: 0, r1: 9, c1: 3 } };
		const p = emptySetup();
		expect(pageAction(p, { kind: 'orientation', landscape: true }, here)).toMatchObject({ setup: { landscape: true }, label: 'Orientation' });
		expect(pageAction(p, { kind: 'margins', preset: 'narrow' }, here)).toMatchObject({ setup: { margins: MARGINS.narrow }, label: 'Marges' });
		expect(pageAction(p, { kind: 'paper', paper: 'A4' }, here)).toMatchObject({ setup: { paper: '' } });
		const a = pageAction(p, { kind: 'area', clear: false }, here);
		expect(a).toMatchObject({ setup: { area: here.sel }, label: "Zone d'impression" });
		expect(pageAction(p, { kind: 'break', op: 'insert' }, here)).toMatchObject({ setup: { rowBreaks: [4], colBreaks: [2] } });
		expect(pageAction(p, { kind: 'break', op: 'remove' }, here)).toHaveProperty('notice');
		expect(pageAction(p, { kind: 'break', op: 'reset' }, here)).toHaveProperty('notice');
		expect(pageAction(p, { kind: 'print', what: 'headings', on: true }, here)).toMatchObject({ setup: { headings: true } });
		expect(pageAction(p, { kind: 'dialog', tab: 'sheet' }, here)).toBeNull();
		expect(p).toEqual(emptySetup());
	});

	it("ajuste en pages ou met à l'échelle, l'un excluant l'autre", () => {
		let p = withScale(emptySetup(), 80);
		expect(p.scale).toBe(80);
		expect(scaleLabel(p)).toBe('80 %');
		p = withFit(p, true, 1);
		expect(p).toMatchObject({ fitWide: 1, scale: 0 });
		expect(scaleLabel(p)).toBe('Ajusté : 1 page en largeur, auto en hauteur');
		p = withScale(p, 100);
		expect(p).toMatchObject({ fitWide: 0, fitTall: 0, scale: 0 });
		expect(withScale(p, 1000).scale).toBe(400);
	});

	it('insère et supprime les sauts autour de la cellule active, comme Excel', () => {
		const p = emptySetup();
		expect(withBreak(p, 0, 0)).toBeNull();
		const a = withBreak(p, 10, 0)!;
		expect(a.rowBreaks).toEqual([10]);
		expect(a.colBreaks ?? []).toEqual([]);
		const b = withBreak(a, 5, 3)!;
		expect(b.rowBreaks).toEqual([5, 10]);
		expect(b.colBreaks).toEqual([3]);
		expect(a.rowBreaks).toEqual([10]); // copie, l'original ne bouge pas
		expect(withoutBreak(b, 7, 7)).toBeNull();
		expect(withoutBreak(b, 10, 3)).toMatchObject({ rowBreaks: [5], colBreaks: [] });
		expect(withoutBreaks(b)).toMatchObject({ rowBreaks: [], colBreaks: [] });
	});

	it('lit et écrit les adresses de la boîte', () => {
		expect(parseRange('$a$1:$f$40')).toEqual({ r0: 0, c0: 0, r1: 39, c1: 5 });
		expect(parseRange('F40:A1')).toEqual({ r0: 0, c0: 0, r1: 39, c1: 5 });
		expect(parseRange(' ')).toBeNull();
		expect(parseRange('A0')).toBeUndefined();
		expect(rangeA1({ r0: 0, c0: 0, r1: 39, c1: 27 })).toBe('$A$1:$AB$40');
		expect(parseSpan('$1:$2', true)).toEqual({ from: 0, to: 1 });
		expect(parseSpan('c', false)).toEqual({ from: 2, to: 2 });
		expect(parseSpan('A:1', false)).toBeUndefined();
		expect(spanA1({ from: 0, to: 1 }, true)).toBe('$1:$2');
		expect(spanA1({ from: 26, to: 27 }, false)).toBe('$AA:$AB');
	});

	it('rend les codes des en-têtes', () => {
		const ctx = { page: 2, pages: 5, sheet: 'Ventes', file: 'Stock.lattis', date: new Date(2026, 9, 19, 8, 5) };
		expect(renderHF('Page &[Page] sur &[Pages] – &[Onglet] – &[Fichier] – &[Date] &[Heure] – R&&D', ctx)).toBe('Page 2 sur 5 – Ventes – Stock.lattis – 19/10/2026 08:05 – R&D');
		expect(hfLabel({}, ctx)).toBe('(aucun)');
		expect(hfLabel({ center: 'Page &[Page] sur &[Pages]' }, { ...ctx, page: 1, pages: 1 })).toBe('Page 1 sur ?');
		expect(hfLabel({ left: 'Confidentiel', right: '&[Onglet]' }, ctx)).toBe('Confidentiel ; Ventes');
	});

	it('copie sans partager les objets', () => {
		const p = { ...emptySetup(), area: { r0: 0, c0: 0, r1: 1, c1: 1 } };
		const c = copySetup(p);
		c.area!.r1 = 9;
		expect(p.area.r1).toBe(1);
		expect(copySetup(null)).toEqual({ header: {}, footer: {} });
	});
});
