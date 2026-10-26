// Moteur de visite guidée : des étapes décrites comme des données (cible
// montrée, texte, vérification de l'action, action faite à la place de
// l'utilisateur), une bulle par étape, Passer à tout moment.
//
// Une étape commence en phase « act » : l'utilisateur fait l'action, ce que
// check vérifie à chaque appel de poll() ; Suivant (next) la fait à sa place
// (perform, animée), puis la vérifie. L'action faite, l'étape passe en phase
// « done » si elle a un texte de conclusion (doneText : ce qu'il faut
// remarquer), sinon à l'étape suivante ; en phase « done », Suivant avance.
// Après la dernière étape, la visite est « done » ; Passer la rend
// « skipped ». L'issue est retenue dans memory (TourMemory).
//
// Le moteur ne connaît ni le DOM ni Svelte : la bulle (TourBubble.svelte)
// lit snapshot() et s'abonne par onchange.

export type TourOutcome = 'done' | 'skipped';

export type TourAside = { code?: string; grid?: string[][] };

/** Où retenir l'issue de la visite (poste, session du navigateur…). */
export interface TourMemory {
	read(): TourOutcome | null;
	write(v: TourOutcome): void;
}

/** Étape de la visite ; C : contexte fourni par l'hôte (accès au classeur, à l'interface). */
export interface TourStep<C> {
	id: string;
	/** Texte de la bulle pendant l'action (une phrase), selon l'état du classeur. */
	text: (ctx: C) => string;
	/** Texte une fois l'action faite (absent : l'étape suivante vient aussitôt). */
	doneText?: (ctx: C) => string;
	/** Élément ou rectangle montré par la bulle (null : bulle centrée). */
	target: (ctx: C, phase: 'act' | 'done') => Element | DOMRect | null;
	/** Complément de la bulle : formule à taper (code) ou lignes à taper (grid). */
	aside?: (ctx: C, phase: 'act' | 'done') => TourAside | null;
	/** Préparation à l'entrée dans l'étape (tableau préparé…). */
	enter?: (ctx: C, signal: AbortSignal) => Promise<void> | void;
	/** L'action est-elle faite ? */
	check: (ctx: C) => boolean | Promise<boolean>;
	/** Fait l'action à la place de l'utilisateur (Suivant), en l'animant. */
	perform: (ctx: C, signal: AbortSignal) => Promise<void>;
}

export type TourStatus = 'idle' | 'running' | 'done' | 'skipped';
export type TourPhase = 'act' | 'done';

export interface TourSnapshot {
	status: TourStatus;
	index: number;
	total: number;
	phase: TourPhase;
	/** Suivant fait l'action (ou l'étape se prépare) : boutons occupés. */
	busy: boolean;
	/** Dernière erreur d'une action faite par Suivant (vide sinon). */
	error: string;
}

export interface TourOptions {
	memory?: TourMemory;
	onchange?: (s: TourSnapshot) => void;
	/** Fin de la visite (faite ou passée). */
	onend?: (outcome: TourOutcome) => void;
}

export class Tour<C> {
	private status: TourStatus = 'idle';
	private index = 0;
	private phase: TourPhase = 'act';
	private busy = false;
	private checking = false;
	private error = '';
	private abort = new AbortController();

	constructor(
		readonly steps: TourStep<C>[],
		readonly ctx: C,
		private readonly opts: TourOptions = {}
	) {}

	/** La visite a-t-elle déjà été faite ou passée (memory) ? */
	seen(): TourOutcome | null {
		return this.opts.memory?.read() ?? null;
	}

	snapshot(): TourSnapshot {
		return { status: this.status, index: this.index, total: this.steps.length, phase: this.phase, busy: this.busy, error: this.error };
	}

	get step(): TourStep<C> | undefined {
		return this.status === 'running' ? this.steps[this.index] : undefined;
	}

