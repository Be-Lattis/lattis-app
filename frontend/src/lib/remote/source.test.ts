import { describe, expect, it } from 'vitest';
import { advancedSummary, chooser, keptKeys, pushSummary, pushedLine, pushedOrigin, shareProblems, shareWays, sourceStatus, sqlNameFor, sqlNameProblem } from './source';

describe('chooser', () => {
	it('classeur non publié : les deux voies', () => {
		const c = chooser('workbook', '');
		expect(c.ways).toHaveLength(2);
		expect(c.why).toBe('');
	});
	it('classeur collaboratif sans ce tableau : seule la voie qui marche, sans refus affiché', () => {
		const c = chooser('table', "Un classeur collaboratif ne se republie pas : ce tableau n'y entre plus avec le classeur.");
		expect(c.title).toBe('Ce tableau n’est que sur votre poste');
		expect(c.lead).toContain('partagé en direct');
		expect(c.ways.map((w) => [w.way, w.title, w.disabled])).toEqual([['table', 'Publier ce tableau seul au Data Lake', '']]);
		expect(c.why).toContain('ne se republie pas');
	});
	it('classeur partagé en direct : ajouter au classeur partagé d’abord, puis le tableau seul', () => {
		const c = chooser('collab', '');
		expect(c.ways.map((w) => [w.way, w.title, w.disabled])).toEqual([
			['shared', 'Ajouter ce tableau au classeur partagé', ''],
			['table', 'Publier ce tableau seul au Data Lake', '']
		]);
		expect(c.why).toBe('');
		const off = chooser('collab', 'Hors ligne : un tableau s’ajoute au classeur partagé quand le serveur répond');
		expect(off.ways[0].disabled).toContain('Hors ligne');
		expect(off.ways[1].disabled).toBe('');
	});
});

describe('contrôle du formulaire', () => {
	const ok = { server: 's', name: 'Ventes', sqlName: 'ventes', columns: 3, badFilter: false };
	it('nom SQL', () => {
		expect(sqlNameProblem('')).toBe('');
		expect(sqlNameProblem('ventes_2026')).toBe('');
		expect(sqlNameProblem('2026')).toContain('lettre');
		expect(sqlNameProblem('Ventes')).toContain('lettre minuscule');
		expect(sqlNameProblem('ventes-2026')).toContain('Minuscules');
		expect(sqlNameProblem('a'.repeat(64))).toContain('63');
	});
	it('un message par champ à corriger', () => {
		expect(shareProblems(ok)).toEqual({});
		expect(shareProblems({ ...ok, server: '', name: ' ', columns: 0, mode: 'upsert', key: '', badFilter: true })).toEqual({
			server: 'Connectez-vous à un Serveur Lattis',
			name: 'Donnez un nom à la donnée',
			columns: 'Cochez au moins une colonne',
			key: 'Choisissez la colonne qui identifie une ligne',
			audience: 'Un filtre de lignes est à corriger'
		});
		expect(shareProblems({ ...ok, sqlName: 'X' }).sql).toBeTruthy();
		expect(shareProblems({ ...ok, sqlName: null })).toEqual({});
	});
	it('résumé des options avancées', () => {
		expect(advancedSummary({ sqlName: 'ventes', mode: 'upsert', key: 'Code', onSave: true })).toBe('donnees.ventes · Mettre à jour par clé (Code) · à chaque enregistrement');
		expect(advancedSummary({ sqlName: '', mode: 'replace', onSave: false, suspended: true })).toBe('Remplacer tout · suspendue');
	});
});

describe('shareWays', () => {
	it('classeur non publié : publier le classeur ou le tableau seul', () => {
		const w = shareWays('workbook', '');
		expect(w.map((x) => x.way)).toEqual(['workbook', 'table']);
		expect(w[0].title).toContain('gestion des droits');
		expect(w[0].disabled).toBe('');
		expect(w[1].detail).toContain('le classeur reste sur votre poste');
	});
	it('classeur publié sans ce tableau : republier, ou fermé avec la raison', () => {
		expect(shareWays('table', '')[0].title).toContain('Republier');
		const w = shareWays('table', 'Un classeur collaboratif ne se republie pas');
		expect(w[0].disabled).toContain('collaboratif');
		expect(w[1].disabled).toBe('');
	});
});

