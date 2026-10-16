import { describe, expect, it } from 'vitest';
import { clampZoom, parseZoom, sliderOf, stepZoom, zoomLabel, zoomOfSlider } from './zoom';

describe('zoom', () => {
	it('borne et arrondit', () => {
		expect(clampZoom(0.01)).toBe(0.1);
		expect(clampZoom(9)).toBe(4);
		expect(clampZoom(1.234)).toBe(1.23);
		expect(clampZoom(NaN)).toBe(1);
	});

	it('avance par pas de 10 % calés sur les dizaines', () => {
		expect(stepZoom(1, 1)).toBe(1.1);
		expect(stepZoom(1, -1)).toBe(0.9);
		expect(stepZoom(1.15, 1)).toBe(1.2);
		expect(stepZoom(1.15, -1)).toBe(1.1);
		expect(stepZoom(0.1, -1)).toBe(0.1);
		expect(stepZoom(4, 1)).toBe(4);
	});

	it('libellé et saisie', () => {
		expect(zoomLabel(1.25)).toBe('125 %');
		expect(parseZoom('80 %')).toBe(0.8);
		expect(parseZoom('abc')).toBeNull();
		expect(parseZoom('900')).toBeNull();
	});

	it('curseur : 100 % au milieu, aller-retour', () => {
		expect(sliderOf(1)).toBe(0.5);
		expect(zoomOfSlider(0.5)).toBe(1);
		expect(zoomOfSlider(0)).toBe(0.1);
		expect(zoomOfSlider(1)).toBe(4);
		for (const z of [0.5, 0.75, 1.5, 2]) expect(zoomOfSlider(sliderOf(z))).toBeCloseTo(z, 2);
	});
});
