import { describe, expect, it } from 'vitest';
import { demoClock, demoEndState } from './demoend';

describe('demoClock', () => {
	it('rien hors démo ou sans fin annoncée', () => {
		expect(demoClock(null, 0)).toBeNull();
		expect(demoClock({ demo: true }, 0)).toBeNull();
		expect(demoClock({ demo: false, demoDeadline: 1000, now: 0 }, 0)).toBeNull();
	});

	it("corrige l'écart de l'horloge du navigateur", () => {
		// Navigateur en avance de 5 s sur le moteur.
		expect(demoClock({ demo: true, demoDeadline: 100_000, demoWarnBefore: 60_000, now: 10_000 }, 15_000)).toEqual({ deadline: 100_000, warn: 60_000, offset: -5_000 });
		// Sans heure du moteur : pas de correction ; préavis absent : 0.
		expect(demoClock({ demo: true, demoDeadline: 100_000 }, 15_000)).toEqual({ deadline: 100_000, warn: 0, offset: 0 });
	});
});

describe('demoEndState', () => {
	const c = demoClock({ demo: true, demoDeadline: 1_000_000, demoWarnBefore: 120_000, now: 0 }, 0)!;

	it('avant le préavis : rien à montrer', () => {
		expect(demoEndState(c, 1_000_000 - 180_000)).toMatchObject({ phase: 'running', minutes: 3, countdown: '03:00' });
	});

	it('pendant le préavis : minutes réelles et compte à rebours', () => {
		expect(demoEndState(c, 1_000_000 - 120_000)).toMatchObject({ phase: 'warning', minutes: 2, countdown: '02:00' });
		expect(demoEndState(c, 1_000_000 - 90_500)).toMatchObject({ phase: 'warning', minutes: 2, countdown: '01:31' });
		expect(demoEndState(c, 1_000_000 - 59_000)).toMatchObject({ phase: 'warning', minutes: 1, countdown: '00:59' });
		expect(demoEndState(c, 1_000_000 - 1)).toMatchObject({ phase: 'warning', minutes: 1, countdown: '00:01' });
	});

	it("à l'heure et après : terminée", () => {
		expect(demoEndState(c, 1_000_000)).toMatchObject({ phase: 'ended', remaining: 0, countdown: '00:00' });
		expect(demoEndState(c, 2_000_000)).toMatchObject({ phase: 'ended', remaining: 0 });
	});

	it("l'horloge du moteur fait foi", () => {
		// Navigateur en retard de 30 s : la fin arrive 30 s plus tôt à son horloge.
		const late = demoClock({ demo: true, demoDeadline: 1_000_000, demoWarnBefore: 60_000, now: 500_000 }, 470_000)!;
		expect(demoEndState(late, 970_000).phase).toBe('ended');
		expect(demoEndState(late, 920_000)).toMatchObject({ phase: 'warning', countdown: '00:50' });
	});

	it('sans préavis : pas de bandeau, la fin reste', () => {
		const none = demoClock({ demo: true, demoDeadline: 1_000, now: 0 }, 0)!;
		expect(demoEndState(none, 999).phase).toBe('running');
		expect(demoEndState(none, 1_000).phase).toBe('ended');
	});
});
