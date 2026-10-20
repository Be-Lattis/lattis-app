// Partager au Data Lake Lattis : choix entre publier le classeur (avec ses
// droits) et publier le tableau seul comme source poussée liée au classeur
// (pipeline « push » alimenté par le poste, internal/bridge/table_source.go).
import { fmtDateTime, fmtTime, t, tAny } from '#lib/i18n/index.js';

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

/** Modes proposés, avec leur explication (textes relus dans la langue de l'interface). */
const pushMode = (mode: PushMode) => ({
	mode,
	get label() {
	return tAny(`remote.source.mode.${mode}.label`);
},
get detail() {
return tAny(`remote.source.mode.${mode}.detail`);
}
});
export const PUSH_MODES: { mode: PushMode; readonly label: string; readonly detail: string }[] = [pushMode('replace'), pushMode('append'), pushMode('upsert')];

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

/** Voie choisie : publier le classeur, le tableau seul, ou (classeur partagé en direct) ajouter le tableau au classeur partagé. */
export type ShareWay = 'workbook' | 'table' | 'shared';

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
			title: republish ? t('remote.source.way.republish.title') : t('remote.source.way.publish.title'),
			detail: republish
				? t('remote.source.way.republish.detail')
				: t('remote.source.way.publish.detail'),
			disabled: reason
		},
		{
			way: 'table',
			title: t('remote.source.way.table.title'),
			detail: t('remote.source.way.table.detail'),
			disabled: ''
		}
	];
}

/** Écran du choix de la voie : titre, phrase d'explication, voies ouvertes seulement, et « pourquoi pas tout le classeur ». */
export interface Chooser {
	title: string;
	lead: string;
	ways: WayChoice[];
	/** Pourquoi la voie du classeur n'est pas proposée ('' : elle l'est). */
	why: string;
}

/**
 * Le choix en clair. Classeur partagé en direct sans ce tableau (choice
 * « collab ») : l'ajouter au classeur partagé (recommandé ; reason dit
 * pourquoi c'est fermé : droits, hors ligne), ou le publier seul au Data
 * Lake. Classeur collaboratif où ce tableau n'a pas été publié avec une
 * raison (choice « table », reason) : seule reste la publication du
 * tableau seul, présentée sans voie grisée ni refus en rouge.
 */
export function chooser(choice: string, reason: string): Chooser {
	const ways = shareWays(choice, reason);
	if (choice === 'collab') {
		// Classeur partagé en direct sans ce tableau : l'ajouter au classeur
		// (recommandé, en premier ; reason : pourquoi c'est fermé), ou le
		// publier seul au Data Lake.
		const table = ways.find((w) => w.way === 'table') as WayChoice;
		return {
			title: t('remote.source.chooser.localTitle'),
			lead: t('remote.source.chooser.notInCollab'),
			ways: [
				{
					way: 'shared',
					title: t('remote.source.way.shared.title'),
					detail: t('remote.source.way.shared.detail'),
					disabled: reason
				},
				{ ...table, title: t('remote.source.way.tableAlone') }
			],
			why: ''
		};
	}
	if (!reason) return { title: t('remote.source.chooser.howTitle'), lead: t('remote.source.chooser.notPublished'), ways, why: '' };
	const table = ways.find((w) => w.way === 'table') as WayChoice;
	return {
		title: t('remote.source.chooser.localTitle'),
		lead: choice === 'table' ? t('remote.source.chooser.notInCollab') : reason,
		ways: [{ ...table, title: t('remote.source.way.tableAlone') }],
		why:
			choice === 'table'
				? t('remote.source.chooser.whyCollab')
				: reason
	};
}


/** Message de la barre d'état après un envoi (comme bridge.PushSummary). */
export function pushSummary(table: string, r: PushResult | null | undefined, blanked = 0): string {
	if (!r) return t('remote.source.push.done', { table });
	const n = r.rows_read ?? 0;
	let msg = t('remote.source.push.read', { table, n });
	const parts: string[] = [];
	if (r.inserted) parts.push(t('remote.source.push.inserted', { n: r.inserted }));
	if (r.updated) parts.push(t('remote.source.push.updated', { n: r.updated }));
	if (r.deleted) parts.push(t('remote.source.push.deleted', { n: r.deleted }));
	if (r.rejected) parts.push(t('remote.source.push.rejected', { n: r.rejected }));
	msg += parts.length ? ` (${parts.join(', ')})` : t('remote.source.push.unchanged');
	const x = r.rejects?.[0];
	if (x) msg += t('remote.source.push.reject', { row: String(x.row), column: x.column, value: x.value, error: x.error });
	if (blanked) msg += t('remote.source.push.blanked', { n: blanked });
	return msg;
}

