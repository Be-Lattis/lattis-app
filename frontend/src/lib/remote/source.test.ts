import { describe, expect, it } from 'vitest';
import { keptKeys, pushSummary, shareWays, sourceStatus, sqlNameFor } from './source';

describe('shareWays', () => {
	it('classeur non publié : publier le classeur ou le tableau seul', () => {
		const w = shareWays('workbook', '');
		expect(w.map((x) => x.way)).toEqual(['workbook', 'table']);
		expect(w[0].title).toContain('gestion des droits');
		expect(w[0].disabled).toBe('');
		expect(w[1].detail).toContain('sans être mis à disposition');
	});
	it('classeur publié sans ce tableau : republier, ou fermé avec la raison', () => {
		expect(shareWays('table', '')[0].title).toContain('Republier');
		const w = shareWays('table', 'Un classeur collaboratif ne se republie pas');
		expect(w[0].disabled).toContain('collaboratif');
		expect(w[1].disabled).toBe('');
	});
});

describe('pushSummary', () => {
	const r = { run_id: 'r', status: 'ok', rows: 3, inserted: 1, updated: 2, deleted: 0, rejected: 0 };
	it('dit ce qui a changé', () => {
		expect(pushSummary('Ventes', r)).toBe('Source « Ventes » mise à jour : 3 lignes (1 ajoutée, 2 modifiées)');
	});
	it('rien de changé, valeurs vides', () => {
		expect(pushSummary('T', { ...r, rows: 1, inserted: 0, updated: 0 }, 2)).toBe(
			"Source « T » mise à jour : 1 ligne, rien n'a changé ; 2 valeurs envoyées vides (erreur ou saisie hors type)"
		);
	});
});

describe('sourceStatus', () => {
	const now = new Date('2026-10-19T15:00:00');
	it('échec, jamais, tableau supprimé', () => {
		expect(sourceStatus({ tableName: 'T', lastError: 'refusé' }, now)).toBe('Dernière mise à jour en échec : refusé');
		expect(sourceStatus({ tableName: 'T' }, now)).toBe('Jamais mise à jour');
		expect(sourceStatus({ tableName: '' }, now)).toContain('supprimé');
	});
	it('dernier envoi du jour', () => {
		const s = sourceStatus({ tableName: 'T', lastPush: new Date('2026-10-19T14:05:00').toISOString(), lastResult: { run_id: '', status: 'ok', rows: 12, inserted: 0, updated: 0, deleted: 0, rejected: 0 } }, now);
		expect(s).toMatch(/^Mise à jour aujourd’hui à 14:05 · 12 lignes$/);
	});
});

describe('sqlNameFor', () => {
	it('minuscules sans accents, commence par une lettre', () => {
		expect(sqlNameFor('Prix du café')).toBe('prix_du_cafe');
		expect(sqlNameFor('2026 Ventes')).toBe('d_2026_ventes');
		expect(sqlNameFor('  ')).toBe('');
	});
});

describe('keptKeys', () => {
	it('une clé non envoyée tombe', () => {
		expect(keptKeys(['Code', 'Nom'], new Set(['Nom', 'Prix']))).toEqual(['Nom']);
	});
});
