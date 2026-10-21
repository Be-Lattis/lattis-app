// Tuiles OpenStreetMap des cartes. L'interface ne les demande pas au réseau
// elle-même : elles passent par le classeur (commande charts.tile), soumises
// à la même autorisation par hôte que le module http des scripts (accord de
// l'utilisateur, indépendant du scripting). Sans accord, ou hors ligne, la
// carte garde un fond neutre et dit pourquoi (tileProblem) : démo publique
// sans connexion sortante, hôte non autorisé (bouton pour l'autoriser,
// allowTiles), ou erreur du réseau.
//
// Deux demandes au plus à la fois : la première peut attendre la réponse de
// l'utilisateur (autoriser l'hôte), et cette réponse doit pouvoir passer
// (un navigateur ne garde que quelques connexions ouvertes par serveur).

import { errorText, msgCause } from '#lib/i18n/errors.js';

export type TileFetcher = (z: number, x: number, y: number) => Promise<string>;

/** Pourquoi les tuiles manquent : démo publique, hôte non autorisé, autre échec (text). */
export interface TileProblem {
	kind: 'demo' | 'refused' | 'failed';
	text: string;
}

let fetcher: TileFetcher | null = null;
let allower: (() => Promise<unknown>) | null = null;
let problem: TileProblem | null = null;

/** Lit l'échec d'une demande de tuile (erreur du moteur, charts.tile). */
export function problemOf(e: unknown): TileProblem {
	const id = msgCause(e)?.id ?? '';
	if (id === 'api.tileDemoOffline') return { kind: 'demo', text: errorText(e) };
	if (id.startsWith('http.host')) return { kind: 'refused', text: errorText(e) };
	return { kind: 'failed', text: errorText(e) };
}

/** Dernier échec des tuiles (null : la dernière est arrivée). */
export function tileProblem(): TileProblem | null {
	return problem;
}

/** Branche l'autorisation de l'hôte des tuiles (bouton de la carte, au démarrage). */
export function setTileAllower(f: (() => Promise<unknown>) | null) {
	allower = f;
}

/** Une autorisation des tuiles est branchée (pas dans la démo publique). */
export function canAllowTiles(): boolean {
	return !!allower;
}

/**
 * Autorise l'hôte des tuiles (réponse de l'utilisateur au bouton de la
 * carte) et oublie les échecs : les tuiles se redemandent tout de suite.
 */
export async function allowTiles() {
	if (!allower) return;
	await allower();
	problem = null;
	for (const k of failed) cache.delete(k);
	failed.clear();
}

// Tuiles en échec gardées dans cache (jusqu'à RETRY_MS) : allowTiles les oublie.
const failed = new Set<string>();
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
	failed.clear();
	problem = null;
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
							(s) => {
								if (typeof s !== 'string' || !s.startsWith('data:image/')) return null;
								problem = null;
								return s;
							},
							(e) => {
								problem = problemOf(e);
								return null;
							}
						)
						.finally(done);
				})
			: Promise.resolve(null);
		const q = p;
		q.then((s) => {
			if (cache.get(k) !== q) return;
			if (s === undefined) cache.delete(k);
			else if (s === null) {
				failed.add(k);
				setTimeout(() => {
					if (cache.get(k) !== q) return;
					cache.delete(k);
					failed.delete(k);
				}, RETRY_MS);
			}
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
