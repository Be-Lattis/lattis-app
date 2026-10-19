// Tâches longues : chaque appel au moteur (Wails passe par fetch vers
// /wails/runtime) est compté ; tant qu'au moins un appel dure depuis plus de
// DELAY ms, la barre d'état affiche une barre de progression (BusyBar).

import { writable, type Readable } from 'svelte/store';

/** Délai avant d'afficher la barre : les appels rapides ne la montrent pas. */
export const DELAY = 400;

/** Suivi des appels en cours (fonctions pures, testées). */
export class Pending {
	private starts = new Map<number, number>();
	private next = 0;

	begin(now: number): number {
		const id = this.next++;
		this.starts.set(id, now);
		return id;
	}

	end(id: number) {
		this.starts.delete(id);
	}

	/** Un appel dure depuis au moins delay ms. */
	long(now: number, delay = DELAY): boolean {
		for (const t of this.starts.values()) if (now - t >= delay) return true;
		return false;
	}

	get size() {
		return this.starts.size;
	}
}

const state = writable(false);
/** Vrai quand une tâche longue est en cours. */
export const busy: Readable<boolean> = { subscribe: state.subscribe };

const calls = writable(0);
/**
 * Appels de l'interface en cours qui attendent le serveur d'un classeur
 * partagé (syncCall) : la barre d'état dit « Synchronisation… » et pas
 * « Traitement… » (décision 0012).
 */
export const syncCalls: Readable<number> = { subscribe: calls.subscribe };

/** Marque un appel qui attend le serveur (voir syncCalls). */
export async function syncCall<T>(p: Promise<T>): Promise<T> {
	calls.update((n) => n + 1);
	try {
		return await p;
	} finally {
		calls.update((n) => n - 1);
	}
}

let installed = false;

/** Installe le suivi des appels (une fois). */
export function trackCalls() {
	if (installed || typeof window === 'undefined' || !window.fetch) return;
	installed = true;
	const pending = new Pending();
	let timer = 0;
	const refresh = () => {
		const now = performance.now();
		state.set(pending.long(now));
		window.clearTimeout(timer);
		if (pending.size) timer = window.setTimeout(refresh, pending.long(now) ? 250 : DELAY);
	};
	const fetch0 = window.fetch.bind(window);
	window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
		const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
		if (!url.includes('/wails/runtime')) return fetch0(input, init);
		const id = pending.begin(performance.now());
		refresh();
		return fetch0(input, init).finally(() => {
			pending.end(id);
			refresh();
		});
	};
}
