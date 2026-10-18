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
	it('Styles, Cellules et Édition gardent leurs libellés à 1280 et 1400 px (ruban Accueil)', () => {
		const accueil: GroupShape[] = [
			{ title: 'Annuler', labels: ['Annuler', 'Rétablir'], iconOk: true },
			{ title: 'Presse-papiers', labels: ['Coller', 'Couper', 'Copier', 'Reproduire'], iconOk: true },
			{ title: 'Police', labels: [134, 'Gras', 'Italique', 'Souligné', null, null, 'Effacer'], iconOk: true },
			{ title: 'Bordures', labels: ['Toutes', 'Contour', 'Aucune'], iconOk: true },
			{ title: 'Alignement', labels: ['Haut', 'Milieu', 'Bas', 'Gauche', 'Centrer', 'Droite', 'Renvoyer à la ligne', 'Fusionner et centrer'], iconOk: true },
			{ title: 'Nombre', labels: ['0,00', 'Monétaire', 'Pourcentage', 'Date', 'Standard', 'Ajouter une décimale', 'Réduire les décimales'], iconOk: true },
			{ title: 'Styles', labels: ['Mise en forme conditionnelle ▾'] },
			{ title: 'Cellules', labels: ['Insérer ▾', 'Supprimer ▾', 'Format ▾'] },
			{ title: 'Édition', labels: ['Trier A → Z', 'Rechercher', 'Remplacer'] }
		];
		for (const width of [1388, 1268, 1180]) {
			const m = fit(accueil, width);
			expect(m.slice(6).every((x) => x === 'large' || x === 'small')).toBe(true);
			expect(accueil.reduce((s, g, i) => s + groupWidth(g, m[i]), 0)).toBeLessThanOrEqual(width);
		}
	});

	it('suit les styles du ruban (libellés mesurés, sans retour à la ligne)', () => {
		// Mesure fictive : 6 px par caractère, quelle que soit la taille.
		const m = (s: string) => s.length * 6;
		const g: GroupShape = { title: 'Édition', labels: ['Trier A → Z', 'Rechercher', 'Remplacer'] };
		// Grand : max(54, libellé + 14) par commande, écart de 2, marges du groupe 17.
		expect(groupWidth(g, 'large', m)).toBe(80 + 2 + 74 + 2 + 68 + 17);
		// Petit : une colonne de trois, icône 16 + écart 6 + libellé + 14.
		expect(groupWidth(g, 'small', m)).toBe(22 + 66 + 14 + 17);
		// Le titre, avec ses marges de 12 px, fixe la largeur minimale.
		expect(groupWidth({ title: 'Requêtes et connexions', labels: ['A'] }, 'icon', m)).toBe(22 * 6 + 24 + 17);
	});

	it('une liste de largeur fixe compte pour sa largeur', () => {
		const g: GroupShape = { title: 'Police', labels: [134, 'Gras'] };
		expect(groupWidth(g, 'icon')).toBe(134 + 2 + 28 + 2 + 17);
	});

	it('chaque mode est plus étroit que le précédent', () => {
		for (const g of groups) {
			expect(groupWidth(g, 'small')).toBeLessThanOrEqual(groupWidth(g, 'large'));
			expect(groupWidth(g, 'icon')).toBeLessThanOrEqual(groupWidth(g, 'small'));
		}
	});
});
