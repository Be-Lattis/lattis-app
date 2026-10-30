// Dix parcours de l'utilisateur sur la version web (npm run e2e) : ils
// protègent le découpage de la grille et de la page
// (docs/audits/audit-architecture-2026-10-30.md). Un moteur neuf par parcours.
import { readFileSync } from 'node:fs';
import { test, expect, activeCell, goTo, typeRows, typeTable, ribbonTab } from './fixtures';

test.describe('démonstration', () => {
	test.use({ demo: 'med' });
	test('1. ouvrir la démonstration : document, onglets et données', async ({ app }) => {
		await expect(app.getByText('Bienvenue dans Lattis')).toBeVisible();
		await app.getByText('Ventes', { exact: true }).last().click();
		await expect(app.getByRole('grid')).toBeVisible();
		await app.getByRole('grid').click({ position: { x: 200, y: 60 } });
		await expect(activeCell(app)).not.toContainText('vide');
	});
});

test('2. saisir un tableau', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B3');
	await expect(activeCell(app)).toContainText('Prix');
	await expect(activeCell(app)).toContainText('3');
	await expect(app.getByRole('textbox', { name: 'Barre de formule' })).toHaveValue('3');
	// Le tableau est né : la zone de nom montre le tableau et la colonne.
	await expect(app.locator('input.table-name')).toHaveValue('Feuille1');
	// Barre d'état : somme de la sélection B2:B4.
	await goTo(app, 'B2');
	await app.keyboard.press('Shift+ArrowDown');
	await app.keyboard.press('Shift+ArrowDown');
	await expect(app.getByRole('contentinfo', { name: "Barre d'état" })).toContainText('10');
});

test('3. colonne calculée', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'C1');
	await typeRows(app, [['Double'], ['=[@Prix]*2']]);
	await goTo(app, 'C4');
	await expect(activeCell(app)).toContainText('10');
	await expect(app.getByRole('textbox', { name: 'Barre de formule' })).toHaveValue(/^=\[@?Prix\]\*2$/);
});

test('4. trier puis filtrer', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await ribbonTab(app, 'Données');
	await app.getByRole('button', { name: 'Trier Z → A', exact: true }).click();
	await goTo(app, 'A2');
	await expect(activeCell(app)).toContainText('Kiwi');
	// Filtre : décocher « Poire » dans le panneau ▾ de la colonne Produit.
	await app.getByRole('button', { name: 'Filtrer', exact: true }).click();
	await app.getByRole('checkbox', { name: 'Poire' }).uncheck();
	await app.getByRole('button', { name: 'OK', exact: true }).click();
	await expect(app.getByRole('button', { name: 'Effacer', exact: true })).toBeEnabled();
	await goTo(app, 'A2');
	await app.keyboard.press('ArrowDown');
	await expect(activeCell(app)).toContainText('Pomme');
	// Effacer les filtres : Poire revient.
	await app.getByRole('button', { name: 'Effacer', exact: true }).click();
	await expect(app.getByRole('button', { name: 'Effacer', exact: true })).toBeDisabled();
	await goTo(app, 'A2');
	await app.keyboard.press('ArrowDown');
	await expect(activeCell(app)).toContainText('Poire');
});

test('5. tableau croisé dynamique', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await ribbonTab(app, 'Insertion');
	await app.getByRole('button', { name: 'Tableau croisé dynamique', exact: true }).click();
	await app.getByRole('button', { name: 'Nouvelle feuille', exact: true }).click();
	const panel = app.getByRole('complementary', { name: 'Champs de tableau croisé dynamique' });
	await expect(panel).toBeVisible();
	await expect(app.getByRole('grid', { name: 'Feuille TCD 1' })).toBeVisible();
	await panel.getByRole('checkbox', { name: 'Produit' }).check();
	await expect(panel.getByRole('region', { name: 'Lignes' }).getByRole('button', { name: 'Produit' })).toBeVisible();
	// Le résultat se lit en SQL comme un tableau.
	await app.getByRole('button', { name: 'SQL', exact: true }).click();
	await expect(app.getByRole('button', { name: /TCD1/ })).toBeVisible();
});

