import { describe, expect, it } from 'vitest';
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
