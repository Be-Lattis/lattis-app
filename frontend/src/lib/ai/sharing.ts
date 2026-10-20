// Partage des données avec l'assistant (réglage ai.dataSharing, Fichier ›
// Paramètres › Assistant IA) : il vaut pour l'assistant intégré comme pour
// Claude Desktop ou Claude Code par le serveur MCP (internal/sharing).
import { t } from '#lib/i18n/index.js';

/** Libellé court du partage (pied du volet, configuration de Claude). */
export function sharingLabel(mode: unknown): string {
	switch (mode) {
		case 'sample':
			return t('ai.sharing.label.sample');
		case 'full':
			return t('ai.sharing.label.full');
	}
	return t('ai.sharing.label.schema');
}

/** Ce que voit l'assistant avec ce partage, en une phrase. */
export function sharingSummary(mode: unknown): string {
	switch (mode) {
		case 'sample':
			return t('ai.sharing.summary.sample');
		case 'full':
			return t('ai.sharing.summary.full');
	}
	return t('ai.sharing.summary.schema');
}