test('6. graphique', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await ribbonTab(app, 'Insertion');
	await app.getByRole('button', { name: 'Histogramme', exact: true }).click();
	const chart = app.getByRole('group', { name: 'Graphique « Prix »' });
	await expect(chart).toBeVisible();
	await expect(chart.getByRole('row', { name: 'Kiwi 5' })).toBeAttached();
	await expect(app.getByRole('button', { name: /Annuler : Insérer un graphique/ })).toBeVisible();
});

test('7. requête SQL', async ({ app }) => {
	await typeTable(app);
	await app.getByRole('button', { name: 'SQL', exact: true }).click();
	await app.locator('.cm-content').click();
	await app.keyboard.press('Control+A');
	await app.keyboard.type('SELECT SUM(Prix) AS total FROM Feuille1');
	await app.getByRole('button', { name: 'Exécuter' }).click();
	await expect(app.getByText('1 ligne', { exact: false })).toBeVisible();
	await expect(app.getByRole('row', { name: '10' })).toBeVisible();
});

test('8. annuler et rétablir', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await app.keyboard.type('7');
	await app.keyboard.press('Enter');
	await goTo(app, 'B2');
	await expect(activeCell(app)).toContainText('7');
	await app.keyboard.press('Control+Z');
	await expect(activeCell(app)).toContainText(', 2');
	await app.keyboard.press('Control+Y');
	await expect(activeCell(app)).toContainText(', 7');
	// Le bouton de la barre de titre annule aussi.
	await app.getByRole('button', { name: /^Annuler : / }).first().click();
	await expect(activeCell(app)).toContainText(', 2');
});

test('9. enregistrer : le classeur se télécharge', async ({ app }) => {
	await typeTable(app);
	const download = app.waitForEvent('download');
	await app.keyboard.press('Control+S');
	const file = await download;
	expect(file.suggestedFilename()).toMatch(/\.lattis$/);
	const bytes = readFileSync(await file.path());
	// Un .lattis est une archive zip (internal/archive).
	expect(bytes.subarray(0, 2).toString()).toBe('PK');
	expect(bytes.length).toBeGreaterThan(1000);
});

test('10. paramètres : une fonctionnalité activée reste activée', async ({ app }) => {
	await ribbonTab(app, 'Insertion');
	await expect(app.getByRole('button', { name: /Sparkline/i })).toHaveCount(0);
	await app.getByRole('button', { name: 'Fichier', exact: true }).click();
	await app.getByText('Paramètres…').click();
	const dialog = app.getByRole('dialog', { name: 'Paramètres' });
	await dialog.getByRole('checkbox', { name: 'Graphiques sparkline' }).check();
	await dialog.getByRole('button', { name: 'Fermer' }).click();
	await ribbonTab(app, 'Insertion');
	await expect(app.getByRole('button', { name: /Sparkline/i }).first()).toBeVisible();
	await app.reload();
	await ribbonTab(app, 'Insertion');
	await expect(app.getByRole('button', { name: /Sparkline/i }).first()).toBeVisible();
});

// Surfaces posées sur la grille (menus, rechercher, format) : les premières
// sorties de Sheet.svelte.
test('11. menu contextuel, rechercher, format de cellule', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await app.getByRole('grid').click({ button: 'right', position: { x: 230, y: 56 } });
	const menu = app.getByRole('menu').first();
	await expect(menu).toBeVisible();
	await app.keyboard.press('Escape');
	await expect(menu).toBeHidden();
	await app.keyboard.press('Control+f');
	const find = app.getByRole('textbox', { name: /Rechercher/ }).first();
	await expect(find).toBeVisible();
	await find.fill('Kiwi');
	await find.press('Enter');
	await expect(activeCell(app)).toContainText('Kiwi');
	await app.keyboard.press('Escape');
	await expect(find).toBeHidden();
	// Par le ruban : Remplacer ouvre la barre avec le second champ.
	await ribbonTab(app, 'Accueil');
	await app.getByRole('button', { name: 'Remplacer', exact: true }).click();
	await expect(app.getByRole('textbox', { name: /Remplacer/ }).first()).toBeVisible();
	await app.keyboard.press('Escape');
	await goTo(app, 'B2');
	await app.keyboard.press('Control+1');
	const fmt = app.getByRole('dialog').first();
	await expect(fmt).toBeVisible();
	await app.keyboard.press('Escape');
	await expect(fmt).toBeHidden();
});

