import { afterEach, describe, expect, it } from 'vitest';
import { allowTiles, canAllowTiles, loadTile, problemOf, setTileAllower, setTileFetcher, tileProblem } from './tiles';

const engineError = (id: string, message = 'texte') => Object.assign(new Error(message), { cause: { id, message } });

afterEach(() => {
	setTileFetcher(null);
	setTileAllower(null);
});

describe('problemOf', () => {
	it('distingue la démo, le refus et les autres échecs', () => {
		expect(problemOf(engineError('api.tileDemoOffline', 'pas de réseau'))).toEqual({ kind: 'demo', text: 'pas de réseau' });
		expect(problemOf(engineError('http.hostRefusedByUser'))).toMatchObject({ kind: 'refused' });
		expect(problemOf(engineError('http.hostRefusedSession'))).toMatchObject({ kind: 'refused' });
		expect(problemOf(new Error('dial tcp: i/o timeout'))).toEqual({ kind: 'failed', text: 'dial tcp: i/o timeout' });
	});
});

describe('tuiles refusées puis autorisées', () => {
	it('retient le refus, puis redemande après l’accord', async () => {
		let allowed = false;
		let calls = 0;
		setTileFetcher(async () => {
			calls++;
			if (!allowed) throw engineError('http.hostRefusedByUser');
			return 'data:image/png;base64,AA==';
		});
		expect(canAllowTiles()).toBe(false);
		setTileAllower(async () => (allowed = true));
		expect(canAllowTiles()).toBe(true);
		expect(await loadTile(1, 0, 0)).toBeNull();
		expect(tileProblem()).toMatchObject({ kind: 'refused' });
		expect(await loadTile(1, 0, 0)).toBeNull(); // échec gardé (RETRY_MS) : pas de nouvelle demande
		expect(calls).toBe(1);
		await allowTiles();
		expect(tileProblem()).toBeNull();
		expect(await loadTile(1, 0, 0)).toBe('data:image/png;base64,AA==');
		expect(calls).toBe(2);
	});
	it('dit la démo publique sans réseau', async () => {
		setTileFetcher(async () => {
			throw engineError('api.tileDemoOffline');
		});
		expect(await loadTile(2, 1, 1)).toBeNull();
		expect(tileProblem()?.kind).toBe('demo');
	});
});
