import { describe, expect, it } from 'vitest';
import { decodeObj, encodeObj, objFromText, rememberObj } from './objects';

describe('objets du presse-papiers', () => {
	it('graphique : aller-retour', () => {
		const o = { kind: 'chart' as const, config: { source: 'Ventes', kind: 'bar' } };
		expect(decodeObj(encodeObj(o))).toEqual(o);
		expect(decodeObj('Ventes\t12')).toBeNull();
		expect(decodeObj('⁣lattis-chart:{illisible')).toBeNull();
	});
	it('tableau croisé retrouvé par son texte', () => {
		const o = { kind: 'pivot' as const, config: { source: 'Ventes' } };
		rememberObj('Pays\tVentes\r\nFrance\t3\r\n', o);
		expect(objFromText('Pays\tVentes\nFrance\t3')).toEqual(o);
		expect(objFromText('Pays\tVentes')).toBeNull();
		rememberObj('', null);
		expect(objFromText('Pays\tVentes\nFrance\t3')).toBeNull();
	});
});
