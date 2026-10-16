import { afterEach, describe, expect, it, vi } from 'vitest';
import { dismiss, distance } from './dismiss';

describe('distance au menu', () => {
	const r = { left: 10, top: 10, right: 110, bottom: 60 };
	it('nulle à l’intérieur', () => expect(distance(r, 50, 30)).toBe(0));
	it('horizontale, verticale, diagonale', () => {
		expect(distance(r, 150, 30)).toBe(40);
		expect(distance(r, 50, 0)).toBe(10);
		expect(distance(r, 113, 64)).toBe(5);
	});
});

// Fenêtre factice (pas de DOM sous vitest) : un EventTarget suffit.
function setup() {
	vi.useFakeTimers();
	const win = new EventTarget();
	vi.stubGlobal('window', win);
	const inside = {};
	const node = {
		getBoundingClientRect: () => ({ left: 10, top: 10, right: 110, bottom: 60 }),
		contains: (t: unknown) => t === inside
	} as unknown as HTMLElement;
	const close = vi.fn();
	const action = dismiss(node, close);
	vi.runAllTimers();
	const fire = (type: string, props: Record<string, unknown> = {}) => {
		const e = new Event(type);
		for (const [k, v] of Object.entries(props)) Object.defineProperty(e, k, { value: v });
		win.dispatchEvent(e);
	};
	return { close, action, fire, inside };
}

describe('fermeture des menus flottants', () => {
	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});
	it('Échap ferme le menu', () => {
		const { close, fire } = setup();
		fire('keydown', { key: 'a' });
		expect(close).not.toHaveBeenCalled();
		fire('keydown', { key: 'Escape' });
		expect(close).toHaveBeenCalled();
	});
	it('un clic ailleurs (y compris pointerdown de la grille) ferme le menu, pas un clic dedans', () => {
		const { close, fire, inside } = setup();
		fire('pointerdown', { target: inside });
		expect(close).not.toHaveBeenCalled();
		fire('pointerdown', { target: {} });
		expect(close).toHaveBeenCalledTimes(1);
	});
	it('la souris loin du menu le ferme, sauf sur un sous-menu qui en déborde', () => {
		const { close, fire, inside } = setup();
		fire('mousemove', { clientX: 300, clientY: 30, target: inside });
		expect(close).not.toHaveBeenCalled();
		fire('mousemove', { clientX: 300, clientY: 30, target: {} });
		expect(close).toHaveBeenCalledTimes(1);
	});
	it('redimensionner la fenêtre ou lui faire perdre le focus ferme le menu', () => {
		const { close, fire } = setup();
		fire('resize');
		fire('blur');
		expect(close).toHaveBeenCalledTimes(2);
	});
	it('plus rien après destroy', () => {
		const { close, fire, action } = setup();
		action.destroy();
		fire('keydown', { key: 'Escape' });
		expect(close).not.toHaveBeenCalled();
		fire('resize');
		expect(close).not.toHaveBeenCalled();
	});
});
