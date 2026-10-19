import { describe, expect, it } from 'vitest';
import { allLabel, choiceHiddenCols, choiceLabel, choiceStatus, choiceStep, newLabel, nextChoice } from './choice';

describe('onglet à choix : libellés', () => {
	it("l'onglet montre la valeur choisie, ou la catégorie sur (Toutes)", () => {
		expect(choiceLabel('Ventes', { name: 'Année', value: '2025', all: false })).toBe('Ventes · 2025');
		expect(choiceLabel('Ventes', { name: 'Année', value: '', all: true })).toBe('Ventes · Année');
	});
	it("l'onglet suit le renommage de la colonne de catégorie", () => {
		// Le nom vient de la colonne source (canvas.SheetChoice.name).
		expect(choiceLabel('Ventes', { name: 'Exercice', value: '', all: true })).toBe('Ventes · Exercice');
		expect(allLabel('Exercice')).toBe('(Tous les exercices)');
	});
	it('entrées de la liste selon la catégorie', () => {
		expect(allLabel('Année')).toBe('(Toutes les années)');
		expect(allLabel('Mois')).toBe('(Tous les mois)');
		expect(allLabel('Rayon')).toBe('(Toutes les valeurs de « Rayon »)');
		expect(newLabel('Année')).toBe('Nouvelle année…');
		expect(newLabel('mois')).toBe('Nouveau mois…');
		expect(newLabel('Rayon')).toBe('Nouvelle valeur…');
	});
	it('rappel pour la barre d’état', () => {
		expect(choiceStatus({ name: 'Année', value: '2025', all: false })).toBe("Année : 2025 (choix de l'onglet)");
		expect(choiceStatus({ name: 'Année', value: '2025', all: false }, 12)).toBe("Année : 2025 (choix de l'onglet) · 12 lignes d'autres valeurs masquées");
		expect(choiceStatus({ name: 'Mois', value: '', all: true })).toBe("Mois : (Tous les mois) (choix de l'onglet)");
	});
});

describe('Nouvelle année… : valeur suivante', () => {
	it('suit la série', () => {
		expect(nextChoice(['2023', '2024', '2025'])).toBe('2026');
		expect(nextChoice(['2025'])).toBe('2026');
		expect(nextChoice(['Octobre', 'Novembre'])).toBe('Décembre');
		expect(nextChoice(['Lot 1', 'Lot 2', 'Lot 3'])).toBe('Lot 4');
		expect(nextChoice(['01/11/2025', '01/12/2025'])).toBe('01/01/2026');
	});
	it('rien quand elle ne se devine pas', () => {
		expect(nextChoice([])).toBe('');
		expect(nextChoice(['Paris', 'Lyon'])).toBe('');
		expect(nextChoice(['Paris'])).toBe('');
	});
});

describe('valeur voisine (Ctrl+Alt+Pg.Suiv / Pg.Préc)', () => {
	const vs = ['2023', '2024', '2025'];
	it('passe à la suivante ou à la précédente', () => {
		expect(choiceStep(vs, { name: 'Année', value: '2024', all: false }, 1)).toBe('2025');
		expect(choiceStep(vs, { name: 'Année', value: '2024', all: false }, -1)).toBe('2023');
	});
	it('rien au bout ; depuis (Toutes), la première ou la dernière', () => {
		expect(choiceStep(vs, { name: 'Année', value: '2025', all: false }, 1)).toBeNull();
		expect(choiceStep(vs, { name: 'Année', value: '', all: true }, 1)).toBe('2023');
		expect(choiceStep(vs, { name: 'Année', value: '', all: true }, -1)).toBe('2025');
		expect(choiceStep([], { name: 'Année', value: '', all: true }, 1)).toBeNull();
	});
});

describe('colonne de catégorie masquée', () => {
	const ventes = { row: 0, col: 0, rows: 3, columns: [1, 2, 3], follows: true, choiceCol: 0 };
	const depenses = { row: 10, col: 0, rows: 2, columns: [1, 2], follows: true, choiceCol: 0 };
	it('masquée quand une seule valeur est choisie', () => {
		expect([...choiceHiddenCols({ name: 'Année', value: '2025', all: false }, [ventes, depenses])]).toEqual([0]);
	});
	it('affichée sur (Toutes) ou sans choix', () => {
		expect(choiceHiddenCols({ name: 'Année', value: '', all: true }, [ventes]).size).toBe(0);
		expect(choiceHiddenCols(null, [ventes]).size).toBe(0);
	});
	it("pas quand la colonne de la feuille sert à autre chose", () => {
		const tarifs = { row: 20, col: 0, rows: 2, columns: [1, 2], follows: false };
		const c = { name: 'Année', value: '2025', all: false };
		expect(choiceHiddenCols(c, [ventes, tarifs]).size).toBe(0);
		expect(choiceHiddenCols(c, [ventes], [{ col: 0 }]).size).toBe(0);
		// Catégorie en deuxième colonne d'un tableau, première d'un autre : chacune occupe l'autre.
		expect(choiceHiddenCols(c, [{ ...ventes, choiceCol: 1 }, depenses]).size).toBe(0);
	});
});
