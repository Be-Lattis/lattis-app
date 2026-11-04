// Stabilité de la version web derrière une liaison lente (audit du 04/11/2026,
// docs/audits/audit-stabilite-web-2026-11-04.md) : 150 ms dans chaque sens
// plus un aléa, coupures en pleine saisie. Ce qui est tapé doit arriver, dans
// l'ordre, et rester après une reconnexion.
import type { Page } from '@playwright/test';
import { test, expect, activeCell, goTo, typeRows } from './fixtures';

test.use({ latency: Number(process.env.LATTIS_E2E_LATENCY ?? 150) });
test.describe.configure({ timeout: 180_000 });

async function expectCell(page: Page, ref: string, text: string) {
	await goTo(page, ref);
	// Large : derrière la liaison lente, la file d'écriture se vide à son rythme.
	await expect(activeCell(page)).toContainText(text, { timeout: 40_000 });
}

const rows = (n: number) => [['Code', 'Nom', 'Qté'], ...Array.from({ length: n }, (_, i) => [`C${i + 1}`, `Article ${i + 1}`, String((i + 1) * 3)])];

test('saisie rapide derrière une liaison lente : tout arrive, dans l’ordre', async ({ app }) => {
	await app.getByRole('grid').click({ position: { x: 100, y: 40 } });
	await expect(activeCell(app)).toContainText('A1,');
	await typeRows(app, rows(12));
	await expectCell(app, 'C13', 'Qté, 36');
	await expectCell(app, 'A2', 'C1');
	await expectCell(app, 'B7', 'Article 6');
	// Même cellule retapée vite : la dernière saisie l'emporte.
	await goTo(app, 'C2');
	for (const v of ['10', '20', '30', '40', '50']) {
		await app.keyboard.type(v);
		await app.keyboard.press('Enter');
		await app.keyboard.press('ArrowUp');
	}
	await app.keyboard.press('ArrowDown');
	await expectCell(app, 'C2', 'Qté, 50');
	// Relu après une reconnexion (la page relit tout le classeur).
	await app.evaluate(() => (window as unknown as { __lattisNet: { cut(): void } }).__lattisNet.cut());
	await expectCell(app, 'C2', 'Qté, 50');
	await expectCell(app, 'C13', 'Qté, 36');
});

test('annuler vite plusieurs fois derrière une liaison lente', async ({ app }) => {
	await app.getByRole('grid').click({ position: { x: 100, y: 40 } });
	await typeRows(app, rows(3));
	await expectCell(app, 'C4', 'Qté, 9');
	await goTo(app, 'C2');
	for (const v of ['100', '200', '300']) {
		await app.keyboard.type(v);
		await app.keyboard.press('Enter');
	}
	await app.keyboard.press('Control+Z');
	await app.keyboard.press('Control+Z');
	await app.keyboard.press('Control+Z');
	await expectCell(app, 'C2', 'Qté, 3');
	await expectCell(app, 'C3', 'Qté, 6');
	await expectCell(app, 'C4', 'Qté, 9');
	await app.keyboard.press('Control+Y');
	await expectCell(app, 'C2', 'Qté, 100');
});

test('coupure en pleine saisie : rien de ce qui est tapé ne se perd en silence', async ({ app }) => {
	await app.getByRole('grid').click({ position: { x: 100, y: 40 } });
	await typeRows(app, rows(2));
	await expectCell(app, 'C3', 'Qté, 6');
	await goTo(app, 'A4');
	const net = () => app.evaluate(() => (window as unknown as { __lattisNet: { cut(): void } }).__lattisNet.cut());
	// Coupure juste après des saisies encore en route, puis saisie pendant la
	// coupure (file de la page, envoyée à la reconnexion).
	await typeRows(app, [['C3', 'Article 3', '9']]);
	await net();
	await typeRows(app, [['C4', 'Article 4', '12']]);
	// Ligne en route pendant la coupure : refaite à la reconnexion ; ligne
	// tapée pendant la coupure : envoyée à la reconnexion.
	await expectCell(app, 'C5', 'Qté, 12');
	await expectCell(app, 'A5', 'C4');
	await expectCell(app, 'C4', 'Qté, 9');
	await expectCell(app, 'B4', 'Article 3');
});
