// Édition web : ce qui n'est que dans la session est gardé sur le serveur après
// chaque modification (brouillon, bridge.DraftStatus). Logique sans composant,
// testée par vitest.

import { writable } from 'svelte/store';
import { t } from '#lib/i18n/index.js';

/** État d'enregistrement sur le serveur (bridge.DraftStatus, évènement draft:status). */
export interface DraftStatus {
	/** Vide (rien à enregistrer), saving, saved ou failed. */
	state: string;
	/** Avec failed : tooBig, unreachable ou error. */
	reason?: string;
	/** Cette session garde ses brouillons sur le serveur. */
	active?: boolean;
}

export const draftStatus = writable<DraftStatus>({ state: '' });

/** Le travail de la session survit à sa fin : les avertissements « perdu » n'ont pas lieu d'être. */
export function draftKept(s: DraftStatus): boolean {
	return !!s.active && s.state !== 'failed';
}

export interface DraftLabel {
	text: string;
	busy: boolean;
	warn: boolean;
}

/** Texte de la barre d'état ; null : rien à dire. */
export function draftLabel(s: DraftStatus): DraftLabel | null {
	switch (s.state) {
		case 'saving':
			return { text: t('shell.draft.saving'), busy: true, warn: false };
		case 'saved':
			return { text: t('shell.draft.saved'), busy: false, warn: false };
		case 'failed':
			if (s.reason === 'tooBig') return { text: t('shell.draft.tooBig'), busy: false, warn: true };
			if (s.reason === 'unreachable') return { text: t('shell.draft.unreachable'), busy: false, warn: true };
			return { text: t('shell.draft.error'), busy: false, warn: true };
	}
	return null;
}
