// Un moteur web neuf par essai (cmd/lattis-engine, compilé par global-setup) :
// session, téléchargements et profil (réglages) dans un dossier temporaire,
// effacé à la fin. Les essais ne se voient pas et peuvent tourner en parallèle.
//
// La grille est un canvas : les vérifications passent par la zone de nom, la
// barre de formule et la cellule cachée de GridA11y.svelte (« B2, Prix, 3 »),
// qui décrit la cellule active.
import { test as base, expect, type Page } from '@playwright/test';
import { spawn, type ChildProcess } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

type Options = { demo: string; recent: object[]; latency: number };

export const test = base.extend<Options & { app: Page }>({
	demo: ['', { option: true }],
	recent: [[], { option: true }],
	// Latence ajoutée à la liaison WebSocket, dans chaque sens (ms) : voir slowSocket.
	latency: [0, { option: true }],
	app: async ({ page, demo, recent, latency }, use, testInfo) => {
		if (latency) await page.addInitScript(slowSocket, latency);
		const dir = mkdtempSync(join(tmpdir(), 'lattis-e2e-'));
		const cfg = join(dir, 'config');
		const cache = join(dir, 'cache');
		mkdirSync(cfg);
		mkdirSync(cache);
		if (recent.length) {
			mkdirSync(join(cache, 'Lattis'));
			writeFileSync(join(cache, 'Lattis', 'recent.json'), JSON.stringify(recent));
		}
		const args = ['--addr', '127.0.0.1:0', '--static', resolve(import.meta.dirname, '..', 'dist-web'), '--dir', join(dir, 'session')];
		if (demo) args.push('--demo', demo);
		const proc = spawn(process.env.LATTIS_E2E_ENGINE!, args, {
			env: {
				...process.env,
				// Profil isolé : config.json, récents et cache dans le dossier de l'essai.
				APPDATA: cfg,
				LOCALAPPDATA: cache,
				XDG_CONFIG_HOME: cfg,
				XDG_CACHE_HOME: cache,
				HOME: dir,
				LATTIS_SYSTEM_LOCALE: 'fr-FR'
			},
			stdio: ['ignore', 'pipe', 'pipe']
		});
		// Journal du moteur et console de la page : joints au rapport d'un essai en échec.
		let log = '';
		proc.stdout!.on('data', (b: Buffer) => (log += b.toString()));
		proc.stderr!.on('data', (b: Buffer) => (log += b.toString()));
		page.on('console', (m) => m.type() === 'error' && (log += `[console] ${m.text()}\n`));
		page.on('pageerror', (e) => (log += `[page] ${e.message}\n`));
		const url = await engineURL(proc);
		await page.goto(url);
		await expect(page.getByRole('button', { name: 'Fichier', exact: true })).toBeVisible({ timeout: 30_000 });
		if (!demo) await expect(page.getByRole('grid')).toBeVisible();
		await use(page);
		if (testInfo.status !== testInfo.expectedStatus) await testInfo.attach('moteur', { body: log, contentType: 'text/plain' });
		proc.kill();
		await new Promise((r) => (proc.exitCode !== null ? r(null) : proc.once('exit', r)));
		rmSync(dir, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
	}
});

export { expect };

// engineURL attend la ligne « Lattis (web) : http://127.0.0.1:port/ ».
function engineURL(proc: ChildProcess): Promise<string> {
	return new Promise((ok, ko) => {
		let log = '';
		const timer = setTimeout(() => ko(new Error('moteur web muet après 60 s :\n' + log)), 60_000);
		const read = (b: Buffer) => {
			log += b.toString();
			const m = /Lattis \(web\) : (http:\/\/\S+)/.exec(log);
			if (m) {
				clearTimeout(timer);
				ok(m[1]);
			}
		};
		proc.stdout!.on('data', read);
		proc.stderr!.on('data', read);
		proc.once('exit', (code) => ko(new Error(`moteur web arrêté (${code}) :\n${log}`)));
	});
}

// slowSocket remplace WebSocket dans la page par une liaison lente : chaque
// message (dans les deux sens) arrive après latency ms plus un aléa, dans
// l'ordre (comme TCP). window.__lattisNet règle la latence en cours de route
// et coupe la liaison (cut : le moteur et la page voient une coupure ; les
// messages en route sont perdus).
function slowSocket(latency: number) {
	type Net = { up: number; down: number; jitter: number; sockets: WebSocket[]; sent: string[]; log: string[]; cut: () => void };
	const net: Net = {
		up: latency,
		down: latency,
		jitter: latency / 2,
		sockets: [],
		sent: [],
		log: [],
		cut() {
			for (const s of net.sockets.splice(0)) s.close();
		}
	};
	(window as unknown as { __lattisNet: Net }).__lattisNet = net;
	const Real = window.WebSocket;
	// File ordonnée par sens : un message ne double jamais le précédent.
	const lane = () => {
		let last = 0;
		return (dir: 'up' | 'down', fn: () => void) => {
			const now = performance.now();
			last = Math.max(last, now + net[dir] + Math.random() * net.jitter);
			setTimeout(fn, last - now);
		};
	};
	class Slow {
		static CONNECTING = 0;
		static OPEN = 1;
		static CLOSING = 2;
		static CLOSED = 3;
		onopen: ((e: Event) => void) | null = null;
		onmessage: ((e: MessageEvent) => void) | null = null;
		onclose: ((e: CloseEvent) => void) | null = null;
		onerror: ((e: Event) => void) | null = null;
		_ws: WebSocket;
		_up = lane();
		_down = lane();
		_gone = false;
		constructor(url: string | URL, protocols?: string | string[]) {
			const ws = new Real(url, protocols);
			this._ws = ws;
			net.sockets.push(ws);
			ws.onopen = (e) => this._down('down', () => !this._gone && this.onopen?.(e));
			ws.onmessage = (e) =>
				this._down('down', () => {
					net.log.push(`${Math.round(performance.now())} < ${String(e.data).slice(0, 120)}`);
					if (!this._gone) this.onmessage?.(e);
				});
			ws.onerror = (e) => this.onerror?.(e);
			ws.onclose = (e) => {
				// La coupure se voit aussitôt ; ce qui était en route est perdu.
				this._gone = true;
				this.onclose?.(e);
			};
		}
		get readyState() {
			return this._gone ? 3 : this._ws.readyState;
		}
		send(m: string) {
			net.sent.push(m);
			net.log.push(`${Math.round(performance.now())} > ${m.slice(0, 120)}`);
			this._up('up', () => this._ws.readyState === 1 && this._ws.send(m));
		}
		close(code?: number, reason?: string) {
			this._ws.close(code, reason);
		}
	}
	(window as unknown as { WebSocket: unknown }).WebSocket = Slow;
}

// --- Aides --------------------------------------------------------------------

/** Texte de la cellule cachée qui décrit la cellule active. */
export function activeCell(page: Page) {
	return page.locator('[role=gridcell]');
}

/** Va à une cellule par la zone de nom (« B2 »). */
export async function goTo(page: Page, ref: string) {
	// La zone de nom change de forme quand la cellule active entre dans un
	// tableau (tableau / colonne) : on réessaie si elle a été remplacée entre-temps.
	await expect(async () => {
		const box = page.locator('input.name-box, .name-box input.table-name').first();
		await box.click({ timeout: 2000 });
		await box.fill(ref, { timeout: 2000 });
		await box.press('Enter', { timeout: 2000 });
		await expect(activeCell(page)).toContainText(ref + ',', { timeout: 2000 });
	}).toPass({ timeout: 15_000 });
}

/** Saisit des lignes à partir de la cellule active : Tab entre les cellules, Entrée en fin de ligne. */
export async function typeRows(page: Page, rows: string[][]) {
	for (const row of rows) {
		for (let i = 0; i < row.length; i++) {
			await page.keyboard.type(row[i]);
			await page.keyboard.press(i < row.length - 1 ? 'Tab' : 'Enter');
		}
	}
}

/** Feuille vide → tableau Produit / Prix de trois lignes, à partir de A1. */
export async function typeTable(page: Page) {
	await page.getByRole('grid').click({ position: { x: 100, y: 40 } });
	await expect(activeCell(page)).toContainText('A1,');
	await typeRows(page, [
		['Produit', 'Prix'],
		['Pomme', '2'],
		['Poire', '3'],
		['Kiwi', '5']
	]);
	// La dernière saisie est enregistrée par le moteur avant la suite.
	await goTo(page, 'B4');
	await expect(activeCell(page)).toContainText('Prix, 5');
}

/** Ouvre un onglet du ruban (Accueil, Données…). */
export async function ribbonTab(page: Page, name: string) {
	await page.getByRole('tab', { name, exact: true }).click();
}
