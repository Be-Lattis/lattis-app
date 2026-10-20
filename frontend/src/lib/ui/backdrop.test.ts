import { describe, expect, it, vi } from 'vitest';
import { backdrop } from './backdrop';

// Fond factice : les écouteurs reçoivent un évènement dont on choisit la cible.
function setup() {
	const on: Record<string, (e: { target: unknown }) => void> = {};
	const back = { addEventListener: (t: string, f: (e: { target: unknown }) => void) => (on[t] = f), removeEventListener: () => {} };
	const box = {};
	const close = vi.fn();
	backdrop(back as unknown as HTMLElement, close);
	// Le « click » d'un geste commencé et fini sur deux éléments vise leur ancêtre commun (le fond).
	const gesture = (down: unknown, up: unknown) => {
		on.pointerdown({ target: down });
		on.click({ target: down === up ? up : back });
	};
	return { back, box, close, gesture };
}

describe('fond de boîte modale', () => {
	it('se ferme sur un clic enfoncé et relâché sur le fond', () => {
		const { back, close, gesture } = setup();
		gesture(back, back);
		expect(close).toHaveBeenCalledTimes(1);
	});
	it('reste ouvert après un cliqué-glissé commencé dans la boîte et relâché dehors', () => {
		const { back, box, close, gesture } = setup();
		gesture(box, back);
		expect(close).not.toHaveBeenCalled();
	});
	it('reste ouvert sur un clic dans la boîte', () => {
		const { box, close, gesture } = setup();
		gesture(box, box);
		expect(close).not.toHaveBeenCalled();
	});
});
