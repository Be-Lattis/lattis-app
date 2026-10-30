// Parcours au clavier seul (audit d'ergonomie du 30/10) : aucune souris, du
// chargement à la fin. Le focus part de la grille ; chaque boîte rend le
// focus à la grille quand on la ferme (Échap). Les vérifications passent par
// les rôles et les noms accessibles, comme un lecteur d'écran.
import type { Page } from '@playwright/test';
import { test, expect, activeCell, typeRows as type } from './fixtures';

test('K1. saisir et mettre en forme sans souris', async ({ app }) => {
	// Le focus est sur la grille dès le chargement.
	await expect(app.getByRole('grid')).toBeFocused();
	await expect(activeCell(app)).toContainText('A1,');
	await type(app, [
		['Produit', 'Prix'],
		['Pomme', '2'],
		['Poire', '3']
	]);
	await app.keyboard.press('Control+Home');
	await expect(activeCell(app)).toContainText('A1,');
	await app.keyboard.press('Control+b');
	await expect(app.getByRole('button', { name: /^Gras/ }).first()).toHaveAttribute('aria-pressed', 'true');
	await app.keyboard.press('Control+z');
	await expect(app.getByRole('button', { name: /^Gras/ }).first()).toHaveAttribute('aria-pressed', 'false');
	// Format de cellule (Ctrl+1) : une boîte nommée, Échap rend la grille.
	await app.keyboard.press('Control+1');
	await expect(app.getByRole('dialog').first()).toBeVisible();
	await app.keyboard.press('Escape');
	await expect(app.getByRole('grid')).toBeFocused();
});

test('K2. ruban par les touches accélératrices', async ({ app }) => {
	// Alt montre les lettres, A choisit l'onglet Données, Échap revient à la grille.
	await app.keyboard.press('Alt');
	await expect(app.locator('.keytips .keytip').first()).toBeVisible();
	await app.keyboard.press('a');
	await expect(app.getByRole('tab', { name: 'Données', exact: true })).toHaveAttribute('aria-selected', 'true');
	await app.keyboard.press('Escape');
	await app.keyboard.press('Escape');
	await expect(app.getByRole('grid')).toBeFocused();
});

test('K3. menu contextuel, rechercher et filtre au clavier', async ({ app }) => {
	await type(app, [
		['Produit', 'Prix'],
		['Pomme', '2'],
		['Kiwi', '5']
	]);
	await app.keyboard.press('Control+Home');
	// Maj+F10 : le menu contextuel de la cellule, parcouru aux flèches.
	await app.keyboard.press('Shift+F10');
	const menu = app.getByRole('menu').first();
	await expect(menu).toBeVisible();
	await app.keyboard.press('ArrowDown');
	await app.keyboard.press('Escape');
	await expect(menu).toBeHidden();
	await expect(app.getByRole('grid')).toBeFocused();
	// Ctrl+F : la recherche prend le focus, Entrée va au résultat, Échap rend la grille.
	await app.keyboard.press('Control+f');
	const find = app.getByRole('textbox', { name: /Rechercher/ }).first();
	await expect(find).toBeFocused();
	await app.keyboard.type('Kiwi');
	await app.keyboard.press('Enter');
	await expect(activeCell(app)).toContainText('Kiwi');
	await app.keyboard.press('Escape');
	await expect(app.getByRole('grid')).toBeFocused();
});

test('K4. enregistrer au clavier', async ({ app }) => {
	await type(app, [['Note'], ['1']]);
	const download = app.waitForEvent('download');
	await app.keyboard.press('Control+s');
	expect((await download).suggestedFilename()).toMatch(/\.lattis$/);
});

/** Alt puis les lettres des info-bulles de touches, chacune attendue avant d'être tapée. */
async function keytips(app: Page, ...keys: string[]) {
	await app.keyboard.press('Alt');
	for (const k of keys) {
		await expect(app.locator('.keytips .keytip').filter({ hasText: new RegExp(`^${k.toUpperCase()}$`) }).first()).toBeVisible();
		await app.keyboard.press(k);
	}
}

