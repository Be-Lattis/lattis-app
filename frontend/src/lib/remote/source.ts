// Partager au Data Lake Lattis : choix entre publier le classeur (avec ses
// droits) et publier le tableau seul comme source poussée liée au classeur
// (pipeline « push » alimenté par le poste, internal/bridge/table_source.go).

/** Valeur refusée par le pipeline (workbook.PushReject). */
export interface PushReject {
	row: number;
	column: string;
	value: string;
	error: string;
}

/** Bilan d'un envoi (workbook.PushRun, une exécution du pipeline). */
export interface PushResult {
	status: string;
	rows_read: number;
	inserted: number;
	updated: number;
	deleted: number;
	rejected: number;
	rejects?: PushReject[] | null;
	error?: string;
}

/** Mode d'écriture d'une source poussée (fixé à sa création). */
export type PushMode = 'replace' | 'append' | 'upsert';

/** Modes proposés, avec leur explication. */
export const PUSH_MODES: { mode: PushMode; label: string; detail: string }[] = [
	{ mode: 'replace', label: 'Remplacer tout', detail: 'à chaque mise à jour, la donnée devient le tableau envoyé' },
	{ mode: 'append', label: 'Ajouter à la suite', detail: 'les lignes envoyées s’ajoutent à celles déjà reçues (journal, relevés)' },
	{ mode: 'upsert', label: 'Mettre à jour par clé', detail: 'une ligne de même clé est modifiée, une nouvelle est ajoutée ; rien n’est supprimé' }
];

/** Lien d'un tableau avec sa source (bridge.TableSourceLink). */
export interface SourceLink {
	tbl: number;
	tableName: string;
	server: string;
	pipeline: string;
	dataset: string;
	workbookId?: string;
	tableId?: string;
	sqlName: string;
	name: string;
	mode?: string;
	columns: string[] | null;
	keys: string[] | null;
	onSave: boolean;
	suspended?: boolean;
	lastPush?: string;
	lastResult?: PushResult | null;
	lastError?: string;
	blanked?: number;
}

/** Voie choisie : publier le classeur, ou le tableau seul. */
export type ShareWay = 'workbook' | 'table';

/** Une des deux voies de la boîte : ouverte ou fermée (avec la raison). */
export interface WayChoice {
	way: ShareWay;
	title: string;
	detail: string;
	disabled: string;
}

/**
 * Les deux voies proposées pour un tableau hors d'un classeur publié :
 * choice « workbook » (classeur non publié) ou « table » (classeur publié
 * sans ce tableau) ; reason : pourquoi la première est fermée.
 */
export function shareWays(choice: string, reason: string): WayChoice[] {
	const republish = choice === 'table';
	return [
		{
			way: 'workbook',
			title: republish ? 'Republier le classeur avec ce tableau' : 'Publier le classeur avec gestion des droits',
			detail: republish
				? 'Le classeur est republié sur son serveur avec ce tableau coché ; ses droits (rôles, filtres) s’appliquent. Le tableau se partage ensuite au Data Lake depuis ce même menu.'
				: 'Le classeur entier est publié sur un Serveur Lattis : vous choisissez qui l’ouvre et ce que chacun voit ou modifie (rôles, filtres). Le tableau se partage ensuite au Data Lake depuis ce même menu.',
			disabled: reason
		},
		{
			way: 'table',
			title: 'Publier uniquement ce tableau comme source de données',
			detail:
				'Une nouvelle donnée du Data Lake, liée à ce classeur et alimentée par un pipeline : seules les colonnes choisies de ce tableau sont envoyées, quand vous le demandez ou à chaque enregistrement. Le classeur reste sur votre poste, sans être mis à disposition.',
			disabled: ''
		}
	];
}

const plural = (n: number) => (n > 1 ? 's' : '');

