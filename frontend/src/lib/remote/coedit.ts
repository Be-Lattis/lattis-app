// Travail à plusieurs sur un classeur d'un dossier partagé (NAS, décision
// 0009) : textes de l'indicateur de présence, sans composant (testés par
// vitest). L'état vient de DocumentService.CoEditState (internal/coedit.State).
import type { Peer, State } from '#bindings/internal/coedit/models.js';
import { fmtTime, t } from '#lib/i18n/index.js';

/** Nom d'une session : « Alice (PC-COMPTA) ». */
export function peerName(p: Peer): string {
	const who = p.user || t('remote.coedit.otherUser');
	return p.machine ? `${who} (${p.machine})` : who;
}

/** Texte de l'indicateur : vide si le classeur n'est pas en travail à plusieurs. */
export function presenceText(st: State | null | undefined): string {
	if (!st?.active) return '';
	const peers = (st.peers ?? []).filter((p) => !p.joining);
	if (!peers.length) return t('remote.coedit.alone');
	const names = peers.map(peerName);
	if (names.length <= 3) return t('remote.coedit.with', { names: names.join(', ') });
	return t('remote.coedit.withMore', { names: names.slice(0, 2).join(', '), n: names.length - 2 });
}

/** Infobulle de l'indicateur : chaque session et depuis quand. */
export function presenceTitle(st: State | null | undefined): string {
	if (!st?.active) return '';
	const lines = [t('remote.coedit.title')];
	for (const p of st.peers ?? []) {
		const since = p.since ? new Date(p.since) : null;
		const at = since && !isNaN(since.getTime()) ? t('remote.coedit.since', { time: fmtTime(since, { hour: '2-digit', minute: '2-digit' }) }) : '';
		lines.push(`• ${peerName(p)}${p.joining ? t('remote.coedit.joining') : at}`);
	}
	return lines.join('\n');
}

/** Message d'avertissement (dossier injoignable, état local à rouvrir), vide sinon. */
export function presenceWarning(st: State | null | undefined): string {
	if (!st?.active) return '';
	return st.stale || st.offline || '';
}
