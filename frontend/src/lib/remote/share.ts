// Classeurs partagés (Fichier › Publier sur un serveur…, Ouvrir depuis un
// serveur, Republier) : logique des boîtes et du bandeau, sans composant
// (testée par vitest). L'état vient de DocumentService.ShareState
// (internal/remote/workbook.State), les vérifications de ShareCheck.
import type { Check, State, TableCheck } from '#bindings/internal/remote/workbook/models.js';
import type { Conflict } from '#bindings/internal/remote/collab/models.js';
import { formatValue, type ColumnType } from '#lib/grid/format.js';

export const FROZEN = 'frozen';
export const COLLABORATIVE = 'collaborative';

/** Libellé d'un mode de partage. */
export function modeLabel(mode: string): string {
	return mode === FROZEN ? 'Figé' : mode === COLLABORATIVE ? 'Collaboratif' : mode;
}

const ROLES: Record<string, string> = { owner: 'propriétaire', designer: 'concepteur', editor: 'éditeur', participant: 'participant', reader: 'lecteur' };

/** Rôles sur un classeur, en clair (« propriétaire, lecteur »). */
export function rolesLabel(roles: readonly string[] | null | undefined): string {
	return (roles ?? []).map((r) => ROLES[r] ?? r).join(', ');
}

const TYPES: Record<string, string> = { text: 'Texte', integer: 'Entier', decimal: 'Décimal', date: 'Date', datetime: 'Date et heure', boolean: 'Booléen' };

/** Libellé d'un type du serveur. */
export function remoteTypeLabel(t: string): string {
	return TYPES[t] ?? t;
}

/** Tableaux cochés d'avance : déjà publiés (republication), sinon ceux qui se publient tels quels. */
export function defaultSelection(check: Check | null, republish: boolean): number[] {
	const tables = check?.tables ?? [];
	if (republish) return tables.filter((t) => t.published).map((t) => t.id);
	return tables.filter((t) => !(t.problems ?? []).length && !t.invalid).map((t) => t.id);
}

/**
 * Ce qui empêche de publier les tableaux choisis (même règles que
 * workbook.Check.Blocking côté Go) ; vide : rien.
 */
export function blocking(check: Check | null, ids: readonly number[]): string[] {
	const out: string[] = [];
	if (!ids.length) return ['Cochez au moins un tableau à publier.'];
	const seen = new Map<string, string>();
	for (const t of check?.tables ?? []) {
		if (!ids.includes(t.id)) continue;
		for (const p of t.problems ?? []) out.push(`« ${t.name} » : ${p}`);
		if (t.invalid) out.push(`« ${t.name} » : ${t.invalid} saisie(s) invalide(s) à corriger, ou colonne à accepter comme texte`);
		const k = t.name.trim().toLowerCase();
		const other = seen.get(k);
		if (other !== undefined) out.push(`« ${other} » et « ${t.name} » : même nom sur le serveur (la casse n'y compte pas)`);
		seen.set(k, t.name);
	}
	return out;
}

/** Colonnes dont le type « élargissable » sera figé par la publication (§9). */
export function inferredColumns(t: TableCheck): string[] {
	return (t.columns ?? []).filter((c) => c.inferred && !c.computed).map((c) => `${c.name} (${remoteTypeLabel(c.remote)})`);
}

export type BannerKind = 'new' | 'offline' | 'readonly' | 'conflict';

/** Bouton d'un bandeau. */
export type BannerAction = 'reload' | 'login' | 'conflicts' | 'retry' | 'copy' | 'detach';

export interface Banner {
	kind: BannerKind;
	text: string;
	/** Détail technique (infobulle, « Détails ») : l'erreur de connexion telle quelle. */
	detail?: string;
	/** Boutons du bandeau, dans l'ordre. */
	actions?: { action: BannerAction; label: string }[];
}

/** Hôte d'une adresse (« localhost:8080 »), sinon l'adresse. */
function hostOf(url: string): string {
	try {
		return new URL(url).host || url;
	} catch {
		return url;
	}
}

/** Nom du serveur d'un classeur partagé : son nom, sinon son hôte. */
export function serverLabel(s: Pick<State, 'server' | 'serverName'>): string {
	return s.serverName?.trim() || hostOf(s.server ?? '');
}

/** Coupure due à une session à rouvrir (plutôt qu'à un serveur arrêté). */
function needsLogin(reason: string | undefined): boolean {
	return /connexion requise|connectez-vous|se connecter|session expirée/i.test(reason ?? '');
}

/**
 * Texte du bandeau hors ligne : lisible, sans l'erreur technique (dans
 * Banner.detail). Classeur collaboratif : modifiable hors ligne, la file
 * part au retour du serveur (décision 0014) ; figé : lecture seule.
 */
