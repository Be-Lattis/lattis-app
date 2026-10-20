import { describe, expect, it, vi } from 'vitest';

vi.mock('#bindings/internal/bridge/index.js', () => ({
	DocumentService: { LattisState: vi.fn(async () => ({ enabled: true, allowUserServers: true, servers: [] })) }
}));

import { lattis, requestLogin, requestLoginOrLocal } from './lattis.svelte.js';

describe('page de connexion pour ouvrir un classeur partagé', () => {
	it('« Ouvrir quand même (en local) » rend local', async () => {
		lattis.state = { enabled: true, allowUserServers: true, servers: [] };
		const p = requestLoginOrLocal('https://lattis.test');
		await vi.waitFor(() => expect(lattis.login?.local).toBe(true));
		lattis.login!.done('local');
		expect(await p).toBe('local');
	});
	it('requestLogin ne propose pas la copie locale', async () => {
		lattis.state = { enabled: true, allowUserServers: true, servers: [] };
		const p = requestLogin('https://lattis.test');
		await vi.waitFor(() => expect(lattis.login).not.toBeNull());
		expect(lattis.login?.local).toBe(false);
		lattis.login!.done(false);
		expect(await p).toBe(false);
	});
});
