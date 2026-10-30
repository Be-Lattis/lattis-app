// Pastilles posées sur les tableaux (nom, « suit l'onglet », clé proposée)
// et ce qu'un clic dessus fait. Sorti de Sheet.svelte
// (docs/decoupage-sheet.md).
import { tick } from 'svelte';
import { clip } from '#lib/ui/menu.js';
import { errorText, t as tr } from '#lib/i18n/index.js';
import { type SheetPivot, type SheetTable, type SheetSource, type Entry } from './model';

/** Ce que createBadges lit et modifie dans Sheet.svelte. */
export interface BadgesHost {
	readonly toLocal: (e: MouseEvent) => { x: number; y: number; };
	readonly followBadges: { kind: 'follow' | 'name' | 'unnamed' | 'local'; e: Entry; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[];
	readonly pivotBadges: { p: SheetPivot; label: string; radii: number[]; x0: number; x1: number; y0: number; y1: number; clip: { x0: number; x1: number; y0: number; y1: number } }[];
	readonly ctx: CanvasRenderingContext2D;
	readonly moveTo: (row: number, col: number, extend?: boolean) => void;
	readonly ensureVisible: (row: number, col: number) => void;
	readonly tableNameEl: HTMLInputElement | undefined;
	readonly source: SheetSource;
	followMenu: { x: number; y: number; e: Entry } | null;
	menu: { x: number; y: number; e: Entry; n: number; i: number; header: boolean; rowOnly?: boolean; confirmDelete?: boolean } | null;
	readonly wrap: HTMLDivElement;
	writing: Promise<void>;
	readonly say: (m: string) => void;
	readonly keyHintBoxes: { e: Entry; x0: number; x1: number; y0: number; y1: number }[];
	readonly closeMenus: () => void;
	hintMenu: { x: number; y: number; e: Entry } | null;
}

export function createBadges(h: BadgesHost) {
	function badgeAt(ev: MouseEvent) {
		const { x, y } = h.toLocal(ev);
		return h.followBadges.find((b) => x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1) ?? null;
	}
	function followBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b?.kind === 'follow' ? b.e : null;
	}
	/** Pastille du nom d'un tableau sous la souris : son tableau. */
	function nameBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b && (b.kind === 'name' || b.kind === 'unnamed') ? b.e : null;
	}
	/** Pastille du nom d'un tableau croisé sous la souris : son TCD. */
	function pivotBadgeAt(ev: MouseEvent): SheetPivot | null {
		const { x, y } = h.toLocal(ev);
		return h.pivotBadges.find((b) => x >= b.x0 && x <= b.x1 && y >= b.y0 && y <= b.y1)?.p ?? null;
	}
	/** Pastille « sur ce poste » sous la souris : son tableau. */
	function localBadgeAt(ev: MouseEvent): Entry | null {
		const b = badgeAt(ev);
		return b?.kind === 'local' ? b.e : null;
	}

	/** Texte coupé (…) pour tenir en w pixels (police courante). */
	function fitText(t: string, w: number): string {
		if (h.ctx.measureText(t).width <= w) return t;
		let s = t;
		while (s.length > 1 && h.ctx.measureText(s + '…').width > w) s = s.slice(0, -1);
		return s + '…';
	}

	/** Clic sur la pastille du nom : la cellule passe sur l'en-tête du tableau et le champ du nom s'ouvre. */
	async function renameFromBadge(en: Entry) {
		h.moveTo(en.t.row, en.t.col);
		h.ensureVisible(en.t.row, en.t.col);
		await tick();
		h.tableNameEl?.focus();
		h.tableNameEl?.select();
	}

	/** Coche ou décoche « Suivre l'année » d'un tableau. */
	function toggleFollow(t: SheetTable, on: boolean) {
		const set = h.source.setFollow;
		if (!set) return;
		h.followMenu = h.menu = null;
		h.wrap.focus();
		h.writing = h.writing.catch(() => undefined).then(async () => {
			try {
				await set(t, on);
			} catch (err) {
				h.say(errorText(err));
			}
		});
	}

	/** Tableau dont la clé proposée est sous la souris. */
	function keyHintAt(ev: MouseEvent): Entry | null {
		const { x, y } = h.toLocal(ev);
		return h.keyHintBoxes.find((b) => x >= b.x0 - 2 && x <= b.x1 + 1 && y >= b.y0 && y <= b.y1)?.e ?? null;
	}

	/** Pose la clé proposée (une étape d'annulation). */
	function acceptKeyHint(e: Entry) {
		const setKey = h.source.setKey;
		if (!setKey) return;
		const t = e.t;
		const name = t.columns[0]?.name ?? '';
		h.wrap.focus();
		h.writing = h.writing
			.catch(() => undefined)
			.then(async () => {
				try {
					await setKey(t, 0);
					h.say(tr('grid.key.set', { name, table: t.name }));
				} catch (err) {
					h.say(errorText(err));
				}
			});
	}

	/** Clic droit sur la clé proposée : son petit menu. */
	function openHintMenu(ev: MouseEvent): boolean {
		const he = keyHintAt(ev);
		if (!he) return false;
		ev.preventDefault();
		h.closeMenus();
		const r = h.wrap.getBoundingClientRect();
		h.hintMenu = { x: ev.clientX - r.left, y: ev.clientY - r.top, e: he };
		return true;
	}

	return {
		followBadgeAt,
		nameBadgeAt,
		pivotBadgeAt,
		localBadgeAt,
		fitText,
		renameFromBadge,
		toggleFollow,
		keyHintAt,
		acceptKeyHint,
		openHintMenu
	};
}