/** Tab jusqu'à l'élément dont le nom (aria-label ou texte) commence par name. */
async function tabTo(app: Page, name: string, max = 40) {
	for (let i = 0; i < max; i++) {
		const got = await app.evaluate(() => {
			const a = document.activeElement as HTMLElement | null;
			const input = a as HTMLInputElement | null;
			return (a?.getAttribute('aria-label') || input?.labels?.[0]?.textContent || a?.textContent || '').trim();
		});
		if (got.startsWith(name)) return;
		await app.keyboard.press('Tab');
	}
	throw new Error(`« ${name} » jamais atteint au clavier`);
}

test("K5. filtre d'une colonne par le ruban", async ({ app }) => {
	await type(app, [
		['Produit', 'Prix'],
		['Pomme', '2'],
		['Kiwi', '5']
	]);
	await app.keyboard.press('ArrowUp');
	await expect(activeCell(app)).toContainText('Kiwi');
	await app.keyboard.press('Control+Home');
	// Alt, A (Données), F (Filtrer) : le panneau de la colonne, une boîte nommée.
	await keytips(app, 'a', 'f');
	const panel = app.getByRole('dialog', { name: '« Produit »' });
	await expect(panel).toBeVisible();
	await tabTo(app, 'Kiwi');
	await app.keyboard.press('Space');
	await expect(panel.getByRole('checkbox', { name: 'Kiwi' })).not.toBeChecked();
	await app.keyboard.press('Escape');
	await expect(panel).toBeHidden();
	await expect(app.getByRole('grid')).toBeFocused();
});

test('K6. règles de mise en forme conditionnelle au clavier', async ({ app }) => {
	await type(app, [['Prix'], ['1'], ['3'], ['5']]);
	await app.keyboard.press('ArrowUp');
	await expect(activeCell(app)).toContainText('5');
	// Alt, H, L : le menu ; une règle « Supérieur à » 2.
	await keytips(app, 'h', 'l');
	// Le menu prend le focus ; Entrée ouvre le sous-menu et y met le focus.
	await expect(app.getByRole('menuitem', { name: /^Règles de mise en surbrillance/ })).toBeFocused();
	await app.keyboard.press('Enter');
	await expect(app.locator('.cf-sub > button', { hasText: 'Supérieur à…' })).toBeFocused();
	await app.keyboard.press('Enter');
	const quick = app.getByRole('dialog').first();
	await expect(quick).toBeVisible();
	await app.keyboard.type('2');
	await app.keyboard.press('Enter');
	await expect(quick).toBeHidden();
	// Gestionnaire : la liste des règles se parcourt aux flèches, Suppr supprime.
	await keytips(app, 'h', 'l');
	await expect(app.getByRole('menuitem', { name: /^Règles de mise en surbrillance/ })).toBeFocused();
	// ↑ depuis le premier élément : le dernier, « Gérer les règles… ».
	await app.keyboard.press('ArrowUp');
	await expect(app.getByRole('menuitem', { name: 'Gérer les règles…' })).toBeFocused();
	await app.keyboard.press('Enter');
	const list = app.getByRole('listbox', { name: /Règle/ });
	await expect(list.getByRole('option')).toHaveCount(1);
	await tabTo(app, 'Dupliquer la règle');
	await app.keyboard.press('Enter');
	await expect(list.getByRole('option')).toHaveCount(2);
	// Tab entre dans la liste sur la règle choisie (la copie, en tête), ↓ passe à la suivante.
	const first = list.getByRole('option').first();
	for (let i = 0; i < 20 && !(await first.evaluate((el) => el === document.activeElement)); i++) await app.keyboard.press('Tab');
	await expect(list.getByRole('option').first()).toBeFocused();
	await app.keyboard.press('ArrowDown');
	await expect(list.getByRole('option').nth(1)).toBeFocused();
	await expect(list.getByRole('option').nth(1)).toHaveAttribute('aria-selected', 'true');
	await app.keyboard.press('Delete');
	await expect(list.getByRole('option')).toHaveCount(1);
});
