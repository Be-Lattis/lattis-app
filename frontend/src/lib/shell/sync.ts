// Barre d'état d'un classeur du serveur (décision 0012, synchronisation.md
// §2.11) : les modifications sont faites et affichées en local sans
// attendre le serveur ; ce qui part au serveur se voit en bas à droite
// (« Synchronisation… »), à la place de « Traitement… », réservé au travail
// local long. Logique sans composant, testée par vitest.

/** File d'envoi d'un classeur collaboratif (workbook.SyncStatus, évènement share:sync). */
export interface SyncStatus {
	shared: boolean;
	online: boolean;
	pending: number;
	sending: boolean;
}

/** État de partage (workbook.State) réduit à sa file d'envoi. */
export function syncOf(s: { shared?: boolean; mode?: string; online?: boolean; pending?: number; sending?: boolean } | null | undefined): SyncStatus | null {
	if (!s?.shared || s.mode !== 'collaborative') return null;
	return { shared: true, online: !!s.online, pending: s.pending ?? 0, sending: !!s.sending };
}

export interface SyncLabel {
	text: string;
	/** Animation de chargement (« Synchronisation… »). */
	busy: boolean;
	/** Hors ligne avec des modifications en attente. */
	warn: boolean;
}

/** Durée de « Synchronisé » après la fin d'un envoi (ms). */
export const DONE_FOR = 1500;

/**
 * Texte de la barre d'état ; null : rien à dire. calls : appels de
 * l'interface en cours qui attendent le serveur (syncCall) ; done : un envoi
 * vient de finir (« Synchronisé » montré DONE_FOR ms).
 */
export function syncLabel(s: SyncStatus | null, calls = 0, done = false): SyncLabel | null {
	const n = s?.pending ?? 0;
	if (s?.shared && !s.online) {
		return n > 0 ? { text: `Hors ligne — ${n} en attente`, busy: false, warn: true } : null;
	}
	if (calls > 0 || (s?.shared && (s.sending || n > 0))) {
		return { text: n > 1 ? `Synchronisation… (${n})` : 'Synchronisation…', busy: true, warn: false };
	}
	if (done) return { text: 'Synchronisé', busy: false, warn: false };
	return null;
}

/** Vrai si la file part ou attend (pour repérer la fin d'un envoi). */
export function syncActive(s: SyncStatus | null, calls = 0): boolean {
	return calls > 0 || (!!s?.shared && s.online && (s.sending || s.pending > 0));
}
