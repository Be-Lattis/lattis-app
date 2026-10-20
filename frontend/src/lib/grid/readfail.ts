// Lecture d'un tableau refusée par le moteur (bloc de lignes « hors du
// tableau » : la mise en page de la grille en annonçait plus qu'il n'en a).
// Plutôt qu'un message technique répété, la grille relit sa mise en page et
// recharge le tableau, une fois ; le détail reste dans la console.
import { t } from '#lib/i18n/index.js';

/** Message de la barre d'état. */
export function readFailNotice(table: string): string {
	return t('grid.readFail', { table });
}

/** Délai pendant lequel un tableau déjà rechargé ne l'est plus (pas de boucle). */
export const RECOVER_EVERY_MS = 30_000;

/**
 * Rechargements faits après une lecture refusée, par tableau : un seul par
 * période, pour qu'une erreur qui ne passe pas en rechargeant ne fasse pas
 * relire la mise en page sans fin.
 */
export class ReadRecovery {
	private last = new Map<number, number>();

	constructor(private every = RECOVER_EVERY_MS) {}

	/** Vrai s'il faut recharger le tableau maintenant (et le note). */
	take(table: number, now: number): boolean {
		const t = this.last.get(table);
		if (t !== undefined && now - t < this.every) return false;
		this.last.set(table, now);
		return true;
	}

	/** Autre feuille, autre classeur : tout est oublié. */
	reset() {
		this.last.clear();
	}
}
