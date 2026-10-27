import { describe, expect, it } from 'vitest';
import { MENU_GRACE_MS, onObject, PAN_SLOP, PanGesture, pansWith, SpaceHold } from './pan';

describe('main sur la feuille (pan.ts)', () => {
	it('boutons qui font glisser : droit, milieu, gauche avec Espace', () => {
		expect(pansWith(2, false)).toBe(true);
		expect(pansWith(1, false)).toBe(true);
		expect(pansWith(0, false)).toBe(false);
		expect(pansWith(0, true)).toBe(true);
	});

	it('sur un objet : milieu et Espace + gauche pour la feuille, droit partagé, gauche à l’objet', () => {
		expect(onObject(1, false)).toBe('pan');
		expect(onObject(0, true)).toBe('pan');
		expect(onObject(2, false)).toBe('menu');
		expect(onObject(2, true)).toBe('menu');
		expect(onObject(0, false)).toBe('object');
		expect(onObject(3, false)).toBe('object');
	});

	it('sous le seuil, rien ne glisse ; au-delà, tout le chemin depuis l’appui', () => {
		const p = new PanGesture();
		p.begin(2, 100, 100);
		expect(p.move(102, 101)).toBeNull();
		expect(p.panning).toBe(false);
		expect(p.move(100 - PAN_SLOP, 100)).toEqual({ dx: PAN_SLOP, dy: 0 });
		expect(p.panning).toBe(true);
		// Glisser vers la gauche et le haut montre la droite et le bas.
		expect(p.move(90, 80)).toEqual({ dx: 100 - PAN_SLOP - 90, dy: 20 });
		expect(p.move(95, 85)).toEqual({ dx: -5, dy: -5 });
	});

	it('clic droit sans bouger (Windows : menu au relâcher) : le menu s’ouvre', () => {
		const p = new PanGesture<string>();
		p.begin(2, 10, 10);
		p.move(11, 12);
		expect(p.end(2, 1000)).toEqual({ panned: false, menu: null });
		expect(p.menu('m', 1001)).toBe('open');
	});

	it('clic droit glissé (Windows) : le menu qui suit le relâcher est écarté, une fois', () => {
		const p = new PanGesture<string>();
		p.begin(2, 10, 10);
		p.move(40, 10);
		expect(p.end(2, 1000)).toEqual({ panned: true, menu: null });
		expect(p.menu('m', 1001)).toBe('drop');
		expect(p.menu('m', 1002)).toBe('open');
	});

	it('un menu bien plus tard n’est plus écarté', () => {
		const p = new PanGesture<string>();
		p.begin(2, 10, 10);
		p.move(40, 10);
		p.end(2, 1000);
		expect(p.menu('m', 1000 + MENU_GRACE_MS + 1)).toBe('open');
	});

	it('menu à l’appui (macOS) : retenu, ouvert au relâcher sans glissé', () => {
		const p = new PanGesture<string>();
		p.begin(2, 10, 10);
		expect(p.menu('m', 1)).toBe('defer');
		expect(p.end(2, 5)).toEqual({ panned: false, menu: 'm' });
	});

	it('menu à l’appui (macOS) puis glissé : abandonné', () => {
		const p = new PanGesture<string>();
		p.begin(2, 10, 10);
		expect(p.menu('m', 1)).toBe('defer');
		p.move(30, 30);
		expect(p.menu('m2', 2)).toBe('drop');
		expect(p.end(2, 5)).toEqual({ panned: true, menu: null });
	});

	it('clic du milieu glissé : pas de menu en jeu ; autre bouton relâché : ignoré', () => {
		const p = new PanGesture<string>();
		p.begin(1, 0, 0);
		p.move(10, 0);
		expect(p.end(0, 1)).toBeNull();
		expect(p.active).toBe(true);
		expect(p.end(1, 2)).toEqual({ panned: true, menu: null });
		expect(p.menu('m', 3)).toBe('open');
	});

	it('nouvel appui : l’écart du menu précédent tombe', () => {
		const p = new PanGesture<string>();
		p.begin(2, 0, 0);
		p.move(20, 0);
		p.end(2, 100);
		p.begin(2, 0, 0);
		p.end(2, 120);
		expect(p.menu('m', 121)).toBe('open');
	});
});

describe('Espace tenue', () => {
	it('appuyée et relâchée sans clic : l’espace se saisit', () => {
		const s = new SpaceHold();
		s.press();
		expect(s.down).toBe(true);
		expect(s.release()).toBe(true);
		expect(s.down).toBe(false);
	});

	it('un clic pendant qu’elle est tenue : pas d’espace', () => {
		const s = new SpaceHold();
		s.press();
		s.click();
		expect(s.release()).toBe(false);
	});

	it('frappe rapide : l’espace passe avant la touche suivante, une seule fois', () => {
		const s = new SpaceHold();
		s.press();
		expect(s.flush()).toBe(true);
		expect(s.flush()).toBe(false);
		expect(s.release()).toBe(false);
	});

	it('relâchée sans appui : rien', () => {
		expect(new SpaceHold().release()).toBe(false);
	});
});
