// Nom du classeur ouvert à l'écran (barre de titre, fenêtre du système,
// récents, messages) : pour un classeur ouvert depuis un Serveur Lattis,
// son titre sur le serveur et « en ligne sur <serveur> », jamais son cache
// local (<cache>/partages/<hôte>/<identifiant>.lattis). DocState.remote
// vient de internal/bridge (remotedoc.go).
import type { DocState, RecentFile } from '#bindings/internal/bridge/models.js';
import { hostOf } from '#lib/remote/lattis.js';

/** Ce qu'il faut savoir du serveur d'un classeur. */
interface ServerRef {
	server?: string;
	serverName?: string;
}

/** Nom du serveur, sinon son hôte. */
export function serverLabel(r: ServerRef | null | undefined): string {
	if (!r) return '';
	return r.serverName?.trim() || (r.server ? hostOf(r.server) : '');
}

/** Dernier élément d'un chemin. */
export function baseName(path: string): string {
	return path.split(/[\\/]/).pop() ?? '';
}

/** Badge de la barre de titre : « en ligne sur LPO », « hors ligne — LPO » ; vide pour un fichier local. */
export function remoteBadge(st: DocState | null | undefined): string {
	const r = st?.remote;
	if (!r) return '';
	const where = serverLabel(r);
	return r.online ? `en ligne sur ${where}` : `hors ligne — ${where}`;
}

/**
 * Badge d'un classeur partagé, d'après son état de partage : aussi pour le
 * fichier de l'auteur d'un classeur publié, qui n'est pas un cache mais se
 * synchronise de même. Vide pour un classeur local.
 */
export function shareBadge(s: (ServerRef & { shared?: boolean; online?: boolean; gone?: string }) | null | undefined): string {
	if (!s?.shared || s.gone === 'deleted') return '';
	const where = serverLabel(s);
	return s.online ? `en ligne sur ${where}` : `hors ligne — ${where}`;
}

/** Titre de la fenêtre du système (barre des tâches) : « Fichier nids — en ligne sur LPO – Lattis ». */
export function windowTitle(title: string, badge = ''): string {
	if (!title) return 'Lattis';
	return `${title}${badge ? ` — ${badge}` : ''} – Lattis`;
}

/** Nom du fichier du classeur pour les propositions de nom (regroupement d'onglets…) : titre du serveur, sinon chemin, sinon titre. */
export function docFile(st: DocState): string {
	return st.remote ? st.title : st.path || st.title;
}

/** Nom de fichier affiché (mise en page : &[Fichier]) : titre d'un classeur du serveur, sinon nom du fichier. */
export function docFileName(st: DocState): string {
	return st.remote ? st.title : st.path ? baseName(st.path) : st.title;
}

/** Message après Ctrl+S : celui du moteur (classeur du serveur : ce qui est arrivé au serveur), sinon le fichier écrit. */
export function savedNotice(st: DocState): string {
	if (st.notice) return st.notice;
	if (st.remote) return `Enregistré sur ${serverLabel(st.remote)} : ${st.title}`;
	return `Enregistré : ${st.path}`;
}

/** Classeur du serveur dans les récents. */
export function recentRemote(r: RecentFile): boolean {
	return !!r.workbookId;
}

/** Clé d'un récent (liste Svelte). */
export function recentKey(r: RecentFile): string {
	return recentRemote(r) ? `${r.server}\u0000${r.workbookId}` : r.path;
}

/** Nom d'un récent : titre d'un classeur du serveur, nom d'un fichier local. */
export function recentLabel(r: RecentFile): string {
	return recentRemote(r) ? r.title || 'Classeur partagé' : baseName(r.path);
}

/** Infobulle d'un récent : « En ligne sur LPO : https://…/w/<id> », ou le chemin du fichier. */
export function recentDetail(r: RecentFile): string {
	return recentRemote(r) ? `En ligne sur ${serverLabel(r)}${r.link ? ` : ${r.link}` : ''}` : r.path;
}
