import { describe, expect, it } from 'vitest';
import { isWrite } from './write.js';

describe('isWrite', () => {
	it.each([
		['UPDATE Commandes SET Montant = 1', true],
		['  -- note\n/* x */ delete from "Commandes"', true],
		['INSERT INTO Commandes (Client) VALUES (\'a\')', true],
		['SELECT * FROM Commandes', false],
		["SELECT 'update' FROM t", false],
		['WITH a AS (SELECT 1) UPDATE Commandes SET x = 1', true],
		['WITH a AS (SELECT 1 AS "delete") SELECT * FROM a', false],
		['', false]
	])('%s', (sql, want) => expect(isWrite(sql)).toBe(want));
});
