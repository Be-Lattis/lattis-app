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
	{ mode: 'replace', label: 'Remplacer tout', detail: 'La donnée devient le tableau envoyé.' },
	{ mode: 'append', label: 'Ajouter à la suite', detail: 'Les lignes s’ajoutent à celles déjà reçues (journal, relevés).' },
	{ mode: 'upsert', label: 'Mettre à jour par clé', detail: 'Même clé : ligne modifiée ; sinon ajoutée. Rien n’est supprimé.' }
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
			title: republish ? 'Republier le classeur avec ce tableau' : 'Publier le classeur avec gestion des droits',
			detail: republish
				? 'Le classeur est republié avec ce tableau ; ses rôles et filtres s’appliquent.'
				: 'Tout le classeur va sur le serveur, avec rôles et filtres ; le tableau se partage ensuite.',
			disabled: reason
		},
		{
			way: 'table',
			title: 'Publier uniquement ce tableau',
			detail: 'Seules les colonnes choisies sont envoyées ; le classeur reste sur votre poste, sans être mis à disposition.',
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
			title: 'Ce tableau n’est que sur votre poste',
			lead: 'Le classeur est partagé en direct, mais ce tableau n’en fait pas partie.',
			ways: [
				{
					way: 'shared',
					title: 'Ajouter ce tableau au classeur partagé',
					detail: 'Recommandé. Il est créé sur le serveur avec ses lignes ; les membres le reçoivent et le modifient selon leurs droits. Il se partage ensuite au Data Lake depuis ce même menu.',
					disabled: reason
				},
				{ ...table, title: 'Publier ce tableau seul au Data Lake' }
			],
			why: ''
		};
	}
	if (!reason) return { title: 'Comment partager ce tableau ?', lead: 'Ce tableau n’est pas dans un classeur publié.', ways, why: '' };
	const table = ways.find((w) => w.way === 'table') as WayChoice;
	return {
		title: 'Ce tableau n’est que sur votre poste',
		lead: choice === 'table' ? 'Le classeur est partagé en direct, mais ce tableau n’en fait pas partie.' : reason,
		ways: [{ ...table, title: 'Publier ce tableau seul au Data Lake' }],
		why:
			choice === 'table'
				? 'Un classeur partagé en direct ne se republie pas : la liste de ses tableaux est fixée à sa publication, et un tableau créé ensuite reste sur votre poste.'
				: reason
	};
}

const plural = (n: number) =>(n > 1 ? 's' : '');

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

/** Nom SQL d'une donnée : contrôle local ('' : accepté, ou vide pour laisser le serveur le proposer). */
export function sqlNameProblem(s: string): string {
	const v = s.trim();
	if (!v) return '';
	if (v.length > 63) return '63 caractères au plus';
	if (!/^[a-z]/.test(v)) return 'Commence par une lettre minuscule';
	if (!/^[a-z0-9_]+$/.test(v)) return 'Minuscules sans accent, chiffres et _ seulement';
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
	if (!f.server) p.server = 'Connectez-vous à un Serveur Lattis';
	if (!f.name.trim()) p.name = 'Donnez un nom à la donnée';
	const sql = f.sqlName === null ? '' : sqlNameProblem(f.sqlName);
	if (sql) p.sql = sql;
	if (f.columns === 0) p.columns = 'Cochez au moins une colonne';
	if (f.mode === 'upsert' && !f.key) p.key = 'Choisissez la colonne qui identifie une ligne';
	if (f.badFilter) p.audience = 'Un filtre de lignes est à corriger';
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
	if (o.suspended) parts.push('suspendue');
	else if (o.onSave !== undefined) parts.push(o.onSave ? 'à chaque enregistrement' : 'à la demande');
	return parts.join(' · ');
}
