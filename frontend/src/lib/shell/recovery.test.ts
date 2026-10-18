import { describe, expect, it } from 'vitest';
import { groupSessions, olderLabel, sessionName } from './recovery';

const s = (dir: string, modified: string, workbook = '', source = '') => ({ dir, modified, workbook, source });

describe('récupération : sessions par classeur', () => {
	it('regroupe par identifiant de classeur, la plus récente en tête', () => {
		const g = groupSessions([
			s('w1', '2026-10-18T18:40:55+02:00', 'aaa', 'C:\\a\\Ventes.lattis'),
			s('w2', '2026-10-18T18:58:44+02:00', 'aaa', 'D:\\b\\Ventes.lattis'),
			s('w3', '2026-10-18T18:50:00+02:00', 'bbb'),
			s('w4', '2026-10-18T18:10:00+02:00', 'aaa', 'C:\\a\\Ventes.lattis'),
		]);
		expect(g.map((x) => x.latest.dir)).toEqual(['w2', 'w3']);
		expect(g[0].older.map((x) => x.dir)).toEqual(['w1', 'w4']);
		expect(g[1].older).toEqual([]);
	});

	it('ne fusionne pas deux classeurs du même nom', () => {
		const g = groupSessions([
			s('w1', '2026-10-18T18:58:44+02:00', 'x1'),
			s('w2', '2026-10-18T18:58:44+02:00', 'x2'),
		]);
		expect(g).toHaveLength(2);
		expect(g.map((x) => sessionName(x.latest))).toEqual(['Classeur sans titre', 'Classeur sans titre']);
	});

	it('une session sans identifiant forme son propre groupe', () => {
		const g = groupSessions([s('w1', '2026-10-18T10:00:00Z'), s('w2', '2026-10-18T11:00:00Z')]);
		expect(g.map((x) => x.latest.dir)).toEqual(['w2', 'w1']);
	});

	it('nomme les sessions et les versions plus anciennes', () => {
		expect(sessionName(s('w', '', '', '/home/l/Budget.lattis'))).toBe('Budget.lattis');
		expect(olderLabel(1)).toBe('+ 1 version plus ancienne');
		expect(olderLabel(2)).toBe('+ 2 versions plus anciennes');
	});
});
