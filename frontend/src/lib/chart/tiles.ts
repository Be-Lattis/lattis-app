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
// undefined : demande abandonnée avant d'être partie (plus personne ne voulait la tuile).
const cache = new Map<string, Promise<string | null | undefined>>();
// Tuiles reçues, lisibles tout de suite (une carte qui renaît les affiche sans attendre).
const ready = new Map<string, string>();
const READY_MAX = 600;
// Qui attend chaque tuile pas encore partie (voir loadTile).
const wanters = new Map<string, Set<() => boolean>>();

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
	ready.clear();
	wanters.clear();
}

/** Délai avant de redemander une tuile qui a échoué (refus, hors ligne). */
export const RETRY_MS = 30_000;

/** Tuile déjà reçue (data URL), sans attendre ; undefined sinon. */
export function tileNow(z: number, x: number, y: number): string | undefined {
	return ready.get(`${z}/${x}/${y}`);
}

/**
 * Tuile en data URL, ou null (pas de source, refus, hors ligne). wanted dit
 * si l'appelant veut encore la tuile : une demande en file d'attente que
 * plus personne ne veut (la carte a été zoomée ou déplacée entre-temps)
 * n'est pas envoyée et rend undefined — on ne charge que ce qu'on voit
 * (règles d'usage des tuiles OpenStreetMap). Sans wanted, toujours voulue.
 */
export function loadTile(z: number, x: number, y: number, wanted: () => boolean = () => true): Promise<string | null | undefined> {
	const k = `${z}/${x}/${y}`;
	let p = cache.get(k);
	const ws = wanters.get(k);
	if (p && ws) ws.add(wanted);
	if (!p) {
		const f = fetcher;
		const mine = new Set([wanted]);
		wanters.set(k, mine);
		p = f
			? slot().then((done) => {
					wanters.delete(k);
					if (![...mine].some((w) => w())) {
						done();
						return undefined;
					}
					return f(z, x, y)
						.then(
							(s) => (typeof s === 'string' && s.startsWith('data:image/') ? s : null),
							() => null
						)
						.finally(done);
				})
			: Promise.resolve(null);
		const q = p;
		q.then((s) => {
			if (cache.get(k) !== q) return;
			if (s === undefined) cache.delete(k);
			else if (s === null) setTimeout(() => cache.get(k) === q && cache.delete(k), RETRY_MS);
			else {
				ready.set(k, s);
				// Mémoire bornée : les plus anciennes s'en vont (le classeur les garde aussi).
				for (const old of ready.keys()) {
					if (ready.size <= READY_MAX) break;
					ready.delete(old);
					cache.delete(old);
				}
			}
		});
		cache.set(k, p);
	}
	return p;
}