export function offlineText(s: State): string {
	const where = serverLabel(s);
	const n = s.pending ?? 0;
	const waiting = n ? ` (${n} modification${n > 1 ? 's' : ''} en attente)` : '';
	const why = needsLogin(s.offline) ? `Connexion à ${where} requise` : `Le serveur ${where} ne répond pas`;
	if (s.mode === COLLABORATIVE) return `${why}. Hors ligne : vos modifications seront envoyées au retour du serveur${waiting}.`;
	return `${why} : ce classeur partagé est en lecture seule.`;
}

/** Bandeau du classeur partagé ouvert (null : rien à dire). */
export function banner(s: State | null | undefined): Banner | null {
	if (!s?.shared) return null;
	if (!s.online) {
		const actions: NonNullable<Banner['actions']> = [
			needsLogin(s.offline) ? { action: 'login', label: 'Se connecter…' } : { action: 'retry', label: 'Réessayer' },
			{ action: 'copy', label: 'Travailler sur une copie locale' }
		];
		if (s.canManage) actions.push({ action: 'detach', label: 'Détacher du serveur…' });
		return { kind: 'offline', text: offlineText(s), ...(s.offline ? { detail: s.offline } : {}), actions };
	}
	if (s.newVersion) return { kind: 'new', text: `Nouvelle version disponible (version ${s.newVersion})`, actions: [{ action: 'reload', label: 'Recharger' }] };
	if (s.conflicts) {
		const n = s.conflicts;
		return { kind: 'conflict', text: `${n} conflit${n > 1 ? 's' : ''} : cellule${n > 1 ? 's' : ''} modifiée${n > 1 ? 's' : ''} par un collègue entre-temps`, actions: [{ action: 'conflicts', label: 'Voir' }] };
	}
	if (s.readOnly) return { kind: 'readonly', text: `${s.readOnly} — version ${s.version}${s.serverName ? ` sur ${s.serverName}` : ''}` };
	return null;
}

/**
 * Question avant « Détacher du serveur » : ce qui se passe, en clair.
 * cached : classeur ouvert depuis le serveur (sa copie est dans le cache :
 * il faudra choisir où l'enregistrer).
 */
export function detachQuestion(s: State, cached: boolean): string[] {
	const where = serverLabel(s);
	const out = [
		`« ${s.title} » deviendra un classeur local ordinaire : il ne se synchronisera plus avec ${where}.`,
		'Le classeur du serveur n’est pas modifié : les autres membres le gardent tel quel, et vous pourrez le rouvrir depuis le serveur.'
	];
	const n = s.pending ?? 0;
	if (n) out.push(`${n} modification${n > 1 ? 's' : ''} pas encore envoyée${n > 1 ? 's' : ''} au serveur ${n > 1 ? 'resteront' : 'restera'} dans ce classeur local, sans partir.`);
	if (cached) out.push('Vous allez choisir où enregistrer le classeur local (sa copie actuelle n’est qu’un cache de Lattis).');
	out.push('Ce détachement ne s’annule pas.');
	return out;
}

/** Message de la barre d'état après « Réessayer ». */
export const RETRY_NOTICE = 'Nouvel essai de connexion…';

/** Heure (aujourd'hui) ou date et heure d'une modification, en français ; "" si inconnue. */
export function whenLabel(at: string, now = new Date()): string {
	if (!at) return '';
	const d = new Date(at);
	if (Number.isNaN(d.getTime())) return '';
	const hm = d.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
	if (d.toDateString() === now.toDateString()) return `à ${hm}`;
	return `le ${d.toLocaleDateString('fr-FR')} à ${hm}`;
}

/** Valeur d'un conflit telle que la grille l'afficherait (« (vide) » si vide). */
export function conflictValue(c: Pick<Conflict, 'type'>, v: unknown): string {
	if (v === null || v === undefined || v === '') return '(vide)';
	return formatValue(c.type as ColumnType, v);
}

/** « Modifié par Alice à 10:42 : 1 100 (vous : 1 200) ». */
export function conflictText(c: Conflict, now = new Date()): string {
	const who = c.author ? ` par ${c.author}` : '';
	const when = whenLabel(c.at, now);
	return `Modifié${who}${when ? ' ' + when : ''} : ${conflictValue(c, c.theirs)} (vous : ${conflictValue(c, c.mine)})`;
}

/**
 * Collage dans un tableau partagé : valeurs écartées car hors du type figé
 * de leur colonne (§9), le reste est collé.
 */
export function refusedNotice(ev: { refused?: string[] | null; refusedCount?: number }): string {
	const n = ev.refusedCount ?? 0;
	if (!n) return '';
	const list = (ev.refused ?? []).join(' · ');
	const more = n > (ev.refused?.length ?? 0) ? ' …' : '';
	return `${n} valeur${n > 1 ? 's' : ''} hors du type figé non collée${n > 1 ? 's' : ''} (tableau partagé) : ${list}${more}`;
}

/** « Fichier › Republier » : classeur figé publié, propriétaire ou concepteur. */
export function canRepublish(s: State | null | undefined): boolean {
	return !!s?.shared && s.mode === FROZEN && s.canDesign;
}