/** État d'une source en une ligne : dernier envoi, ou son échec. */
export function sourceStatus(l: Pick<SourceLink, 'lastPush' | 'lastResult' | 'lastError' | 'tableName' | 'suspended'>, now = new Date()): string {
	if (!l.tableName) return t('remote.source.status.tableGone');
	if (l.suspended) return t('remote.source.status.suspended');
	if (l.lastError) return t('remote.source.status.failed', { error: l.lastError });
	if (!l.lastPush) return t('remote.source.status.never');
	const at = new Date(l.lastPush);
	const sameDay = at.toDateString() === now.toDateString();
	const when = sameDay
		? t('remote.source.status.today', { time: fmtTime(at, { hour: '2-digit', minute: '2-digit' }) })
		: t('remote.source.status.on', { date: fmtDateTime(at, { dateStyle: 'short', timeStyle: 'short' }) });
	const n = l.lastResult?.rows_read ?? 0;
	const rej = l.lastResult?.rejected ?? 0;
	return t('remote.source.status.updated', { when, n }) + (rej ? ` · ${t('remote.source.push.rejected', { n: rej })}` : '');
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
	if (!o?.workbook) return s.title || t('remote.source.localWorkbook');
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
	if (!p.enabled) parts.push(t('remote.source.suspended'));
	else if (p.pending) parts.push(t('remote.source.pending'));
	const r = p.last_run;
	if (r?.finished_at) {
		const when = fmtDateTime(new Date(r.finished_at), { dateStyle: 'short', timeStyle: 'short' });
		parts.push(r.status === 'error' ? t('remote.source.failedOn', { date: when }) : t('remote.source.updatedOn', { date: when }));
	} else if (!r) parts.push(t('remote.source.never'));
	return parts.join(' · ');
}

/** Clés gardées : celles qui restent parmi les colonnes envoyées. */
export function keptKeys(keys: readonly string[], sent: ReadonlySet<string>): string[] {
	return keys.filter((k) => sent.has(k));
}

/** Nom SQL d'une donnée : contrôle local ('' : accepté, ou vide pour laisser le serveur le proposer). */
export function sqlNameProblem(s: string): string {
	const v = s.trim();
	if (!v) return '';
	if (v.length > 63) return t('remote.source.sqlName.length');
	if (!/^[a-z]/.test(v)) return t('remote.source.sqlName.start');
	if (!/^[a-z0-9_]+$/.test(v)) return t('remote.source.sqlName.chars');
	return '';
}

/** Champs d'un formulaire de partage, pour son contrôle avant l'envoi. */
export interface ShareForm {
	server: string;
	name: string;
	/** Nom SQL saisi ('' : proposé par le serveur ; null : déjà fixé). */
	sqlName: string | null;
	columns: number;
	mode?: PushMode;
	key?: string;
	/** Un filtre d'audience en cours de saisie est invalide. */
	badFilter: boolean;
}

/** Problème d'un champ du formulaire, par champ ; vide : prêt à envoyer. */
export type ShareProblems = Partial<Record<'server' | 'name' | 'sql' | 'columns' | 'key' | 'audience', string>>;

/** Ce qui empêche d'envoyer le formulaire, champ par champ (messages affichés sous le champ). */
export function shareProblems(f: ShareForm): ShareProblems {
	const p: ShareProblems = {};
	if (!f.server) p.server = t('remote.source.problem.server');
	if (!f.name.trim()) p.name = t('remote.source.problem.name');
	const sql = f.sqlName === null ? '' : sqlNameProblem(f.sqlName);
	if (sql) p.sql = sql;
	if (f.columns === 0) p.columns = t('remote.source.problem.columns');
	if (f.mode === 'upsert' && !f.key) p.key = t('remote.source.problem.key');
	if (f.badFilter) p.audience = t('remote.source.problem.filter');
	return p;
}

/** Résumé d'une ligne des options avancées repliées (« donnees.ventes · Remplacer tout · à chaque enregistrement »). */
export function advancedSummary(o: { sqlName: string; mode?: PushMode; key?: string; onSave?: boolean; suspended?: boolean }): string {
	const parts: string[] = [];
	if (o.sqlName.trim()) parts.push(`donnees.${o.sqlName.trim()}`);
	if (o.mode) {
		const m = PUSH_MODES.find((x) => x.mode === o.mode)?.label ?? o.mode;
		parts.push(o.mode === 'upsert' && o.key ? `${m} (${o.key})` : m);
	}
	if (o.suspended) parts.push(t('remote.source.suspended'));
	else if (o.onSave !== undefined) parts.push(o.onSave ? t('remote.source.onSave') : t('remote.source.onDemand'));
	return parts.join(' · ');
}
