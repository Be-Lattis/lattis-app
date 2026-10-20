// Travail à plusieurs sur un classeur d'un dossier partagé (NAS, décision
// 0009) : textes de l'indicateur de présence, sans composant (testés par
// vitest). L'état vient de DocumentService.CoEditState (internal/coedit.State).
import type { Peer, State } from '#bindings/internal/coedit/models.js';
import { fmtTime } from '#lib/i18n/index.js';

/** Nom d'une session : « Alice (PC-COMPTA) ». */
export function peerName(p: Peer): string {
	const who = p.user || 'un autre utilisateur';
	return p.machine ? `${who} (${p.machine})` : who;
}

/** Texte de l'indicateur : vide si le classeur n'est pas en travail à plusieurs. */
export function presenceText(st: State | null | undefined): string {
	if (!st?.active) return '';
	const peers = (st.peers ?? []).filter((p) => !p.joining);
	if (!peers.length) return 'À plusieurs : vous seul pour le moment';
	const names = peers.map(peerName);
	if (names.length <= 3) return `À plusieurs avec ${names.join(', ')}`;
	return `À plusieurs avec ${names.slice(0, 2).join(', ')} et ${names.length - 2} autres`;
}

/** Infobulle de l'indicateur : chaque session et depuis quand. */
export function presenceTitle(st: State | null | undefined): string {
	if (!st?.active) return '';
	const lines = ["Travail à plusieurs sur ce classeur (dossier partagé) : les données des tableaux se modifient ensemble, le reste attend que vous soyez seul."];
	for (const p of st.peers ?? []) {
		const since = p.since ? new Date(p.since) : null;
		const at = since && !isNaN(since.getTime()) ? ` depuis ${fmtTime(since, { hour: '2-digit', minute: '2-digit' })}` : '';
		lines.push(`• ${peerName(p)}${p.joining ? ' (en train de rejoindre)' : at}`);
	}
	return lines.join('\n');
}

/** Message d'avertissement (dossier injoignable, état local à rouvrir), vide sinon. */
export function presenceWarning(st: State | null | undefined): string {
	if (!st?.active) return '';
	return st.stale || st.offline || '';
}
