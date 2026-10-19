import { describe, expect, it } from 'vitest';
import { peerName, presenceText, presenceTitle, presenceWarning } from './coedit';

const peer = (user: string, machine = '', joining = false) => ({ session: user, user, machine, since: '2026-10-19T09:41:00Z', joining });

describe('indicateur du travail à plusieurs', () => {
	it('ne dit rien hors du travail à plusieurs', () => {
		expect(presenceText(null)).toBe('');
		expect(presenceText({ active: false, peers: [peer('Bob')] })).toBe('');
		expect(presenceTitle({ active: false, peers: [] })).toBe('');
	});
	it('nomme les autres postes', () => {
		expect(peerName(peer('Bob', 'PC-ACCUEIL'))).toBe('Bob (PC-ACCUEIL)');
		expect(peerName(peer(''))).toBe('un autre utilisateur');
		expect(presenceText({ active: true, peers: [] })).toBe('À plusieurs : vous seul pour le moment');
		expect(presenceText({ active: true, peers: [peer('Bob', 'PC-1'), peer('Zoé', '', true)] })).toBe('À plusieurs avec Bob (PC-1)');
		expect(presenceText({ active: true, peers: ['A', 'B', 'C', 'D'].map((n) => peer(n)) })).toBe('À plusieurs avec A, B et 2 autres');
		expect(presenceTitle({ active: true, peers: [peer('Zoé', '', true)] })).toContain('Zoé (en train de rejoindre)');
	});
	it('avertit quand il le faut', () => {
		expect(presenceWarning({ active: true, peers: [], offline: 'Dossier partagé injoignable' })).toBe('Dossier partagé injoignable');
		expect(presenceWarning({ active: true, peers: [], stale: 'Rouvrez', offline: 'x' })).toBe('Rouvrez');
		expect(presenceWarning({ active: true, peers: [] })).toBe('');
	});
});
