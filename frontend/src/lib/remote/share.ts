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

export interface Banner {
	kind: BannerKind;
	text: string;
	/** Bouton du bandeau. */
	action?: 'reload' | 'login' | 'conflicts';
	actionLabel?: string;
}

/** Bandeau du classeur partagé ouvert (null : rien à dire). */
export function banner(s: State | null | undefined): Banner | null {
	if (!s?.shared) return null;
	if (!s.online) {
		const login = /connexion requise|connectez-vous|se connecter|session expirée/i.test(s.offline ?? '');
		const waiting = s.pending ? ` — ${s.pending} modification${s.pending > 1 ? 's' : ''} en attente d'envoi` : '';
		return {
			kind: 'offline',
			text: 'Hors ligne, lecture seule' + (s.offline ? ` (${s.offline})` : '') + waiting,
			...(login ? { action: 'login' as const, actionLabel: 'Se connecter…' } : {})
		};
	}
	if (s.newVersion) return { kind: 'new', text: `Nouvelle version disponible (version ${s.newVersion})`, action: 'reload', actionLabel: 'Recharger' };
	if (s.conflicts) {
		const n = s.conflicts;
		return { kind: 'conflict', text: `${n} conflit${n > 1 ? 's' : ''} : cellule${n > 1 ? 's' : ''} modifiée${n > 1 ? 's' : ''} par un collègue entre-temps`, action: 'conflicts', actionLabel: 'Voir' };
	}
	if (s.readOnly) return { kind: 'readonly', text: `${s.readOnly} — version ${s.version}${s.serverName ? ` sur ${s.serverName}` : ''}` };
	return null;
}

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