// Défilement (molette, clavier) et poignée de recopie : la souris et le clavier de la grille.
test('12. défilement, clavier et recopie', async ({ app }) => {
	const grid = app.getByRole('grid');
	await grid.click({ position: { x: 100, y: 40 } });
	await expect(activeCell(app)).toContainText('A1,');
	await app.mouse.wheel(0, 3000);
	await grid.click({ position: { x: 100, y: 40 } });
	await expect(activeCell(app)).not.toContainText('A1,');
	await app.keyboard.press('Control+Home');
	await expect(activeCell(app)).toContainText('A1,');
	await app.keyboard.press('PageDown');
	await expect(activeCell(app)).not.toContainText('A1,');
	// Recopie : 1 et 2 dans A1:A2, Ctrl+D sur A2:A3 recopie 2 vers A3.
	await goTo(app, 'A1');
	await typeRows(app, [['1'], ['2']]);
	await goTo(app, 'A2');
	await app.keyboard.press('Shift+ArrowDown');
	await app.keyboard.press('Control+d');
	await goTo(app, 'A3');
	await expect(activeCell(app)).toContainText(', 2');
});

// Notes (Révision) : Maj+F2 ouvre la note, le ruban la modifie et la supprime.
test('13. note sur une cellule', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'B2');
	await app.keyboard.press('Shift+F2');
	const note = app.getByRole('textbox', { name: 'Note de la cellule' });
	await expect(note).toBeVisible();
	await note.fill('À vérifier');
	await note.press('Control+Enter');
	await expect(note).toBeHidden();
	await ribbonTab(app, 'Révision');
	await expect(app.getByRole('button', { name: 'Modifier la note' })).toBeVisible();
	await app.getByRole('button', { name: 'Supprimer la note' }).click();
	await expect(app.getByRole('button', { name: 'Nouvelle note' })).toBeVisible();
});

// Validation des données : liste de valeurs, saisie hors liste refusée.
test('14. validation des données', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'A2');
	await ribbonTab(app, 'Données');
	await app.getByRole('button', { name: 'Validation des données', exact: true }).click();
	const dlg = app.getByRole('dialog', { name: /Validation/ });
	await expect(dlg).toBeVisible();
	await dlg.locator('#dv-type').selectOption('list');
	await dlg.locator('textarea').fill('Pomme\nPoire\nKiwi');
	await dlg.getByRole('button', { name: 'OK', exact: true }).click();
	await expect(dlg).toBeHidden();
	await goTo(app, 'A2');
	await app.keyboard.type('Banane');
	await app.keyboard.press('Enter');
	await expect(app.getByText(/n'est pas permis/).first()).toBeVisible();
});

// Lignes masquées (Accueil › Format) : la flèche les saute ; Annuler les rend.
test('15. masquer des lignes', async ({ app }) => {
	await typeTable(app);
	await goTo(app, 'A3');
	await ribbonTab(app, 'Accueil');
	await app.getByRole('button', { name: 'Format ▾', exact: true }).click();
	await app.getByText('Masquer les lignes', { exact: true }).click();
	await goTo(app, 'A2');
	await app.keyboard.press('ArrowDown');
	await expect(activeCell(app)).toContainText('Kiwi');
	await app.keyboard.press('Control+Z');
	await goTo(app, 'A2');
	await app.keyboard.press('ArrowDown');
	await expect(activeCell(app)).toContainText('Poire');
});
