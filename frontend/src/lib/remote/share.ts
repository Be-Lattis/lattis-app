// Classeurs partagés (Fichier › Publier sur un serveur…, Ouvrir depuis un
// serveur, Republier) : logique des boîtes et du bandeau, sans composant
// (testée par vitest). L'état vient de DocumentService.ShareState
// (internal/remote/workbook.State), les vérifications de ShareCheck.
import type { Check, State, TableCheck } from '#bindings/internal/remote/workbook/models.js';
import type { Conflict } from '#bindings/internal/remote/collab/models.js';
import { formatValue, type ColumnType } from '#lib/grid/format.js';
import { fmtDate, fmtTime, t, tAny } from '#lib/i18n/index.js';

export const FROZEN = 'frozen';
export const COLLABORATIVE = 'collaborative';

/** Libellé d'un mode de partage. */
export function modeLabel(mode: string): string {
	return mode === FROZEN ? t('remote.share.mode.frozen') : mode === COLLABORATIVE ? t('remote.share.mode.collaborative') : mode;
}

/** Rôles fournis (pastille du rôle, spécification du serveur §8.1). */
const ROLE_IDS = new Set(['owner', 'designer', 'editor', 'participant', 'reader']);

/** Nom d'un rôle fourni, en minuscules (« propriétaire »). */
function roleName(r: string): string {
	return ROLE_IDS.has(r) ? tAny(`remote.share.role.${r}`) : r;
}

/** Ce que permet chaque rôle fourni. */
function roleDetail(r: string): string {
	return tAny(`remote.share.roleDetail.${r}`);
}

/** Rôles du plus large au plus étroit (la pastille montre le plus large). */
const ROLE_ORDER = ['owner', 'designer', 'editor', 'participant', 'reader'];

/**
 * Pastille du rôle dans la barre de titre : le rôle le plus large de
 * l'utilisateur sur le classeur partagé ouvert, et ce qu'il permet (infobulle,
 * et message au toucher). null : classeur local.
 */
export function roleChip(s: State | null | undefined): { label: string; detail: string } | null {
	if (!s?.shared || s.gone) return null;
	const roles = s.roles ?? [];
	const top = ROLE_ORDER.find((r) => roles.includes(r));
	if (!top) return { label: t('remote.share.roleChip.custom'), detail: t('remote.share.roleChip.customDetail') };
	const name = roleName(top);
	const label = name.charAt(0).toUpperCase() + name.slice(1);
	const frozen = s.mode === FROZEN && top !== 'owner' && top !== 'designer' ? t('remote.share.roleChip.frozen') : '';
	const more = roles.length > 1 ? t('remote.share.roleChip.more', { roles: rolesLabel(roles) }) : '';
	return { label, detail: t('remote.share.roleChip.detail', { role: name, detail: roleDetail(top), frozen, more }) };
}

/** Rôles sur un classeur, en clair (« propriétaire, lecteur »). */
export function rolesLabel(roles: readonly string[] | null | undefined): string {
	return (roles ?? []).map(roleName).join(', ');
}

const TYPES = new Set(['text', 'integer', 'decimal', 'date', 'datetime', 'boolean']);

/** Libellé d'un type du serveur. */
export function remoteTypeLabel(type: string): string {
	return TYPES.has(type) ? tAny(`remote.share.type.${type}`) : type;
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
	if (!ids.length) return [t('remote.share.blocking.none')];
	const seen = new Map<string, string>();
	for (const tb of check?.tables ?? []) {
		if (!ids.includes(tb.id)) continue;
		for (const p of tb.problems ?? []) out.push(t('remote.share.blocking.problem', { table: tb.name, problem: p }));
		if (tb.invalid) out.push(t('remote.share.blocking.invalid', { table: tb.name, n: tb.invalid }));
		const k = tb.name.trim().toLowerCase();
		const other = seen.get(k);
		if (other !== undefined) out.push(t('remote.share.blocking.sameName', { first: other, second: tb.name }));
		seen.set(k, tb.name);
	}
	return out;
}

/** Colonnes dont le type « élargissable » sera figé par la publication (§9). */
export function inferredColumns(tb: TableCheck): string[] {
	return (tb.columns ?? []).filter((c) => c.inferred && !c.computed).map((c) => `${c.name} (${remoteTypeLabel(c.remote)})`);
}

export type BannerKind = 'new' | 'offline' | 'gone' | 'readonly' | 'conflict';

/** Bouton d'un bandeau. */
export type BannerAction = 'reload' | 'login' | 'conflicts' | 'retry' | 'copy' | 'detach' | 'forget';

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
	const pending = n ? t('remote.share.offline.pending', { n }) : '';
	const why = needsLogin(s.offline) ? t('remote.share.offline.loginRequired', { server: where }) : serverDown(where);
	if (s.mode === COLLABORATIVE) return t('remote.share.offline.collaborative', { why, pending });
	return t('remote.share.offline.readOnly', { why });
}

/**
 * « Le serveur localhost:8080 ne répond pas » ; un nom qui commence déjà
 * par « Serveur » ne le répète pas : « Serveur Lattis ne répond pas ».
 */
export function serverDown(where: string): string {
	return /^serveur\b/i.test(where) ? t('remote.share.serverDown.named', { server: where }) : t('remote.share.serverDown.host', { server: where });
}

/** Classeur supprimé du serveur (State.gone) ; l'utilisateur n'y a plus accès. */
export const GONE_DELETED = 'deleted';
export const GONE_REVOKED = 'revoked';

/**
 * Texte du bandeau d'un classeur disparu du serveur (supprimé, ou plus
 * d'accès) : à ne pas confondre avec un serveur qui ne répond pas. Mêmes
 * mots que l'erreur d'ouverture sans cache (bridge.goneError).
 */
