import { describe, expect, it } from 'vitest';
import { syncActive, syncLabel, syncOf, type SyncStatus } from './sync';
import { syncCall, syncCalls } from './busy';
import { get } from 'svelte/store';

const on = (pending = 0, sending = false): SyncStatus => ({ shared: true, online: true, pending, sending });

describe('barre d’état : synchronisation', () => {
	it('dit « Synchronisation… » tant que la file part ou attend, avec le nombre au-delà d’une étape', () => {
		expect(syncLabel(on(0, true))).toEqual({ text: 'Synchronisation…', busy: true, warn: false });
		expect(syncLabel(on(1))?.text).toBe('Synchronisation…');
		expect(syncLabel(on(3, true))?.text).toBe('Synchronisation… (3)');
	});
	it('dit « Synchronisé » juste après, puis plus rien', () => {
		expect(syncLabel(on(), 0, true)).toEqual({ text: 'Synchronisé', busy: false, warn: false });
		expect(syncLabel(on())).toBeNull();
	});
	it('hors ligne : le nombre en attente, sans animation', () => {
		expect(syncLabel({ shared: true, online: false, pending: 3, sending: false })).toEqual({ text: 'Hors ligne — 3 en attente', busy: false, warn: true });
		expect(syncLabel({ shared: true, online: false, pending: 0, sending: false }, 0, true)).toBeNull();
	});
	it('un appel qui attend le serveur compte aussi (classeur local compris)', () => {
		expect(syncLabel(null, 1)?.text).toBe('Synchronisation…');
		expect(syncActive(null, 1)).toBe(true);
		expect(syncActive(on())).toBe(false);
		expect(syncActive(on(2))).toBe(true);
		expect(syncLabel(null)).toBeNull();
	});
	it('ne vaut que pour un classeur collaboratif', () => {
		expect(syncOf({ shared: true, mode: 'frozen', online: true, pending: 2 })).toBeNull();
		expect(syncOf(null)).toBeNull();
		expect(syncOf({ shared: true, mode: 'collaborative', online: true, pending: 2, sending: true })).toEqual({ shared: true, online: true, pending: 2, sending: true });
	});
	it('syncCall compte les appels en cours, même en échec', async () => {
		let done!: () => void;
		const p = syncCall(new Promise<void>((r) => (done = r)));
		expect(get(syncCalls)).toBe(1);
		done();
		await p;
		expect(get(syncCalls)).toBe(0);
		await expect(syncCall(Promise.reject(new Error('refusé')))).rejects.toThrow('refusé');
		expect(get(syncCalls)).toBe(0);
	});
});
