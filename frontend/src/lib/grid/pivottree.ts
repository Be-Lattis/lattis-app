// Tableau croisé en arborescence (disposition compacte d'Excel) : avec
// plusieurs champs en lignes, le calcul (internal/canvas/pivottree.go)
// renvoie une description par ligne (niveau, groupe −/+, sous-total). Ce
// module en tire l'indentation, la case −/+ et les changements de
// configuration (plier, déplier, tout plier) : sur la feuille comme dans un
// onglet Document.

/** Description d'une ligne affichée (PivotRowInfo côté Go). */
export interface PivotRowInfo {
	depth: number;
	group?: boolean;
	collapsed?: boolean;
	subtotal?: boolean;
	total?: boolean;
	path?: string[] | null;
}

/** Ce qui, dans la configuration d'un TCD, dit quels groupes sont pliés. */
export interface FoldConfig {
	collapseAll?: boolean;
	collapsed?: (string[] | null)[] | null;
	expanded?: (string[] | null)[] | null;
}

/** Retrait par niveau (px). */
export const PIVOT_INDENT = 14;
/** Côté de la case −/+ (px). */
export const PIVOT_BOX = 9;
/** Bord gauche de la case −/+ du premier niveau, depuis le bord de la cellule (px). */
const BOX_LEFT = 4;
/** Place de la case −/+ devant l'étiquette (px). */
const BOX_SPACE = PIVOT_BOX + 5;

/** Retrait du texte d'une ligne dans la colonne des étiquettes (px) : le total général n'en a pas. */
export function pivotLabelOffset(info: PivotRowInfo | undefined): number {
	if (!info || info.total) return 0;
	return info.depth * PIVOT_INDENT + BOX_SPACE;
}

/** Bord gauche de la case −/+ d'un groupe, depuis le bord de la cellule (px). */
export function pivotBoxLeft(depth: number): number {
	return BOX_LEFT + depth * PIVOT_INDENT;
}

/** Le point x (depuis le bord gauche de la cellule) tombe-t-il sur la case −/+ de la ligne ? */
export function pivotBoxHit(info: PivotRowInfo | undefined, x: number): boolean {
	if (!info?.group) return false;
	const l = pivotBoxLeft(info.depth);
	return x >= l - 3 && x <= l + PIVOT_BOX + 3;
}

const same = (a: readonly string[] | null | undefined, b: readonly string[]) => !!a && a.length === b.length && a.every((s, i) => s === b[i]);

/** Le groupe path est-il plié ? */
export function isFolded(cfg: FoldConfig, path: readonly string[]): boolean {
	if (cfg.collapseAll) return !(cfg.expanded ?? []).some((p) => same(p, path));
	return (cfg.collapsed ?? []).some((p) => same(p, path));
}

/** Configuration où le groupe path est plié (fold) ou déplié ; les listes vides disparaissent. */
export function setFolded<T extends FoldConfig>(cfg: T, path: readonly string[], fold: boolean): T {
	const out = { ...cfg };
	if (isFolded(cfg, path) === fold) return out;
	// Avec « tout plié », on note les exceptions dépliées ; sinon les groupes pliés.
	const key = cfg.collapseAll ? 'expanded' : 'collapsed';
	const add = cfg.collapseAll ? !fold : fold;
	const list = (cfg[key] ?? []).filter((p) => !same(p, path));
	if (add) list.push([...path]);
	if (list.length) out[key] = list;
	else delete out[key];
	return out;
}

/** Groupes pliés des champs en colonnes (mêmes règles que ceux des lignes). */
export interface ColFoldConfig {
	colCollapseAll?: boolean;
	colCollapsed?: (string[] | null)[] | null;
	colExpanded?: (string[] | null)[] | null;
}

/** Configuration où le groupe de colonnes path est plié (fold) ou déplié. */
export function setColFolded<T extends ColFoldConfig>(cfg: T, path: readonly string[], fold: boolean): T {
	const f = setFolded({ collapseAll: cfg.colCollapseAll, collapsed: cfg.colCollapsed, expanded: cfg.colExpanded }, path, fold);
	const out = { ...cfg };
	delete out.colCollapseAll;
	delete out.colCollapsed;
	delete out.colExpanded;
	if (f.collapseAll) out.colCollapseAll = true;
	if (f.collapsed) out.colCollapsed = f.collapsed;
	if (f.expanded) out.colExpanded = f.expanded;
	return out;
}

/** Configuration toute pliée (Réduire tout) ou toute dépliée (Développer tout). */
export function foldAll<T extends FoldConfig>(cfg: T, fold: boolean): T {
	const out = { ...cfg };
	delete out.collapsed;
	delete out.expanded;
	if (fold) out.collapseAll = true;
	else delete out.collapseAll;
	return out;
}

/**
 * Groupe que vise Développer (expand) ou Réduire depuis une ligne : le groupe
 * de la ligne ; pour Réduire depuis une ligne qui n'a rien à plier (ligne du
 * dernier champ, groupe déjà plié), le groupe parent. null : rien à faire.
 */
export function foldTarget(info: PivotRowInfo | undefined, expand: boolean): string[] | null {
	const path = info?.path;
	if (!info || info.total || !path?.length) return null;
	if (expand) return (info.group && info.collapsed) ? [...path] : null;
	if (info.subtotal || (info.group && !info.collapsed)) return [...path];
	return path.length > 1 ? path.slice(0, -1) : null;
}

/** Y a-t-il quelque chose à plier ou à déplier dans le TCD (Réduire tout, Développer tout) ? */
export function canFoldAll(rows: readonly PivotRowInfo[] | null | undefined, expand: boolean): boolean {
	return !!rows?.some((r) => r.group && (expand ? r.collapsed : !r.collapsed));
}
