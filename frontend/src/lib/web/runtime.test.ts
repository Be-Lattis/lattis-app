import { get } from 'svelte/store';
import { busy, DELAY } from '#lib/shell/busy.js';
import { describe, expect, it, vi } from 'vitest';
import { Call, Events, RpcClient, setClient, type SocketLike } from './runtime';
import { keyLabel } from './platform';
import { shortcutGroups, webGroups } from '#lib/shell/shortcuts.js';

/** WebSocket de test : garde les messages envoyés, simule le moteur. */
class FakeSocket implements SocketLike {
	readyState = 0;
	sent: Record<string, unknown>[] = [];
	onopen: ((ev: unknown) => void) | null = null;
	onclose: ((ev: unknown) => void) | null = null;
	onmessage: ((ev: { data: unknown }) => void) | null = null;
	onerror: ((ev: unknown) => void) | null = null;
	send(data: string) {
		this.sent.push(JSON.parse(data));
	}
	close() {
		this.readyState = 3;
		this.onclose?.({});
	}
	open() {
		this.readyState = 1;
		this.onopen?.({});
	}
	push(msg: object) {
		this.onmessage?.({ data: JSON.stringify({ jsonrpc: '2.0', ...msg }) });
	}
}

function setup() {
	const sockets: FakeSocket[] = [];
	const c = new RpcClient('ws://test/ws', () => {
		const s = new FakeSocket();
		sockets.push(s);
		return s;
	});
	setClient(c);
	return { c, sockets };
}

describe('transport web (JSON-RPC sur WebSocket)', () => {
	it('envoie Call.ByID au moteur, après ouverture, et rend le résultat', async () => {
		const { sockets } = setup();
		const p = Call.ByID(2316013313, 'a', 2);
		const ws = sockets[0];
		expect(ws.sent).toEqual([]); // en file tant que la connexion n'est pas ouverte
		ws.open();
		expect(ws.sent).toEqual([{ jsonrpc: '2.0', id: 1, method: '2316013313', params: ['a', 2] }]);
		ws.push({ id: 1, result: { open: true } });
		await expect(p).resolves.toEqual({ open: true });
	});

	it('compte les appels pour la barre d’état, jusqu’à leur fin', async () => {
		vi.useFakeTimers();
		try {
			const { sockets } = setup();
			const p = Call.ByName('DocumentService.ShareOpen');
			sockets[0].open();
			expect(get(busy)).toBe(false);
			await vi.advanceTimersByTimeAsync(DELAY + 50);
			vi.setSystemTime(Date.now());
			expect(get(busy)).toBe(true);
			sockets[0].push({ id: 1, result: null });
			await p;
			expect(get(busy)).toBe(false);
		} finally {
			vi.useRealTimers();
		}
	});

	it('dit aussi une première connexion manquée, puis son retour', async () => {
		vi.useFakeTimers();
		try {
			const { sockets } = setup();
			const seen: string[] = [];
			const offs = ['host:disconnected', 'host:reconnected', 'doc:changed'].map((n) => Events.On(n, () => seen.push(n)));
			Call.ByName('X').catch(() => {});
			sockets[0].close();
			expect(seen).toEqual(['host:disconnected']);
			await vi.advanceTimersByTimeAsync(10_000);
			sockets[1].open();
			expect(seen).toEqual(['host:disconnected', 'host:reconnected']);
			offs.forEach((off) => off());
		} finally {
			vi.useRealTimers();
		}
	});

	it('rejette avec le genre d’erreur du moteur', async () => {
		const { sockets } = setup();
		const p = Call.ByName('DocumentService.AddDoc');
		sockets[0].open();
		sockets[0].push({ id: 1, error: { code: -32602, message: 'AddDoc attend 1 paramètre(s), reçu 0', data: { kind: 'TypeError' } } });
		await expect(p).rejects.toThrow(TypeError);
		const q = Call.ByName('X');
		sockets[0].push({ id: 2, error: { code: -32000, message: 'aucun classeur ouvert', data: { kind: 'RuntimeError' } } });
		await expect(q).rejects.toThrow('aucun classeur ouvert');
	});

	it('distribue les évènements poussés à Events.On, Once une seule fois', () => {
		const { sockets } = setup();
		Call.ByID(1);
		sockets[0].open();
		const got: unknown[] = [];
		const once: unknown[] = [];
		const off = Events.On('doc:changed', (e) => got.push(e.data));
		Events.Once('doc:changed', (e) => once.push(e.data));
		sockets[0].push({ method: 'event', params: { name: 'doc:changed', data: { dirty: true } } });
		sockets[0].push({ method: 'event', params: { name: 'doc:changed', data: { dirty: false } } });
		off();
		sockets[0].push({ method: 'event', params: { name: 'doc:changed', data: 3 } });
		expect(got).toEqual([{ dirty: true }, { dirty: false }]);
		expect(once).toEqual([{ dirty: true }]);
	});

	it('annule un appel ($/cancel) et rejette les appels en cours si la connexion tombe', async () => {
		const { sockets } = setup();
		const p = Call.ByID(7);
		sockets[0].open();
		p.cancel();
		await expect(p).rejects.toThrow('Appel annulé');
		expect(sockets[0].sent[1]).toEqual({ jsonrpc: '2.0', method: '$/cancel', params: { id: 1 } });
		const q = Call.ByID(8);
		sockets[0].close();
		await expect(q).rejects.toThrow('Connexion au moteur Lattis perdue');
	});

	it('dit la coupure, la session expirée et la reconnexion', async () => {
		vi.useFakeTimers();
		try {
			const sockets: FakeSocket[] = [];
			let expired = false;
			const c = new RpcClient(
				'ws://test/ws',
				() => {
					const s = new FakeSocket();
					sockets.push(s);
					return s;
				},
				async () => expired
			);
			setClient(c);
			const seen: string[] = [];
			const offs = ['host:disconnected', 'host:expired', 'host:reconnected'].map((n) => Events.On(n, () => seen.push(n)));
			Call.ByID(1).catch(() => {});
			sockets[0].open();
			sockets[0].close();
			expect(seen).toEqual(['host:disconnected']);
			// Reconnexions manquées : la session est essayée.
			expired = true;
			for (let i = 0; i < 4; i++) {
				await vi.advanceTimersByTimeAsync(10_000);
				sockets[sockets.length - 1].close();
			}
			await vi.advanceTimersByTimeAsync(0);
			expect(seen).toEqual(['host:disconnected', 'host:expired']);
			await vi.advanceTimersByTimeAsync(10_000);
			sockets[sockets.length - 1].open();
			expect(seen).toEqual(['host:disconnected', 'host:expired', 'host:reconnected']);
			offs.forEach((off) => off());
		} finally {
			vi.useRealTimers();
		}
	});
});

describe('raccourcis gardés par le navigateur', () => {
	it('ont un équivalent affiché en web', () => {
		expect(keyLabel('Ctrl+W', true)).toBe('Ctrl+Alt+W');
		expect(keyLabel('Ctrl+N', true)).toBe('Ctrl+Alt+N');
		expect(keyLabel('Ctrl+W', false)).toBe('Ctrl+W');
		expect(keyLabel('Ctrl+S', true)).toBe('Ctrl+S');
		const keys = webGroups(shortcutGroups).flatMap((g) => g.items.map((s) => s.keys));
		expect(keys).toContain('Ctrl+Alt+W');
		expect(keys).not.toContain('Ctrl+W');
		expect(keys).not.toContain('Ctrl+Pg.Suiv');
	});
});
