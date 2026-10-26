import { describe, expect, it, vi } from 'vitest';
import { sleep, storageMemory, Tour, type TourMemory, type TourOutcome, type TourStep } from './tour';

// Classeur factice : chaque étape attend une valeur, que l'utilisateur ou
// Suivant (perform) y mettent.
type Ctx = { done: Set<string>; log: string[] };

function step(id: string, opts: { doneText?: boolean; enter?: boolean; fail?: boolean } = {}): TourStep<Ctx> {
	return {
		id,
		text: () => `texte ${id}`,
		doneText: opts.doneText ? () => `fait ${id}` : undefined,
		target: () => null,
		enter: opts.enter ? (ctx) => void ctx.log.push(`enter ${id}`) : undefined,
		check: (ctx) => ctx.done.has(id),
		perform: async (ctx) => {
			ctx.log.push(`perform ${id}`);
			if (opts.fail) throw new Error(`échec ${id}`);
			ctx.done.add(id);
		}
	};
}

function memory(): TourMemory & { value: TourOutcome | null } {
	const m = {
		value: null as TourOutcome | null,
		read: () => m.value,
		write: (v: TourOutcome) => void (m.value = v)
	};
	return m;
}

function setup(steps: TourStep<Ctx>[]) {
	const ctx: Ctx = { done: new Set(), log: [] };
	const mem = memory();
	const onend = vi.fn();
	const tour = new Tour(steps, ctx, { memory: mem, onend });
	return { ctx, mem, onend, tour };
}

describe('Tour', () => {
	it("avance quand l'utilisateur fait l'action (poll)", async () => {
		const { ctx, tour } = setup([step('a'), step('b')]);
		await tour.start();
		expect(tour.snapshot()).toMatchObject({ status: 'running', index: 0, total: 2, phase: 'act' });
		await tour.poll();
		expect(tour.snapshot().index).toBe(0); // rien de fait
		ctx.done.add('a');
		await tour.poll();
		expect(tour.snapshot()).toMatchObject({ index: 1, phase: 'act' });
	});

	it('conclut une étape (doneText) avant de passer à la suivante', async () => {
		const { ctx, tour } = setup([step('a', { doneText: true }), step('b')]);
		await tour.start();
		ctx.done.add('a');
		await tour.poll();
		expect(tour.snapshot()).toMatchObject({ index: 0, phase: 'done' });
		// En phase done, poll n'avance plus : Suivant le fait.
		await tour.poll();
		expect(tour.snapshot().index).toBe(0);
		await tour.next();
		expect(tour.snapshot()).toMatchObject({ index: 1, phase: 'act' });
	});

	it("Suivant fait l'action à la place de l'utilisateur (perform)", async () => {
		const { ctx, tour } = setup([step('a', { doneText: true }), step('b')]);
		await tour.start();
		await tour.next();
		expect(ctx.log).toEqual(['perform a']);
		expect(tour.snapshot()).toMatchObject({ index: 0, phase: 'done', busy: false });
		await tour.next();
		await tour.next();
		expect(ctx.log).toEqual(['perform a', 'perform b']);
		expect(tour.snapshot().status).toBe('done');
	});

	it("Suivant ne refait pas une action déjà faite", async () => {
		const { ctx, tour } = setup([step('a'), step('b')]);
		await tour.start();
		ctx.done.add('a');
		await tour.next();
		expect(ctx.log).toEqual([]);
		expect(tour.snapshot().index).toBe(1);
	});

	it("garde l'étape et l'erreur quand l'action échoue", async () => {
		const { tour } = setup([step('a', { fail: true }), step('b')]);
		await tour.start();
		await tour.next();
		expect(tour.snapshot()).toMatchObject({ index: 0, phase: 'act', error: 'échec a', busy: false });
	});

	it("prépare l'étape à son entrée (enter)", async () => {
		const { ctx, tour } = setup([step('a'), step('b', { enter: true })]);
		await tour.start();
		expect(ctx.log).toEqual([]);
		await tour.next();
		expect(ctx.log).toEqual(['perform a', 'enter b']);
	});

	it('retient « done » au bout de la visite', async () => {
		const { mem, onend, tour } = setup([step('a'), step('b')]);
		expect(tour.seen()).toBeNull();
		await tour.start();
		await tour.next();
		await tour.next();
		expect(tour.snapshot().status).toBe('done');
		expect(mem.value).toBe('done');
		expect(onend).toHaveBeenCalledWith('done');
		expect(tour.seen()).toBe('done');
	});

	it('Passer arrête la visite et la retient « skipped »', async () => {
		const { mem, onend, tour } = setup([step('a'), step('b')]);
		await tour.start();
		tour.skip();
		expect(tour.snapshot().status).toBe('skipped');
		expect(mem.value).toBe('skipped');
		expect(onend).toHaveBeenCalledWith('skipped');
		// Plus rien ne bouge ensuite.
		await tour.next();
		await tour.poll();
		expect(tour.snapshot().status).toBe('skipped');
	});

	it("Passer pendant une action l'interrompt", async () => {
		const ctx: Ctx = { done: new Set(), log: [] };
		let aborted = false;
		const slow: TourStep<Ctx> = {
			...step('a'),
			perform: async (_c, signal) => {
				try {
					await sleep(1000, signal);
				} catch {
					aborted = signal.aborted;
					throw new Error('interrompu');
				}
			}
		};
		const tour = new Tour([slow, step('b')], ctx, {});
		await tour.start();
		const p = tour.next();
		expect(tour.snapshot().busy).toBe(true);
		tour.skip();
		await p;
		expect(aborted).toBe(true);
		expect(tour.snapshot()).toMatchObject({ status: 'skipped', error: '' });
	});

	it('se relance depuis la première étape', async () => {
		const { ctx, tour } = setup([step('a'), step('b')]);
		await tour.start();
		tour.skip();
		ctx.done.clear();
		await tour.start();
		expect(tour.snapshot()).toMatchObject({ status: 'running', index: 0 });
	});

	it('signale chaque changement (onchange)', async () => {
		const seen: string[] = [];
		const tour = new Tour([step('a', { doneText: true })], { done: new Set<string>(), log: [] } as Ctx, { onchange: (s) => seen.push(`${s.status}:${s.index}:${s.phase}:${s.busy}`) });
		await tour.start();
		await tour.next();
		await tour.next();
		expect(seen).toEqual(['running:0:act:false', 'running:0:act:true', 'running:0:done:false', 'done:0:done:false']);
	});
});

describe('storageMemory', () => {
	it('lit et écrit l’issue dans un Storage', () => {
		const data = new Map<string, string>();
		const store = { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => void data.set(k, v) };
		const m = storageMemory(() => store);
		expect(m.read()).toBeNull();
		m.write('skipped');
		expect(m.read()).toBe('skipped');
		data.set('lattis.tour', 'autre');
		expect(m.read()).toBeNull();
	});

	it('survit à un stockage indisponible', () => {
		const m = storageMemory(() => {
			throw new Error('bloqué');
		});
		expect(m.read()).toBeNull();
		expect(() => m.write('done')).not.toThrow();
	});
});