describe('pushSummary', () => {
	const r = { status: 'ok', rows_read: 3, inserted: 1, updated: 2, deleted: 0, rejected: 0 };
	it('dit ce qui a changé', () => {
		expect(pushSummary('Ventes', r)).toBe('Source « Ventes » mise à jour : 3 lignes lues (1 ajoutée, 2 modifiées)');
	});
	it('rien de changé, valeurs vides', () => {
		expect(pushSummary('T', { ...r, rows_read: 1, inserted: 0, updated: 0 }, 2)).toBe(
			"Source « T » mise à jour : 1 ligne lue, rien n'a changé ; 2 valeurs envoyées vides (erreur ou saisie hors type)"
		);
	});
	it('première valeur refusée', () => {
		const w = { ...r, status: 'warning', updated: 0, rejected: 1, rejects: [{ row: 3, column: 'Prix', value: 'abc', error: 'nombre attendu' }] };
		expect(pushSummary('T', w)).toBe('Source « T » mise à jour : 3 lignes lues (1 ajoutée, 1 refusée) ; ligne 3, Prix « abc » : nombre attendu');
	});
});

describe('sourceStatus', () => {
	const now = new Date('2026-10-19T15:00:00');
	it('échec, jamais, tableau supprimé', () => {
		expect(sourceStatus({ tableName: 'T', lastError: 'refusé' }, now)).toBe('Dernière mise à jour en échec : refusé');
		expect(sourceStatus({ tableName: 'T' }, now)).toBe('Jamais mise à jour');
		expect(sourceStatus({ tableName: '' }, now)).toContain('supprimé');
		expect(sourceStatus({ tableName: 'T', suspended: true, lastError: 'x' }, now)).toBe('Source suspendue : plus aucun envoi');
	});
	it('dernier envoi du jour', () => {
		const s = sourceStatus({ tableName: 'T', lastPush: new Date('2026-10-19T14:05:00').toISOString(), lastResult: { status: 'warning', rows_read: 12, inserted: 0, updated: 0, deleted: 0, rejected: 2 } }, now);
		expect(s).toMatch(/^Mise à jour aujourd’hui à 14:05 · 12 lignes · 2 refusées$/);
	});
});

describe('sqlNameFor', () => {
	it('minuscules sans accents, commence par une lettre', () => {
		expect(sqlNameFor('Prix du café')).toBe('prix_du_cafe');
		expect(sqlNameFor('2026 Ventes')).toBe('d_2026_ventes');
		expect(sqlNameFor('  ')).toBe('');
	});
});

describe('pushedOrigin et pushedLine', () => {
	it('origine d’une source poussée au catalogue', () => {
		expect(pushedOrigin({ source: { kind: 'source', title: 'Source : Prix', origin: { workbook: 'Ventes', table: 'Prix' } } })).toBe('Ventes › Prix');
		expect(pushedOrigin({ source: { kind: 'source', title: 'Source : Prix', origin: null } })).toBe('Source : Prix');
		expect(pushedOrigin({ source: { kind: 'workbook', title: 'Ventes' } })).toBeNull();
		expect(pushedOrigin({ source: null })).toBeNull();
	});
	it('ligne d’état', () => {
		const p = { pipeline_id: 'p', name: 'Prix', mode: 'upsert', key_columns: ['Code'], origin: { workbook: 'Ventes', table: 'Prix' }, enabled: false, pending: false, datasets: [], last_run: null };
		expect(pushedLine(p)).toBe('Ventes › Prix · Mettre à jour par clé (Code) · suspendue · jamais mise à jour');
	});
});

describe('keptKeys', () => {
	it('une clé non envoyée tombe', () => {
		expect(keptKeys(['Code', 'Nom'], new Set(['Nom', 'Prix']))).toEqual(['Nom']);
	});
});
