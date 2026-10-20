// Signaler un problème : nouvelle issue GitHub du dépôt de l'application,
// avec la version préremplie (menu Fichier et volet d'aide).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import { openOutside } from '#lib/ai/links.js';
import { t } from '#lib/i18n/index.js';

export const ISSUES_URL = 'https://github.com/Be-Lattis/lattis-app/issues/new';

/** Corps proposé pour une nouvelle issue. */
export function issueBody(version: string): string {
	return t('shell.report.issueBody', { version });
}

/** Ouvre la page de création d'une issue dans le navigateur. */
export async function reportIssue() {
	const v = (await DocumentService.About().catch(() => null))?.version ?? '?';
	openOutside(`${ISSUES_URL}?body=${encodeURIComponent(issueBody(v))}`);
}
