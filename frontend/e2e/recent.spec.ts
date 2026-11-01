// La croix d'un récent le retire de la liste (rien d'autre n'est touché).
import { test, expect } from './fixtures';

test.use({ recent: [{ server: 'https://srv.test', workbookId: 'id-1', title: 'Suivi' }] });

test('récents : la croix retire l\'entrée', async ({ app }) => {
	await app.getByRole('button', { name: 'Fichier', exact: true }).click();
	const sub = app.getByRole('menuitem', { name: 'Ouvrir un fichier récent' });
	await expect(sub).toBeEnabled();
	await sub.hover();
	const row = app.locator('.recent-row');
	await expect(row).toContainText('Suivi');
	await row.hover();
	if (process.env.ZZ_SHOT) await app.screenshot({ path: process.env.ZZ_SHOT });
	await row.getByRole('button', { name: 'Retirer de la liste' }).click();
	await expect(sub).toBeDisabled();
});
