import { describe, expect, it } from 'vitest';
import { revealX, viewportBox } from './mobile';

describe('revealX', () => {
	it('ne bouge pas un élément déjà visible', () => {
		expect(revealX(100, 180, 50, 300)).toBe(50);
	});
	it('défile pour montrer un élément à droite ou à gauche, avec la marge', () => {
		expect(revealX(400, 480, 0, 300)).toBe(196);
		expect(revealX(40, 120, 200, 300)).toBe(24);
	});
	it('montre le début d’un élément plus large que la vue', () => {
		expect(revealX(500, 900, 0, 300)).toBe(484);
		expect(revealX(5, 900, 300, 300)).toBe(0);
	});
});

describe('viewportBox', () => {
	it('rien à faire quand toute la fenêtre se voit', () => {
		expect(viewportBox(800, 0, 800)).toBeNull();
		expect(viewportBox(799.5, 0, 800)).toBeNull();
		expect(viewportBox(0, 0, 800)).toBeNull();
	});
	it('ignore le zoom de la page au doigt', () => {
		expect(viewportBox(400, 200, 800, 2)).toBeNull();
	});
	it('clavier à l’écran : hauteur visible et décalage', () => {
		expect(viewportBox(420.4, 120.6, 800)).toEqual({ h: 420, top: 121 });
		expect(viewportBox(420, -3, 800)).toEqual({ h: 420, top: 0 });
	});
});
