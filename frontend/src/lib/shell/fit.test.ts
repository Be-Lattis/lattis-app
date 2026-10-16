import { describe, expect, it } from 'vitest';
import { fit, groupWidth, type GroupShape } from './fit';

const groups: GroupShape[] = [
	{ title: 'Annuler', labels: ['Annuler', 'Rétablir'] },
	{ title: 'Police', labels: ['Gras', 'Italique', 'Souligné', null, null, 'Effacer'] },
	{ title: 'Alignement', labels: ['Gauche', 'Centrer', 'Droite'] },
	{ title: 'Nombre', labels: ['0,00', 'Monétaire', 'Pourcentage', 'Standard'] }
];

describe('ruban adaptatif', () => {
	const full = groups.reduce((s, g) => s + groupWidth(g, 'large'), 0);
	it('tout en grand quand la place suffit', () => {
		expect(fit(groups, full + 10)).toEqual(['large', 'large', 'large', 'large']);
	});
	it('compacte depuis la droite', () => {
		const m = fit(groups, full - 20);
		expect(m[3]).toBe('small');
		expect(m[0]).toBe('large');
	});
	it('replie tout quand il ne reste presque rien', () => {
		expect(fit(groups, 100)).toEqual(['collapsed', 'collapsed', 'collapsed', 'collapsed']);
	});
	it('chaque mode est plus étroit que le précédent', () => {
		for (const g of groups) {
			expect(groupWidth(g, 'small')).toBeLessThanOrEqual(groupWidth(g, 'large'));
			expect(groupWidth(g, 'icon')).toBeLessThanOrEqual(groupWidth(g, 'small'));
		}
	});
});
