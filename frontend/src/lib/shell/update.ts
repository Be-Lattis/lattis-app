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
const OFFERED = new Set(['available', 'manual', 'downloading', 'ready', 'failed']);

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
