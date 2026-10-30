// Préparation des essais de bout en bout : le frontend web doit être construit
// (npm run build:web) et le moteur web compilé une fois (go build -tags web),
// dans un dossier temporaire. LATTIS_E2E_ENGINE désigne un binaire déjà
// compilé (CI) et saute la compilation.
import { execFileSync } from 'node:child_process';
import { existsSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

export default function globalSetup() {
	const frontend = resolve(import.meta.dirname, '..');
	if (!existsSync(join(frontend, 'dist-web', 'index.html'))) {
		throw new Error('frontend/dist-web absent : lancez « npm run build:web » (ou « npm run e2e »)');
	}
	if (process.env.LATTIS_E2E_ENGINE) return;
	const bin = join(mkdtempSync(join(tmpdir(), 'lattis-e2e-bin-')), process.platform === 'win32' ? 'lattis-engine.exe' : 'lattis-engine');
	execFileSync('go', ['build', '-tags', 'web', '-o', bin, './cmd/lattis-engine'], { cwd: resolve(frontend, '..'), stdio: 'inherit' });
	process.env.LATTIS_E2E_ENGINE = bin;
}
