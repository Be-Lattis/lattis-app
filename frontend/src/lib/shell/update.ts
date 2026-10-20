// Mise à jour de l'application de bureau, dans la barre d'état (décisions de
// Louis du 20/10/2026, docs/conception/mise-a-jour-automatique.md) : la
// nouvelle version proposée, son téléchargement, « Redémarrez… », puis
// « Mise à jour effectuée » au premier lancement de la nouvelle version.
// L'état vient du moteur (UpdateService.Status, évènement update:status) ;
// ce module en tire le texte et l'action du clic.
import { t } from '#lib/i18n/index.js';

/** État rendu par UpdateService (internal/update.Status). */
export interface UpdateState {
	phase: string;
	version?: string;
	page?: string;
	elevation?: boolean;
	error?: string;
	updated?: string;
	updatedPage?: string;
}

/**
 * Action du clic : préparer (télécharger et vérifier), ouvrir la page de la
 * version, redémarrer pour appliquer, ouvrir les notes de version.
 */
export type UpdateAction = 'prepare' | 'open' | 'restart' | 'notes' | null;

export interface UpdateView {
	text: string;
	title: string;
	action: UpdateAction;
	/** Page à ouvrir ('open', 'notes'). */
	url?: string;
	/** Téléchargement en cours (indicateur, pas de clic). */
	busy?: boolean;
	/** Échec : montré en avertissement. */
	warn?: boolean;
	/** L'avis se ferme (« Mise à jour effectuée »). */
	dismissible?: boolean;
}

/** Ce que montre la barre d'état pour un état de la mise à jour (null : rien). */
export function updateView(s: UpdateState | null | undefined): UpdateView | null {
	if (!s) return null;
	if (s.updated) {
		return {
			text: t('shell.update.updated'),
			title: t('shell.update.updatedTooltip', { version: s.updated }),
			action: 'notes',
			url: s.updatedPage,
			dismissible: true
		};
	}
	const version = s.version ?? '';
	switch (s.phase) {
		case 'available':
			return {
				text: t('shell.update.available'),
				title: s.elevation ? t('shell.update.availableElevatedTooltip', { version }) : t('shell.update.availableTooltip', { version }),
				action: 'prepare'
			};
		case 'manual':
			return { text: t('shell.update.manual'), title: t('shell.update.manualTooltip', { version }), action: 'open', url: s.page };
		case 'downloading':
			return { text: t('shell.update.downloading'), title: t('shell.update.downloadingTooltip', { version }), action: null, busy: true };
		case 'ready':
			return {
				text: s.elevation ? t('shell.update.readyElevated') : t('shell.update.ready'),
				title: s.error || t('shell.update.readyTooltip', { version }),
				action: 'restart',
				warn: !!s.error
			};
		case 'failed':
			return { text: t('shell.update.failed'), title: s.error ?? '', action: 'prepare', warn: true };
	}
	return null;
}
