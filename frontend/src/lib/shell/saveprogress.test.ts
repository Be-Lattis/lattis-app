import { describe, expect, it, vi } from 'vitest';
import { saveProgressText, withSaveProgress } from './saveprogress';

describe('saveProgressText', () => {
	it('décrit chaque étape', () => {
		expect(saveProgressText(null)).toBe('');
		expect(saveProgressText({ phase: '', done: 0, total: 0 })).toBe('');
		expect(saveProgressText({ phase: 'snapshot', done: 0, total: 0 })).toContain('copie de la base');
		expect(saveProgressText({ phase: 'write', done: 50, total: 200 })).toBe('Enregistrement : écriture du fichier 25 %');
		expect(saveProgressText({ phase: 'write', done: 10, total: 0 })).toContain('0 %');
		expect(saveProgressText({ phase: 'verify', done: 1, total: 1 })).toContain('vérification');
	});
});

describe('withSaveProgress', () => {
	it('n’affiche rien pour un enregistrement rapide', async () => {
		const show = vi.fn();
		const poll = vi.fn(async () => ({ phase: 'write', done: 1, total: 2 }));
		expect(await withSaveProgress(async () => 'ok', poll, show)).toBe('ok');
		expect(poll).not.toHaveBeenCalled();
		expect(show).not.toHaveBeenCalled();
	});

	it('affiche l’avancement d’un enregistrement long, puis l’efface', async () => {
		vi.useFakeTimers();
		try {
			const shown: string[] = [];
			let finish!: (v: string) => void;
			const run = () => new Promise<string>((r) => (finish = r));
			const p = withSaveProgress(run, async () => ({ phase: 'write', done: 3, total: 4 }), (t) => shown.push(t), 400, 250);
			await vi.advanceTimersByTimeAsync(450);
			expect(shown).toEqual(['Enregistrement : écriture du fichier 75 %']);
			finish('fini');
			expect(await p).toBe('fini');
			expect(shown.at(-1)).toBe('');
			await vi.advanceTimersByTimeAsync(1000);
			expect(shown.length).toBe(2);
		} finally {
			vi.useRealTimers();
		}
	});

	it('efface l’avancement même si l’enregistrement échoue', async () => {
		vi.useFakeTimers();
		try {
			const shown: string[] = [];
			let fail!: (e: Error) => void;
			const run = () => new Promise<string>((_, j) => (fail = j));
			const p = withSaveProgress(run, async () => ({ phase: 'snapshot', done: 0, total: 0 }), (t) => shown.push(t));
			await vi.advanceTimersByTimeAsync(450);
			fail(new Error('disque plein'));
			await expect(p).rejects.toThrow('disque plein');
			expect(shown.at(-1)).toBe('');
		} finally {
			vi.useRealTimers();
		}
	});
});
