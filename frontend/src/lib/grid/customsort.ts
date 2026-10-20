// Tri personnalisé (comme les listes personnalisées d'Excel) : logique du
// dialogue qui range les valeurs d'une colonne ou d'un champ de TCD. Les
// valeurs se comparent sans tenir compte de la casse, comme le backend
// (canvas/customsort.go) ; les vides ne s'y rangent pas : ils restent
// toujours à la fin.
import { lower, t } from '#lib/i18n/index.js';

/** Ordre personnalisé (canvas.CustomOrder). */
export interface CustomOrder {
	/** Liste nommée d'où vient l'ordre (Mois, Jours…). */
	list?: string;
	values: string[];
	/** Liste à rebours (les valeurs hors liste restent après). */
	desc?: boolean;
}

/** Liste personnalisée nommée (canvas.CustomList). */
export interface CustomList {
	name: string;
	values: string[];
	builtin?: boolean;
}

/** Élément du dialogue : absent, une valeur de l'ordre que les données n'ont pas (encore). */
export interface OrderItem {
	text: string;
	absent?: boolean;
}

export const fold = (s: string) => lower(s);

/** Retire les vides et les doublons (sans casse), dans l'ordre. */
export function cleanValues(values: string[]): string[] {
	const seen = new Set<string>();
	const out: string[] = [];
	for (const v of values) {
		const k = fold(v);
		if (v === '' || seen.has(k)) continue;
		seen.add(k);
		out.push(v);
	}
	return out;
}

/** Déplace l'élément from à la place to (bornée), dans une copie. */
export function moveItem<T>(list: T[], from: number, to: number): T[] {
	const out = [...list];
	if (from < 0 || from >= out.length) return out;
	to = Math.max(0, Math.min(out.length - 1, to));
	const [x] = out.splice(from, 1);
	out.splice(to, 0, x);
	return out;
}

/**
 * Éléments du dialogue : l'ordre donné d'abord (ses valeurs absentes des
 * données comprises, marquées), puis les valeurs des données qu'il ne
 * range pas, dans leur ordre (croissant, comme le menu ▾).
 */
export function orderItems(values: string[], order: string[] | null | undefined): OrderItem[] {
	const present = new Map<string, string>();
	for (const v of values) if (v !== '' && !present.has(fold(v))) present.set(fold(v), v);
	const out: OrderItem[] = [];
	const used = new Set<string>();
	for (const v of cleanValues(order ?? [])) {
		const k = fold(v);
		used.add(k);
		const p = present.get(k);
		out.push(p !== undefined ? { text: p } : { text: v, absent: true });
	}
	for (const [k, v] of present) if (!used.has(k)) out.push({ text: v });
	return out;
}

/**
 * Liste nommée qui range le mieux ces valeurs (au moins deux valeurs et la
 * moitié de celles des données), pour la proposer d'emblée ; null sinon.
 */
export function suggestList(values: string[], lists: CustomList[]): CustomList | null {
	const vals = cleanValues(values).map(fold);
	if (vals.length < 2) return null;
	let best: CustomList | null = null;
	let hits = 0;
	for (const l of lists) {
		const set = new Set(l.values.map(fold));
		const n = vals.filter((v) => set.has(v)).length;
		if (n > hits) {
			best = l;
			hits = n;
		}
	}
	return best && hits >= 2 && hits * 2 >= vals.length ? best : null;
}

/**
 * Ordre à enregistrer : la liste nommée telle quelle si les éléments la
 * suivent encore (les autres valeurs viennent après de toute façon), sinon
 * tous les éléments dans l'ordre choisi.
 */
export function orderOf(items: OrderItem[], list: CustomList | null, desc: boolean): CustomOrder {
	const values = items.map((i) => i.text);
	if (list) {
		const lv = cleanValues(list.values);
		const head = values.slice(0, lv.length);
		if (head.length === lv.length && head.every((v, i) => fold(v) === fold(lv[i]))) {
			return { list: list.name, values: lv, ...(desc ? { desc } : {}) };
		}
	}
	return { values, ...(desc ? { desc } : {}) };
}

/** Résumé d'un ordre pour un menu : « Mois », ou les premières valeurs. */
export function orderSummary(o: CustomOrder | null | undefined): string {
	if (!o) return '';
	const head = o.list ? t('grid.customSort.summary.list', { name: o.list }) : o.values.slice(0, 3).join(', ') + (o.values.length > 3 ? '…' : '');
	return o.desc ? t('grid.customSort.summary.reversed', { order: head }) : head;
}

/** Retire de la configuration d'un TCD le tri d'un champ (asc/desc) ou son ordre personnalisé. */
export function withoutField<T>(m: Record<string, T> | undefined, field: string): Record<string, T> | undefined {
	if (!m) return undefined;
	const { [field]: _, ...rest } = m;
	return Object.keys(rest).length ? rest : undefined;
}
