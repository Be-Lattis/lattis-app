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
	it('Styles, Cellules et Édition gardent leurs libellés tant que les autres groupes peuvent passer en icônes (ruban Accueil)', () => {
		const accueil: GroupShape[] = [
			{ title: 'Annuler', labels: ['Annuler', 'Rétablir'], sizes: ['icon', 'icon'], iconOk: true },
			{ title: 'Presse-papiers', labels: ['Coller', 'Couper', 'Copier', 'Reproduire'], sizes: ['big', 'small', 'small', 'small'], iconOk: true },
			{ title: 'Police', labels: [134, 'Gras', 'Italique', 'Souligné', null, null], iconOk: true, widest: 'icon' },
			{ title: 'Bordures', labels: ['Toutes', 'Contour', 'Aucune'], sizes: ['icon', 'icon', 'icon'], iconOk: true },
			{
				title: 'Alignement',
				labels: ['Haut', 'Milieu', 'Bas', 'Gauche', 'Centrer', 'Droite', 'Renvoyer à la ligne', 'Fusionner et centrer'],
				sizes: ['icon', 'icon', 'icon', 'icon', 'icon', 'icon', 'big', 'big'],
				iconOk: true
			},
			{ title: 'Nombre', labels: [], rows: { cols: 3, combo: 108 }, iconOk: true },
			{ title: 'Styles', labels: ['Mise en forme conditionnelle ▾', 'Mettre sous forme de tableau'] },
			{ title: 'Cellules', labels: ['Insérer ▾', 'Supprimer ▾', 'Format ▾'] },
			{
				title: 'Édition',
				labels: ['Somme automatique', 'Remplissage ▾', 'Effacer ▾', 'Trier et filtrer ▾', 'Rechercher', 'Remplacer'],
				sizes: ['small', 'small', 'small', 'big', 'small', 'small']
			}
		];
		// Tout dans sa forme d'Excel dès 2000 px.
		expect(fit(accueil, 2000).every((x, i) => x === (accueil[i].widest ?? 'large'))).toBe(true);
		for (const width of [1700, 1600, 1500]) {
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

	it('Alignement : petites icônes et deux grands boutons, tout en icônes quand la place manque (sizes)', () => {
		const align: GroupShape = {
			title: 'Alignement',
			labels: ['Haut', 'Milieu', 'Bas', 'Gauche', 'Centrer', 'Droite', 'Renvoyer à la ligne', 'Fusionner et centrer'],
			sizes: ['icon', 'icon', 'icon', 'icon', 'icon', 'icon', 'big', 'big'],
			iconOk: true
		};
		const gs = [...groups.slice(0, 2), align, groups[3]];
		expect(fit(gs, 10000)).toEqual(['large', 'large', 'large', 'large']);
		const large = groupWidth(align, 'large');
		expect(groupWidth(align, 'icon')).toBeLessThan(groupWidth(align, 'small'));
		expect(groupWidth(align, 'small')).toBeLessThan(large);
		// Fenêtre étroite : il se replie encore en menu.
		expect(fit(gs, 100)).toEqual(['collapsed', 'collapsed', 'collapsed', 'collapsed']);
	});

	it('groupe compact : petites à libellé en colonnes de trois, un grand bouton prend une colonne à lui', () => {
		const m = (s: string) => s.length * 6;
		// Presse-papiers : Coller en grand, une colonne Couper, Copier, Reproduire.
		const clip: GroupShape = { title: 'P', labels: ['Coller', 'Couper', 'Copier', 'Reproduire'], sizes: ['big', 'small', 'small', 'small'] };
		expect(groupWidth(clip, 'large', m)).toBe(54 + 2 + (22 + 60 + 14) + 17);
		// Petit : Coller rejoint la colonne, une seconde colonne à lui seul.
		expect(groupWidth(clip, 'small', m)).toBe(22 + 60 + 14 + 2 + (22 + 36 + 14) + 17);
		// Une colonne entamée s'arrête avant un grand bouton (Édition : trois petites, Trier, deux petites).
		const ed: GroupShape = { title: 'É', labels: ['A', 'B', 'C', 'Trier', 'D', 'E'], sizes: ['small', 'small', 'small', 'big', 'small', 'small'] };
		expect(groupWidth(ed, 'large', m)).toBe(2 * (22 + 6 + 14) + 54 + 2 * 2 + 17);
	});

	it('groupe en lignes (Nombre) : la liste ou les colonnes d’icônes, la même largeur dans tous les modes', () => {
		const nombre: GroupShape = { title: 'Nombre', labels: [], rows: { cols: 3, combo: 108 } };
		expect(groupWidth(nombre, 'large')).toBe(108 + 17);
		expect(groupWidth(nombre, 'icon')).toBe(108 + 17);
		expect(groupWidth({ ...nombre, rows: { cols: 4 } }, 'large')).toBe(4 * 32 - 2 + 17);
		expect(groupWidth(nombre, 'collapsed')).toBeLessThan(groupWidth(nombre, 'large'));
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