	/** Lance la visite à la première étape (relance comprise). */
	async start() {
		this.abort.abort();
		this.abort = new AbortController();
		this.status = 'running';
		this.error = '';
		await this.enter(0);
	}

	/**
	 * Vérifie l'étape en cours (à appeler quand le classeur ou la vue
	 * changent, et régulièrement) : l'action faite, l'étape conclut ou avance.
	 */
	async poll(): Promise<void> {
		const step = this.step;
		if (!step || this.phase !== 'act' || this.busy || this.checking) return;
		this.checking = true;
		let ok = false;
		try {
			ok = await step.check(this.ctx);
		} catch {
			ok = false;
		} finally {
			this.checking = false;
		}
		// L'étape a pu changer pendant la vérification (Suivant, Passer).
		if (ok && this.step === step && this.phase === 'act' && !this.busy) await this.completed();
	}

	/** Suivant : fait l'action de l'étape (phase act), ou passe à la suivante (phase done). */
	async next(): Promise<void> {
		const step = this.step;
		if (!step || this.busy) return;
		if (this.phase === 'done') return this.enter(this.index + 1);
		const signal = this.abort.signal;
		this.busy = true;
		this.error = '';
		this.emit();
		try {
			if (!(await step.check(this.ctx))) await step.perform(this.ctx, signal);
		} catch (e) {
			if (!signal.aborted) this.error = e instanceof Error ? e.message : String(e);
		}
		this.busy = false;
		if (signal.aborted || this.step !== step) return;
		let ok = false;
		try {
			ok = !this.error && (await step.check(this.ctx));
		} catch {
			ok = false;
		}
		if (ok) await this.completed();
		else this.emit();
	}

	/** Passer : la visite s'arrête, retenue comme passée. */
	skip() {
		if (this.status !== 'running') return;
		this.end('skipped');
	}

	private async completed() {
		if (this.step?.doneText) {
			this.phase = 'done';
			this.emit();
			return;
		}
		await this.enter(this.index + 1);
	}

	private async enter(i: number) {
		if (i >= this.steps.length) {
			this.end('done');
			return;
		}
		this.index = i;
		this.phase = 'act';
		const step = this.steps[i];
		const signal = this.abort.signal;
		if (step.enter) {
			this.busy = true;
			this.emit();
			try {
				await step.enter(this.ctx, signal);
			} catch (e) {
				if (!signal.aborted) this.error = e instanceof Error ? e.message : String(e);
			}
			this.busy = false;
			if (signal.aborted || this.index !== i || this.status !== 'running') return;
		}
		this.emit();
		await this.poll();
	}

	private end(outcome: TourOutcome) {
		this.abort.abort();
		this.status = outcome;
		this.busy = false;
		try {
			this.opts.memory?.write(outcome);
		} catch {
			// mémoire indisponible : la visite reviendra, sans plus
		}
		this.emit();
		this.opts.onend?.(outcome);
	}

	private emit() {
		this.opts.onchange?.(this.snapshot());
	}
}

/** Mémoire dans un Storage (sessionStorage : la session du navigateur ; localStorage : le navigateur). */
export function storageMemory(store: () => Pick<Storage, 'getItem' | 'setItem'> | null, key = 'lattis.tour'): TourMemory {
	return {
		read() {
			try {
				const v = store()?.getItem(key);
				return v === 'done' || v === 'skipped' ? v : null;
			} catch {
				return null;
			}
		},
		write(v) {
			try {
				store()?.setItem(key, v);
			} catch {
				// stockage indisponible
			}
		}
	};
}

/** Attente interrompue par signal (rejette avec une AbortError). */
export function sleep(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		if (signal?.aborted) return reject(new DOMException('aborted', 'AbortError'));
		const id = setTimeout(() => {
			signal?.removeEventListener('abort', stop);
			resolve();
		}, ms);
		const stop = () => {
			clearTimeout(id);
			reject(new DOMException('aborted', 'AbortError'));
		};
		signal?.addEventListener('abort', stop, { once: true });
	});
}
