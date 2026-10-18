// Classeurs partagés (Fichier › Partager sur un serveur…, Ouvrir depuis un
// serveur, Republier) : logique des boîtes et du bandeau, sans composant
// (testée par vitest). L'état vient de DocumentService.ShareState
// (internal/remote/workbook.State), les vérifications de ShareCheck.
import type { Check, State, TableCheck } from '#bindings/internal/remote/workbook/models.js';

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

export type BannerKind = 'new' | 'offline' | 'readonly';

export interface Banner {
	kind: BannerKind;
	text: string;
	/** Bouton du bandeau. */
	action?: 'reload' | 'login';
	actionLabel?: string;
}

/** Bandeau du classeur partagé ouvert (null : rien à dire). */
export function banner(s: State | null | undefined): Banner | null {
	if (!s?.shared) return null;
	if (!s.online) {
		const login = /connexion requise|connectez-vous|se connecter|session expirée/i.test(s.offline ?? '');
		return {
			kind: 'offline',
			text: 'Hors ligne, lecture seule' + (s.offline ? ` (${s.offline})` : ''),
			...(login ? { action: 'login' as const, actionLabel: 'Se connecter…' } : {})
		};
	}
	if (s.newVersion) return { kind: 'new', text: `Nouvelle version disponible (version ${s.newVersion})`, action: 'reload', actionLabel: 'Recharger' };
	if (s.readOnly) return { kind: 'readonly', text: `${s.readOnly} — version ${s.version}${s.serverName ? ` sur ${s.serverName}` : ''}` };
	return null;
}

/** « Fichier › Republier » : classeur figé publié, propriétaire ou concepteur. */
export function canRepublish(s: State | null | undefined): boolean {
	return !!s?.shared && s.mode === FROZEN && s.canDesign;
}
