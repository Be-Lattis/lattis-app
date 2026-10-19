import { describe, expect, it } from 'vitest';
import { alterRename } from './rename';

describe('ALTER TABLE … RENAME TO', () => {
	it('lit les deux noms, avec ou sans guillemets', () => {
		expect(alterRename('ALTER TABLE Tableau1 RENAME TO Ventes')).toEqual({ from: 'Tableau1', to: 'Ventes' });
		expect(alterRename('alter table "Mes ventes" rename to "Ventes 2026";')).toEqual({ from: 'Mes ventes', to: 'Ventes 2026' });
		expect(alterRename('ALTER TABLE [Clients] RENAME TO Clientèle')).toEqual({ from: 'Clients', to: 'Clientèle' });
	});
	it('ignore les autres requêtes', () => {
		expect(alterRename('ALTER TABLE x ADD COLUMN y')).toBeNull();
		expect(alterRename('SELECT 1')).toBeNull();
	});
});