export function goneText(s: State): string {
	const where = serverLabel(s);
	const what = s.gone === GONE_REVOKED ? t('remote.share.gone.revoked', { server: where }) : t('remote.share.gone.deleted', { server: where });
	const n = s.pending ?? 0;
	if (!n) return what;
	return t('remote.share.gone.pending', { what, n });
}

/** Erreur d'ouverture d'un classeur disparu du serveur (bridge.goneError) plutôt que d'une coupure. */
export function goneError(msg: string): boolean {
	return /a été supprimé de |n['’]avez plus accès à ce classeur/.test(msg);
}

/** Bandeau du classeur partagé ouvert (null : rien à dire). */
export function banner(s: State | null | undefined): Banner | null {
	if (!s?.shared) return null;
	if (s.gone) {
		return {
			kind: 'gone',
			text: goneText(s),
			actions: [
				{ action: 'copy', label: t('remote.share.banner.saveCopy') },
				{ action: 'forget', label: t('remote.share.banner.forget') }
			]
		};
	}
	if (!s.online) {
		const actions: NonNullable<Banner['actions']> = [
			needsLogin(s.offline) ? { action: 'login', label: t('remote.share.banner.login') } : { action: 'retry', label: t('remote.share.banner.retry') },
			{ action: 'copy', label: t('remote.share.banner.workOnCopy') }
		];
		if (s.canManage) actions.push({ action: 'detach', label: t('remote.share.banner.detach') });
		return { kind: 'offline', text: offlineText(s), ...(s.offline ? { detail: s.offline } : {}), actions };
	}
	if (s.newVersion) return { kind: 'new', text: t('remote.share.banner.newVersion', { version: String(s.newVersion) }), actions: [{ action: 'reload', label: t('remote.share.banner.reload') }] };
	if (s.held) {
		return { kind: 'conflict', text: t('remote.share.banner.held', { n: s.held }), actions: [{ action: 'conflicts', label: t('remote.share.banner.view') }] };
	}
	if (s.conflicts) {
		return { kind: 'conflict', text: t('remote.share.banner.conflicts', { n: s.conflicts }), actions: [{ action: 'conflicts', label: t('remote.share.banner.view') }] };
	}
	if (s.readOnly)
		return {
			kind: 'readonly',
			text: s.serverName ? t('remote.share.banner.readOnlyOn', { reason: s.readOnly, version: String(s.version), server: s.serverName }) : t('remote.share.banner.readOnly', { reason: s.readOnly, version: String(s.version) })
		};
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
		t('remote.share.detach.local', { title: s.title, server: where }),
		t('remote.share.detach.serverKept')
	];
	const n = s.pending ?? 0;
	if (n) out.push(t('remote.share.detach.pending', { n }));
	if (cached) out.push(t('remote.share.detach.cached'));
	out.push(t('remote.share.detach.final'));
	return out;
}

/**
 * Boîte « Retirer du serveur » (propriétaire) : la question, puis ce qui
 * se passe, en peu de mots. cached : classeur ouvert depuis le serveur, à
 * enregistrer (où, sur le poste ; téléchargé, en édition web).
 */
export function removeQuestion(s: State, cached: boolean, web: boolean): { question: string; lines: string[] } {
	const lines = [t('remote.share.remove.everyone'), t('remote.share.remove.copies')];
	if (cached) lines.push(t(web ? 'remote.share.remove.download' : 'remote.share.remove.cached'));
	lines.push(t('remote.share.remove.final'));
	return { question: t('remote.share.remove.question', { title: s.title, server: serverLabel(s) }), lines };
}

/** Message de la barre d'état après « Réessayer ». */
export function retryNotice(): string {
	return t('remote.share.retryNotice');
}

/**
 * Message de la barre d'état après « Réessayer », calculé au chargement
 * (français) : préférer retryNotice(), qui suit la langue.
 */
export const RETRY_NOTICE = 'Nouvel essai de connexion…';

/** Heure (aujourd'hui) ou date et heure d'une modification ; "" si inconnue. */
export function whenLabel(at: string, now = new Date()): string {
	if (!at) return '';
	const d = new Date(at);
	if (Number.isNaN(d.getTime())) return '';
	const hm = fmtTime(d, { hour: '2-digit', minute: '2-digit' });
	if (d.toDateString() === now.toDateString()) return t('remote.share.when.today', { time: hm });
	return t('remote.share.when.date', { date: fmtDate(d), time: hm });
}

/** Valeur d'un conflit telle que la grille l'afficherait (« (vide) » si vide). */
export function conflictValue(c: Pick<Conflict, 'type'>, v: unknown): string {
	if (v === null || v === undefined || v === '') return t('remote.share.conflict.empty');
	return formatValue(c.type as ColumnType, v);
}

/** « Modifié par Alice à 10:42 : 1 100 (vous : 1 200) ». */
export function conflictText(c: Conflict, now = new Date()): string {
	const when = whenLabel(c.at, now);
	const author = c.author;
	const change = author
		? when
			? t('remote.share.conflict.modifiedByWhen', { author, when })
			: t('remote.share.conflict.modifiedBy', { author })
		: when
			? t('remote.share.conflict.modifiedWhen', { when })
			: t('remote.share.conflict.modified');
	return t('remote.share.conflict.text', { change, theirs: conflictValue(c, c.theirs), mine: conflictValue(c, c.mine) });
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
	return t('remote.share.refused', { n, list, more });
}

/** « Fichier › Republier » : classeur figé publié, propriétaire ou concepteur. */
export function canRepublish(s: State | null | undefined): boolean {
	return !!s?.shared && s.mode === FROZEN && s.canDesign;
}
