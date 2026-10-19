// Partager au Data Lake Lattis : choix entre publier le classeur (avec ses
// droits) et publier le tableau seul comme source de données liée au
// classeur (pipeline alimenté par le poste, internal/bridge/table_source.go).

/** Compte rendu d'un envoi (workbook.PushResult). */
export interface PushResult {
	run_id: string;
	status: string;
	rows: number;
	inserted: number;
	updated: number;
	deleted: number;
	rejected: number;
	rejects?: { row: number; error: string }[] | null;
}

/** Lien d'un tableau avec sa source (bridge.TableSourceLink). */
export interface SourceLink {
	tbl: number;
	tableName: string;
	server: string;
	pipeline: string;
	dataset: string;
	sqlName: string;
	name: string;
	columns: string[] | null;
	keys: string[] | null;
	onSave: boolean;
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
				'Une nouvelle donnée du Data Lake, liée à ce classeur : seules les colonnes choisies de ce tableau sont envoyées, quand vous le demandez ou à chaque enregistrement. Le classeur reste sur votre poste, sans être mis à disposition.',
			disabled: ''
		}
	];
}

const plural = (n: number) => (n > 1 ? 's' : '');

/** Message de la barre d'état après un envoi (comme bridge.PushSummary). */
export function pushSummary(table: string, r: PushResult | null | undefined, blanked = 0): string {
	if (!r) return `Source « ${table} » mise à jour`;
	let msg = `Source « ${table} » mise à jour : ${r.rows} ligne${plural(r.rows)}`;
	const parts: string[] = [];
	if (r.inserted) parts.push(`${r.inserted} ajoutée${plural(r.inserted)}`);
	if (r.updated) parts.push(`${r.updated} modifiée${plural(r.updated)}`);
	if (r.deleted) parts.push(`${r.deleted} supprimée${plural(r.deleted)}`);
	if (r.rejected) parts.push(`${r.rejected} refusée${plural(r.rejected)}`);
	msg += parts.length ? ` (${parts.join(', ')})` : ", rien n'a changé";
	if (blanked) {
		const s = plural(blanked);
		msg += ` ; ${blanked} valeur${s} envoyée${s} vide${s} (erreur ou saisie hors type)`;
	}
	return msg;
}

/** État d'une source en une ligne : dernier envoi, ou son échec. */
export function sourceStatus(l: Pick<SourceLink, 'lastPush' | 'lastResult' | 'lastError' | 'tableName'>, now = new Date()): string {
	if (!l.tableName) return 'Tableau supprimé du classeur : la source ne reçoit plus rien';
	if (l.lastError) return `Dernière mise à jour en échec : ${l.lastError}`;
	if (!l.lastPush) return 'Jamais mise à jour';
	const at = new Date(l.lastPush);
	const sameDay = at.toDateString() === now.toDateString();
	const when = sameDay
		? `aujourd’hui à ${at.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })}`
		: `le ${at.toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })}`;
	const n = l.lastResult?.rows ?? 0;
	return `Mise à jour ${when} · ${n} ligne${plural(n)}`;
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

/** Clés gardées : celles qui restent parmi les colonnes envoyées. */
export function keptKeys(keys: readonly string[], sent: ReadonlySet<string>): string[] {
	return keys.filter((k) => sent.has(k));
}
