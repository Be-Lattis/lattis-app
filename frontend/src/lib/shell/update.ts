// Mise à jour de l'application de bureau, dans la barre d'état (décisions de
// Louis du 20/10/2026, docs/conception/mise-a-jour-automatique.md) : la
// nouvelle version proposée, son téléchargement, « Redémarrez… », puis
// « Mise à jour effectuée » au premier lancement de la nouvelle version.
// L'état vient du moteur (UpdateService.Status, évènement update:status) ;
// ce module en tire le texte et l'action du clic.
import { fmtDate, fmtTime, t } from '#lib/i18n/index.js';

/** État rendu par UpdateService (internal/update.Status). */
export interface UpdateState {
	phase: string;
	version?: string;
	page?: string;
	elevation?: boolean;
	error?: string;
	updated?: string;
	updatedPage?: string;
	/** L'installation a échoué (phase 'installFailed') : nombre d'échecs. */
	failures?: number;
	/** Plus d'installation automatique à la fermeture (trop d'échecs). */
	autoStopped?: boolean;
	/** L'installeur peut être lancé avec son interface (sinon : la page). */
	installer?: boolean;
	/** Paramètres › Mises à jour : version installée. */
	current?: string;
	/** Dernière recherche (RFC 3339) ; absente : jamais. */
	lastCheck?: string;
	/** Échec de la dernière recherche, dans la langue de la session. */
	lastError?: string;
	/** Recherche en cours. */
	checking?: boolean;
	/** Recherche impossible : 'dev', 'msix', 'policy', 'off'. */
	unavailable?: string;
	/** Une seconde installation de Lattis sur le poste (absente : une seule). */
	duplicate?: DuplicateState | null;
}

/** L'autre installation (internal/update.Duplicate). */
export interface DuplicateState {
	/** Sa portée : 'user' (pour le compte) ou 'machine' (pour tous). */
	scope: string;
	dir: string;
	version?: string;
	/** Son retrait passe par l'invite UAC. */
	elevation?: boolean;
	/** Retrait en cours. */
	removing?: boolean;
	/** Échec du dernier retrait, dans la langue de la session. */
	error?: string;
}

/** Avis « Deux installations de Lattis sur ce poste — Retirer l'ancienne ». */
export interface DuplicateView {
	text: string;
	title: string;
	/** Retrait en cours (indicateur, pas de clic). */
	busy?: boolean;
	/** Échec du retrait : avertissement, le clic réessaie. */
	warn?: boolean;
}

/**
 * Ce que montre la barre d'état quand deux installations de Lattis sont sur
 * le poste (null : une seule). Le clic retire l'autre
 * (UpdateService.RemoveDuplicate).
 */
export function duplicateView(s: UpdateState | null | undefined): DuplicateView | null {
	const d = s?.duplicate;
	if (!d) return null;
	if (d.removing) return { text: t('shell.update.duplicate.removing'), title: d.dir, busy: true };
	const args = { version: d.version || '?', dir: d.dir };
	const tip = d.scope === 'machine' ? t('shell.update.duplicate.machineTooltip', args) : t('shell.update.duplicate.userTooltip', args);
	if (d.error) return { text: t('shell.update.duplicate.failed'), title: `${d.error} — ${tip}`, warn: true };
	return { text: t('shell.update.duplicate.text'), title: tip };
}

/** « aujourd'hui 23:08 », « hier 09:12 », « 18/10/2026 14:05 ». */
export function checkWhen(iso: string | undefined, now = Date.now()): string {
	const d = iso ? new Date(iso) : null;
	if (!d || isNaN(d.getTime())) return t('shell.update.check.never');
	const hm = fmtTime(d, { hour: '2-digit', minute: '2-digit' });
	const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
	const days = Math.round((day(new Date(now)) - day(d)) / 86400000);
	if (days === 0) return t('shell.update.check.today', { time: hm });
	if (days === 1) return t('shell.update.check.yesterday', { time: hm });
	return `${fmtDate(d)} ${hm}`;
}

/** Une version plus récente est proposée (ou se prépare). */
const OFFERED = new Set(['available', 'manual', 'downloading', 'ready', 'failed', 'installFailed']);

/**
 * Ligne de Paramètres › Mises à jour : « Version 0.3.2-beta · dernière
 * recherche : aujourd'hui 23:08 — à jour » (ou « — 0.3.3-beta disponible »,
 * « — échec : … »), ou la raison qui empêche la recherche.
 */
export function checkLine(s: UpdateState | null | undefined, now = Date.now()): string {
	if (!s) return '';
	const version = s.current || '?';
	switch (s.unavailable) {
		case 'dev':
			return t('shell.update.check.line', { version, detail: t('shell.update.check.dev') });
		case 'msix':
			return t('shell.update.check.line', { version, detail: t('shell.update.check.msix') });
		case 'policy':
			return t('shell.update.check.line', { version, detail: t('shell.update.check.policy') });
		case 'off':
			return t('shell.update.check.line', { version, detail: t('shell.update.check.off') });
	}
	let detail = t('shell.update.check.last', { when: checkWhen(s.lastCheck, now) });
	if (s.lastError) detail += ' — ' + t('shell.update.check.failed', { error: s.lastError });
	else if (s.version && OFFERED.has(s.phase)) detail += ' — ' + t('shell.update.check.found', { version: s.version });
	else if (s.lastCheck) detail += ' — ' + t('shell.update.check.upToDate');
	return t('shell.update.check.line', { version, detail });
}

/**
 * Action du clic : préparer (télécharger et vérifier), ouvrir la page de la
 * version, redémarrer pour appliquer, ouvrir les notes de version, lancer
 * l'installeur avec son interface (l'installation à la fermeture a échoué).
 */
export type UpdateAction = 'prepare' | 'open' | 'restart' | 'notes' | 'install' | null;

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
	/** Second lien, vers une page (la page de la version). */
	link?: { text: string; url: string };
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
		case 'installFailed': {
			// Plus de « Redémarrez… » : l'installation n'a pas abouti.
			const reason = s.error || '?';
			if (!s.installer) {
				return {
					text: t('shell.update.installFailedPage', { version }),
					title: t('shell.update.installFailedPageTooltip', { reason }),
					action: 'open',
					url: s.page,
					warn: true
				};
			}
			return {
				text: t('shell.update.installFailed', { version }),
				title: s.autoStopped
					? t('shell.update.installFailedStopped', { reason, count: s.failures ?? 0 })
					: t('shell.update.installFailedTooltip', { reason }),
				action: 'install',
				warn: true,
				link: s.page ? { text: t('shell.update.releasePage'), url: s.page } : undefined
			};
		}
	}
	return null;
}
