// Tuiles OpenStreetMap des cartes. L'interface ne les demande pas au réseau
// elle-même : elles passent par le classeur (commande charts.tile), soumises
// à la même autorisation par hôte que le module http des scripts. Sans
// accord, ou hors ligne, la carte garde un fond neutre.
//
// Deux demandes au plus à la fois : la première peut attendre la réponse de
// l'utilisateur (autoriser l'hôte), et cette réponse doit pouvoir passer
// (un navigateur ne garde que quelques connexions ouvertes par serveur).

export type TileFetcher = (z: number, x: number, y: number) => Promise<string>;

let fetcher: TileFetcher | null = null;
const cache = new Map<string, Promise<string | null>>();

/** Demandes simultanées au plus. */
export const MAX_PARALLEL = 2;
let running = 0;
const queue: (() => void)[] = [];

function slot(): Promise<() => void> {
	return new Promise((resolve) => {
		const go = () => {
			running++;
			resolve(() => {
				running--;
				queue.shift()?.();
			});
		};
		if (running < MAX_PARALLEL) go();
		else queue.push(go);
	});
}

/** Branche la source des tuiles (au démarrage de l'application). */
export function setTileFetcher(f: TileFetcher | null) {
	fetcher = f;
	cache.clear();
}

/** Délai avant de redemander une tuile qui a échoué (refus, hors ligne). */
export const RETRY_MS = 30_000;

/** Tuile en data URL, ou null (pas de source, refus, hors ligne). */
export function loadTile(z: number, x: number, y: number): Promise<string | null> {
	const k = `${z}/${x}/${y}`;
	let p = cache.get(k);
	if (!p) {
		const f = fetcher;
		p = f
			? slot().then((done) =>
					f(z, x, y)
						.then(
							(s) => (typeof s === 'string' && s.startsWith('data:image/') ? s : null),
							() => null
						)
						.finally(done)
				)
			: Promise.resolve(null);
		p.then((s) => {
			if (s === null) setTimeout(() => cache.get(k) === p && cache.delete(k), RETRY_MS);
		});
		cache.set(k, p);
	}
	return p;
}
