import { describe, expect, it } from 'vitest';
import { BRIDGE_SCRIPT, PAGE_CSP, buildSrcdoc, eventOf, navigationGuard, pageUrl, parseCall, replyOf, urlSource } from './bridge';
import { hostOf } from './hosts';

describe('navigationGuard', () => {
	it('signale un chargement qui suit celui de la page', () => {
		const frame = new EventTarget();
		let escapes = 0;
		const guard = navigationGuard(frame, 'a', () => escapes++);
		frame.dispatchEvent(new Event('load')); // la page
		expect(escapes).toBe(0);
		frame.dispatchEvent(new Event('load')); // location.href = …
		expect(escapes).toBe(1);
		guard.update('b'); // nouveau contenu : un chargement attendu
		frame.dispatchEvent(new Event('load'));
		expect(escapes).toBe(1);
		guard.update('b');
		frame.dispatchEvent(new Event('load'));
		expect(escapes).toBe(2);
		guard.destroy();
		frame.dispatchEvent(new Event('load'));
		expect(escapes).toBe(2);
	});
});

describe('buildSrcdoc', () => {
	it('place la CSP et le pont après le doctype', () => {
		const doc = buildSrcdoc('<!DOCTYPE html>\n<html><body>x</body></html>');
		expect(doc.startsWith('<!DOCTYPE html><meta http-equiv="Content-Security-Policy"')).toBe(true);
		expect(doc).toContain(PAGE_CSP);
		expect(doc.indexOf('<script>')).toBeLessThan(doc.indexOf('<html>'));
		expect(doc.endsWith('\n<html><body>x</body></html>')).toBe(true);
	});
	it('ajoute un doctype à un fragment', () => {
		const doc = buildSrcdoc('<h1>Bonjour</h1>');
		expect(doc.startsWith('<!doctype html><meta')).toBe(true);
		expect(doc.endsWith('<h1>Bonjour</h1>')).toBe(true);
	});
	it('coupe le réseau direct', () => {
		expect(PAGE_CSP).toContain("default-src 'none'");
		expect(PAGE_CSP).not.toMatch(/connect-src|https?:/);
		expect(BRIDGE_SCRIPT).not.toContain('</script');
	});
});

describe('parseCall', () => {
	it('accepte un appel bien formé', () => {
		expect(parseCall({ lattis: 1, id: 3, name: 'db.query', args: { sql: 'SELECT 1' } })).toEqual({ id: 3, name: 'db.query', args: { sql: 'SELECT 1' } });
		expect(parseCall({ lattis: 1, id: 1, name: 'sheets.list' })).toEqual({ id: 1, name: 'sheets.list', args: {} });
	});
	it('refuse le reste', () => {
		for (const m of [null, 'x', { id: 1, name: 'a.b' }, { lattis: 1, id: '1', name: 'a.b' }, { lattis: 1, id: 1, name: 'rien' }, { lattis: 1, id: 1, name: 'a.b', args: [1] }, { lattis: 1, id: NaN, name: 'a.b' }])
			expect(parseCall(m)).toBeNull();
	});
	it('forme les réponses et les événements', () => {
		expect(replyOf(2, { ok: true, value: undefined })).toEqual({ lattis: 1, id: 2, ok: true, result: null });
		expect(replyOf(2, { ok: false, error: 'non' })).toEqual({ lattis: 1, id: 2, ok: false, error: 'non' });
		expect(eventOf('change')).toEqual({ lattis: 1, event: 'change', data: null });
	});
});

/** Exécute le pont dans un faux cadre : messages envoyés au parent, et réception. */
function frame() {
	const sent: unknown[] = [];
	const parent = { postMessage: (m: unknown) => sent.push(m) };
	let listener: ((e: { source: unknown; data: unknown }) => void) | undefined;
	const win: Record<string, unknown> = {};
	new Function('addEventListener', 'parent', 'window', 'console', BRIDGE_SCRIPT)(
		(_: string, f: typeof listener) => (listener = f),
		parent,
		win,
		{ error() {} }
	);
	const lattis = win.lattis as {
		call: (n: string, a?: object) => Promise<unknown>;
		query: (sql: string, params?: unknown[]) => Promise<Record<string, unknown>[]>;
		on: (ev: string, f: (d: unknown) => void) => () => void;
	};
	const receive = (data: unknown, source: unknown = parent) => listener?.({ source, data });
	return { sent, lattis, receive };
}

describe('pont dans la page', () => {
	it('appelle une commande et reçoit sa réponse', async () => {
		const f = frame();
		const p = f.lattis.call('sheets.list');
		expect(f.sent).toEqual([{ lattis: 1, id: 1, name: 'sheets.list', args: {} }]);
		f.receive({ lattis: 1, id: 1, ok: true, result: [{ id: 1, name: 'Feuille1' }] }, {}); // autre source : ignoré
		f.receive({ lattis: 1, id: 1, ok: true, result: [{ id: 1, name: 'Feuille1' }] });
		await expect(p).resolves.toEqual([{ id: 1, name: 'Feuille1' }]);
		const q = f.lattis.call('cells.set', { row: 1 });
		f.receive({ lattis: 1, id: 2, ok: false, error: 'feuille protégée' });
		await expect(q).rejects.toThrow('feuille protégée');
	});
	it('lattis.query donne des lignes { colonne = valeur }', async () => {
		const f = frame();
		const p = f.lattis.query('SELECT * FROM T WHERE x > ?', [1]);
		expect(f.sent[0]).toMatchObject({ name: 'db.query', args: { sql: 'SELECT * FROM T WHERE x > ?', params: [1] } });
		f.receive({ lattis: 1, id: 1, ok: true, result: { columns: ['a', 'b'], rows: [[1, 'x'], [2, 'y']] } });
		await expect(p).resolves.toEqual([
			{ a: 1, b: 'x' },
			{ a: 2, b: 'y' }
		]);
	});
	it('prévient des changements', () => {
		const f = frame();
		let n = 0;
		const off = f.lattis.on('change', () => n++);
		f.receive(eventOf('change'));
		off();
		f.receive(eventOf('change'));
		expect(n).toBe(1);
	});
});

describe('hostOf', () => {
	it('lit un hôte ou une adresse', () => {
		expect(hostOf(' API.Exemple.fr ')).toBe('api.exemple.fr');
		expect(hostOf('https://api.exemple.fr/v1/taux?x=1')).toBe('api.exemple.fr');
		expect(hostOf('http://127.0.0.1:8080/')).toBe('127.0.0.1:8080');
		expect(hostOf('localhost:3000/x')).toBe('localhost:3000');
	});
	it('refuse ce qui n’est pas un hôte', () => {
		for (const s of ['', 'ftp://exemple.fr', 'a b', '-x.fr', 'exemple.fr:port']) expect(hostOf(s)).toBeNull();
	});
});

describe('page web externe', () => {
	it('adresse gardée dans le contenu', () => {
		expect(urlSource('exemple.fr/a')).toBe('<!-- lattis:url https://exemple.fr/a -->');
		expect(pageUrl(urlSource('http://x.org'))).toBe('http://x.org');
		expect(pageUrl('<h1>page</h1>')).toBeNull();
		expect(pageUrl('<!-- lattis:url javascript:alert(1) -->')).toBeNull();
	});
});
