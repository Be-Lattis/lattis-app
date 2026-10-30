// Essais de bout en bout sur la version web (frontend/e2e) :
//   npm run e2e       construit frontend/dist-web, compile le moteur web, lance les parcours
//   npm run e2e:run   parcours seuls (dist-web déjà construit)
// Un moteur neuf (cmd/lattis-engine) par parcours, profil isolé (e2e/fixtures.ts).
// Navigateur déjà installé : LATTIS_E2E_CHANNEL (msedge par défaut sous
// Windows, chrome ailleurs) ; « chromium » pour celui de Playwright
// (npx playwright install chromium). Go doit être dans le PATH, sauf si
// LATTIS_E2E_ENGINE désigne un moteur déjà compilé (go build -tags web).
import { defineConfig } from '@playwright/test';

const channel = process.env.LATTIS_E2E_CHANNEL || (process.platform === 'win32' ? 'msedge' : 'chrome');

export default defineConfig({
	testDir: './e2e',
	globalSetup: './e2e/global-setup.ts',
	timeout: 60_000,
	expect: { timeout: 10_000 },
	fullyParallel: true,
	workers: process.env.CI ? 2 : 4,
	retries: process.env.CI ? 1 : 0,
	reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
	use: {
		channel: channel === 'chromium' ? undefined : channel,
		viewport: { width: 1400, height: 900 },
		locale: 'fr-FR',
		acceptDownloads: true,
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	}
});