/** Message de la barre d'état après un envoi (comme bridge.PushSummary). */
export function pushSummary(table: string, r: PushResult | null | undefined, blanked = 0): string {
	if (!r) return `Source « ${table} » mise à jour`;
	const n = r.rows_read ?? 0;
	let msg = `Source « ${table} » mise à jour : ${n} ligne${plural(n)} lue${plural(n)}`;
	const parts: string[] = [];
	if (r.inserted) parts.push(`${r.inserted} ajoutée${plural(r.inserted)}`);
	if (r.updated) parts.push(`${r.updated} modifiée${plural(r.updated)}`);
	if (r.deleted) parts.push(`${r.deleted} supprimée${plural(r.deleted)}`);
	if (r.rejected) parts.push(`${r.rejected} refusée${plural(r.rejected)}`);
	msg += parts.length ? ` (${parts.join(', ')})` : ", rien n'a changé";
	const x = r.rejects?.[0];
	if (x) msg += ` ; ligne ${x.row}, ${x.column} « ${x.value} » : ${x.error}`;
	if (blanked) {
		const s = plural(blanked);
		msg += ` ; ${blanked} valeur${s} envoyée${s} vide${s} (erreur ou saisie hors type)`;
	}
	return msg;
}

/** État d'une source en une ligne : dernier envoi, ou son échec. */
export function sourceStatus(l: Pick<SourceLink, 'lastPush' | 'lastResult' | 'lastError' | 'tableName' | 'suspended'>, now = new Date()): string {
	if (!l.tableName) return 'Tableau supprimé du classeur : la source ne reçoit plus rien';
	if (l.suspended) return 'Source suspendue : plus aucun envoi';
	if (l.lastError) return `Dernière mise à jour en échec : ${l.lastError}`;
	if (!l.lastPush) return 'Jamais mise à jour';
	const at = new Date(l.lastPush);
	const sameDay = at.toDateString() === now.toDateString();
	const when = sameDay
		? `aujourd’hui à ${at.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
		: `le ${at.toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })}`;
	const n = l.lastResult?.rows_read ?? 0;
	const rej = l.lastResult?.rejected ?? 0;
	return `Mise à jour ${when} · ${n} ligne${plural(n)}${rej ? ` · ${rej} refusée${plural(rej)}` : ''}`;
}

/** Nom SQL proposé pour un nom (minuscules sans accents, chiffres, _). */
export function sqlNameFor(name: string): string {
	let s = name
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toLowerCase()
		.replace(/[^a-z0-9]+/g, '_')
		.replace(/^_+|_+$/g, '');
	if (s && !/^[a-z]/.test(s)) s = 'd_' + s;
	return s.slice(0, 63);
}

/** Origine d'une donnée du catalogue publiée seule depuis un classeur local (null : tableau d'un classeur publié). */
export function pushedOrigin(d: { source?: { kind?: string; title?: string; origin?: { workbook: string; table: string } | null } | null }): string | null {
	const s = d.source;
	if (s?.kind !== 'source') return null;
	const o = s.origin;
	if (!o?.workbook) return s.title || 'classeur local';
	return o.table ? `${o.workbook} › ${o.table}` : o.workbook;
}

/** Une source poussée de l'utilisateur (workbook.PushedSource, my_pushed_sources). */
export interface PushedSource {
	pipeline_id: string;
	name: string;
	mode: string;
	key_columns: string[] | null;
	origin: { workbook: string; table: string } | null;
	enabled: boolean;
	pending: boolean;
	datasets: { id: string; name: string; sql_name: string }[] | null;
	last_run?: (PushResult & { finished_at?: string }) | null;
}

/** Ligne d'état d'une source poussée de la liste « Vos sources poussées ». */
export function pushedLine(p: PushedSource): string {
	const mode = PUSH_MODES.find((m) => m.mode === p.mode)?.label ?? p.mode;
	const parts = [`${p.origin?.workbook || '?'} › ${p.origin?.table || '?'}`, mode + (p.mode === 'upsert' && p.key_columns?.length ? ` (${p.key_columns.join(', ')})` : '')];
	if (!p.enabled) parts.push('suspendue');
	else if (p.pending) parts.push('envoi en attente');
	const r = p.last_run;
	if (r?.finished_at) {
		const when = new Date(r.finished_at).toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' });
		parts.push(r.status === 'error' ? `échec le ${when}` : `mise à jour le ${when}`);
	} else if (!r) parts.push('jamais mise à jour');
	return parts.join(' · ');
}

/** Clés gardées : celles qui restent parmi les colonnes envoyées. */
export function keptKeys(keys: readonly string[], sent: ReadonlySet<string>): string[] {
	return keys.filter((k) => sent.has(k));
}
