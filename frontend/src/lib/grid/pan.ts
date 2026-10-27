// Main sur la feuille (Sheet.svelte) : clic droit ou clic du milieu
// maintenu puis glissé, ou Espace maintenue et clic gauche glissé, fait
// défiler la feuille comme si on la tenait (glisser vers la gauche montre
// la droite). Un clic droit sans bouger (moins de PAN_SLOP px) ouvre le menu
// contextuel comme avant ; après un glissé, aucun menu.
//
// Le menu contextuel arrive au relâcher sous Windows (WebView2), à l'appui
// sous macOS et Linux : à l'appui, il est retenu jusqu'au relâcher et
// abandonné si la feuille a glissé ; au relâcher, celui qui suit un glissé
// est écarté.

/** Déplacement (px de l'écran) en deçà duquel un clic reste un clic. */
export const PAN_SLOP = 4;
/** Délai (ms) après un glissé du clic droit pendant lequel le menu qui suit est écarté. */
export const MENU_GRACE_MS = 500;

/** Ce bouton (MouseEvent.button) fait-il glisser la feuille ? Droit, milieu, ou gauche avec Espace. */
export function pansWith(button: number, space: boolean): boolean {
	return button === 1 || button === 2 || (button === 0 && space);
}

/**
 * Appui sur un objet posé sur la feuille (graphique, image, forme, bouton,
 * segment…) : 'pan' la feuille prend le geste et l'objet ne le voit pas
 * (milieu, Espace + gauche) ; 'menu' la feuille glisse si l'on bouge, et
 * sans glissé l'objet garde son menu (droit) ; 'object' l'objet le traite
 * (gauche seul : il se sélectionne et se déplace, comme avant).
 */
export function onObject(button: number, space: boolean): 'pan' | 'menu' | 'object' {
	if (!pansWith(button, space)) return 'object';
	return button === 2 ? 'menu' : 'pan';
}

/** Ce que devient un événement contextmenu : ouvert, retenu jusqu'au relâcher, ou écarté. */
export type MenuFate = 'open' | 'defer' | 'drop';

export class PanGesture<M = unknown> {
	private g: { button: number; x0: number; y0: number; x: number; y: number; on: boolean } | null = null;
	private deferred: M | null = null;
	private dropUntil = -1;

	/** Un bouton qui peut faire glisser est enfoncé. */
	get active(): boolean {
		return this.g !== null;
	}

	/** La feuille glisse (le seuil est passé). */
	get panning(): boolean {
		return !!this.g?.on;
	}

	/** Bouton enfoncé en (x, y), px de l'écran. */
	begin(button: number, x: number, y: number) {
		this.g = { button, x0: x, y0: y, x, y, on: false };
		this.deferred = null;
		this.dropUntil = -1;
	}

	/**
	 * Pointeur en (x, y) : le défilement à faire (px de l'écran, positif vers
	 * la droite et le bas), null sous le seuil. Le seuil passé, tout le chemin
	 * depuis l'appui est rendu : le point saisi reste sous le pointeur.
	 */
	move(x: number, y: number): { dx: number; dy: number } | null {
		const g = this.g;
		if (!g) return null;
		if (!g.on) {
			if (Math.hypot(x - g.x0, y - g.y0) < PAN_SLOP) return null;
			g.on = true;
		}
		const d = { dx: g.x - x, dy: g.y - y };
		g.x = x;
		g.y = y;
		return d;
	}

	/**
	 * Bouton relâché à l'instant t (ms) : null si ce n'est pas celui du geste.
	 * menu : le menu contextuel retenu à l'appui, à ouvrir maintenant (clic
	 * droit sans glissé).
	 */
	end(button: number, t: number): { panned: boolean; menu: M | null } | null {
		const g = this.g;
		if (!g || button !== g.button) return null;
		this.g = null;
		const menu = this.deferred;
		this.deferred = null;
		if (g.on) {
			if (g.button === 2) this.dropUntil = t + MENU_GRACE_MS;
			return { panned: true, menu: null };
		}
		return { panned: false, menu };
	}

	/** Geste abandonné (fenêtre quittée) : ni glissé ni menu retenu. */
	cancel() {
		this.g = null;
		this.deferred = null;
	}

	/** Événement contextmenu reçu à l'instant t (ms). */
	menu(ev: M, t: number): MenuFate {
		const g = this.g;
		if (g && g.button === 2) {
			if (g.on) return 'drop';
			this.deferred = ev;
			return 'defer';
		}
		if (t <= this.dropUntil) {
			this.dropUntil = -1;
			return 'drop';
		}
		return 'open';
	}
}

/**
 * Espace maintenue sur la grille (hors saisie) : la main est prête, un clic
 * gauche fait glisser la feuille. Appuyée et relâchée sans clic, Espace
 * saisit une espace dans la cellule active, comme avant, au relâcher.
 */
export class SpaceHold {
	down = false;
	private pending = false;

	press() {
		this.down = true;
		this.pending = true;
	}

	/** Clic pendant qu'Espace est tenue : l'espace ne sera pas saisie. */
	click() {
		this.pending = false;
	}

	/** Espace relâchée : true s'il faut saisir l'espace (aucun clic entre-temps). */
	release(): boolean {
		const typed = this.down && this.pending;
		this.down = false;
		this.pending = false;
		return typed;
	}

	/** Autre touche pendant qu'Espace est tenue : true si l'espace en attente est à saisir d'abord. */
	flush(): boolean {
		const p = this.pending;
		this.pending = false;
		return p;
	}

	/** Fenêtre quittée : plus rien de tenu. */
	reset() {
		this.down = false;
		this.pending = false;
	}
}
