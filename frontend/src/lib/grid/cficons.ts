// Jeux d'icônes de la mise en forme conditionnelle (comme ceux d'Excel),
// dessinés sur le canvas de la grille et dans les aperçus du ruban.
import { t } from '#lib/i18n/index.js';

export type IconSetId =
	| '3arrows'
	| '3arrowsgray'
	| '3triangles'
	| '4arrows'
	| '4arrowsgray'
	| '5arrows'
	| '5arrowsgray'
	| '3traffic'
	| '3trafficrim'
	| '3signs'
	| '4traffic'
	| '4redblack'
	| '3symbols'
	| '3symbols2'
	| '3flags'
	| '3stars'
	| '4rating'
	| '5rating'
	| '5quarters'
	| '5boxes';

/** Une icône : forme, variante (direction, niveau) et couleur. */
export interface IconSpec {
	shape: 'arrow' | 'tri' | 'dash' | 'circle' | 'rim' | 'flag' | 'diamond' | 'warn' | 'check' | 'excl' | 'cross' | 'ccheck' | 'cexcl' | 'ccross' | 'star' | 'bars' | 'quarter' | 'boxes';
	/** Flèche : angle en degrés (90 : vers le haut) ; étoile, barres, quartiers, cases : niveau. */
	n?: number;
	color?: string;
}

const RED = '#e0442f';
const YEL = '#f2b705';
const GRN = '#3aa655';
const GRAY = '#8c8c8c';
const BLACK = '#3a3a3a';
const PINK = '#f4a6a6';
const BLUE = '#4472c4';

const arrow = (n: number, color: string): IconSpec => ({
	shape: 'arrow',
	n,
	color
});

/** Jeux d'icônes, icônes de la plus basse à la plus haute valeur. */
export const ICON_SETS: Record<IconSetId, { label: string; icons: IconSpec[] }> = {
	'3arrows': {
		get label() {
			return t('grid.cf.iconSet.3arrows');
		},
		icons: [arrow(-90, RED), arrow(0, YEL), arrow(90, GRN)]
	},
	'3arrowsgray': {
		get label() {
			return t('grid.cf.iconSet.3arrowsgray');
		},
		icons: [arrow(-90, GRAY), arrow(0, GRAY), arrow(90, GRAY)]
	},
	'3triangles': {
		get label() {
			return t('grid.cf.iconSet.3triangles');
		},
		icons: [
			{ shape: 'tri', n: -1, color: RED },
			{ shape: 'dash', color: YEL },
			{ shape: 'tri', n: 1, color: GRN }
		]
	},
	'4arrows': {
		get label() {
			return t('grid.cf.iconSet.4arrows');
		},
		icons: [arrow(-90, RED), arrow(-45, YEL), arrow(45, YEL), arrow(90, GRN)]
	},
	'4arrowsgray': {
		get label() {
			return t('grid.cf.iconSet.4arrowsgray');
		},
		icons: [arrow(-90, GRAY), arrow(-45, GRAY), arrow(45, GRAY), arrow(90, GRAY)]
	},
	'5arrows': {
		get label() {
			return t('grid.cf.iconSet.5arrows');
		},
		icons: [arrow(-90, RED), arrow(-45, YEL), arrow(0, YEL), arrow(45, YEL), arrow(90, GRN)]
	},
	'5arrowsgray': {
		get label() {
			return t('grid.cf.iconSet.5arrowsgray');
		},
		icons: [arrow(-90, GRAY), arrow(-45, GRAY), arrow(0, GRAY), arrow(45, GRAY), arrow(90, GRAY)]
	},
	'3traffic': {
		get label() {
			return t('grid.cf.iconSet.3traffic');
		},
		icons: [
			{ shape: 'circle', color: RED },
			{ shape: 'circle', color: YEL },
			{ shape: 'circle', color: GRN }
		]
	},
	'3trafficrim': {
		get label() {
			return t('grid.cf.iconSet.3trafficrim');
		},
		icons: [
			{ shape: 'rim', color: RED },
			{ shape: 'rim', color: YEL },
			{ shape: 'rim', color: GRN }
		]
	},
	'3signs': {
		get label() {
			return t('grid.cf.iconSet.3signs');
		},
		icons: [
			{ shape: 'diamond', color: RED },
			{ shape: 'warn', color: YEL },
			{ shape: 'circle', color: GRN }
		]
	},
	'4traffic': {
		get label() {
			return t('grid.cf.iconSet.4traffic');
		},
		icons: [
			{ shape: 'circle', color: BLACK },
			{ shape: 'circle', color: RED },
			{ shape: 'circle', color: YEL },
			{ shape: 'circle', color: GRN }
		]
	},
	'4redblack': {
		get label() {
			return t('grid.cf.iconSet.4redblack');
		},
		icons: [
			{ shape: 'circle', color: BLACK },
			{ shape: 'circle', color: GRAY },
			{ shape: 'circle', color: PINK },
			{ shape: 'circle', color: RED }
		]
	},
	'3symbols': {
		get label() {
			return t('grid.cf.iconSet.3symbols');
		},
		icons: [
			{ shape: 'ccross', color: RED },
			{ shape: 'cexcl', color: YEL },
			{ shape: 'ccheck', color: GRN }
		]
	},
	'3symbols2': {
		get label() {
			return t('grid.cf.iconSet.3symbols2');
		},
		icons: [
			{ shape: 'cross', color: RED },
			{ shape: 'excl', color: YEL },
			{ shape: 'check', color: GRN }
		]
	},
	'3flags': {
		get label() {
			return t('grid.cf.iconSet.3flags');
		},
		icons: [
			{ shape: 'flag', color: RED },
			{ shape: 'flag', color: YEL },
			{ shape: 'flag', color: GRN }
		]
	},
	'3stars': {
		get label() {
			return t('grid.cf.iconSet.3stars');
		},
		icons: [
			{ shape: 'star', n: 0 },
			{ shape: 'star', n: 0.5 },
			{ shape: 'star', n: 1 }
		]
	},
	'4rating': {
		get label() {
			return t('grid.cf.iconSet.4rating');
		},
		icons: [1, 2, 3, 4].map((n) => ({ shape: 'bars' as const, n }))
	},
	'5rating': {
		get label() {
			return t('grid.cf.iconSet.5rating');
		},
		icons: [0, 1, 2, 3, 4].map((n) => ({ shape: 'bars' as const, n }))
	},
	'5quarters': {
		get label() {
			return t('grid.cf.iconSet.5quarters');
		},
		icons: [0, 1, 2, 3, 4].map((n) => ({ shape: 'quarter' as const, n }))
	},
	'5boxes': {
		get label() {
			return t('grid.cf.iconSet.5boxes');
		},
		icons: [0, 1, 2, 3, 4].map((n) => ({ shape: 'boxes' as const, n }))
	}
};

/** Galerie du ruban, par famille (comme Excel) ; libellés lus à chaque usage (langue). */
export const ICON_GROUPS: { label: string; sets: IconSetId[] }[] = [
	{
		get label() {
			return t('grid.cf.iconGroup.directional');
		},
		sets: ['3arrows', '3arrowsgray', '3triangles', '4arrows', '4arrowsgray', '5arrows', '5arrowsgray']
	},
	{
		get label() {
			return t('grid.cf.iconGroup.shapes');
		},
		sets: ['3traffic', '3trafficrim', '3signs', '4traffic', '4redblack']
	},
	{ get label() {
			return t('grid.cf.iconGroup.indicators');
		}, sets: ['3symbols', '3symbols2', '3flags'] },
	{
		get label() {
			return t('grid.cf.iconGroup.ratings');
		},
		sets: ['3stars', '4rating', '5quarters', '5rating', '5boxes']
	}
];

/** Dessine l'icône dans le carré (x, y, s). */
export function drawCfIcon(ctx: CanvasRenderingContext2D, ic: IconSpec, x: number, y: number, s: number) {
	const c = ic.color ?? BLUE;
	const cx = x + s / 2;
	const cy = y + s / 2;
	const r = s / 2 - 1;
	ctx.save();
	ctx.lineJoin = 'round';
	ctx.lineCap = 'round';
	ctx.fillStyle = c;
	ctx.strokeStyle = c;
	const circle = (rad = r) => {
		ctx.beginPath();
		ctx.arc(cx, cy, rad, 0, Math.PI * 2);
	};
	const glyph = (shape: 'check' | 'excl' | 'cross', color: string, k: number) => {
		ctx.strokeStyle = color;
		ctx.fillStyle = color;
		ctx.lineWidth = Math.max(1.5, s / 7);
		ctx.beginPath();
		if (shape === 'check') {
			ctx.moveTo(cx - k * 0.55, cy);
			ctx.lineTo(cx - k * 0.1, cy + k * 0.45);
			ctx.lineTo(cx + k * 0.6, cy - k * 0.45);
			ctx.stroke();
		} else if (shape === 'cross') {
			ctx.moveTo(cx - k * 0.45, cy - k * 0.45);
			ctx.lineTo(cx + k * 0.45, cy + k * 0.45);
			ctx.moveTo(cx + k * 0.45, cy - k * 0.45);
			ctx.lineTo(cx - k * 0.45, cy + k * 0.45);
			ctx.stroke();
		} else {
			ctx.moveTo(cx, cy - k * 0.6);
			ctx.lineTo(cx, cy + k * 0.15);
			ctx.stroke();
			ctx.beginPath();
			ctx.arc(cx, cy + k * 0.55, ctx.lineWidth / 1.6, 0, Math.PI * 2);
			ctx.fill();
		}
	};
	switch (ic.shape) {
		case 'arrow': {
			ctx.translate(cx, cy);
			ctx.rotate((-(ic.n ?? 0) * Math.PI) / 180);
			const L = r;
			const h = r * 0.75;
			ctx.beginPath();
			ctx.moveTo(L, 0);
			ctx.lineTo(L - h, -h);
			ctx.lineTo(L - h, -h * 0.4);
			ctx.lineTo(-L, -h * 0.4);
			ctx.lineTo(-L, h * 0.4);
			ctx.lineTo(L - h, h * 0.4);
			ctx.lineTo(L - h, h);
			ctx.closePath();
			ctx.fill();
			break;
		}
		case 'tri': {
			const up = (ic.n ?? 1) > 0;
			ctx.beginPath();
			ctx.moveTo(cx, up ? y + 2 : y + s - 2);
			ctx.lineTo(x + 1, up ? y + s - 3 : y + 3);
			ctx.lineTo(x + s - 1, up ? y + s - 3 : y + 3);
			ctx.closePath();
			ctx.fill();
			break;
		}
		case 'dash':
			ctx.fillRect(x + 2, cy - s / 8, s - 4, s / 4);
			break;
		case 'circle':
			circle();
			ctx.fill();
			break;
		case 'rim':
			circle();
			ctx.fill();
			ctx.lineWidth = Math.max(1, s / 10);
			ctx.strokeStyle = '#3a3a3a';
			circle(r - ctx.lineWidth / 2);
			ctx.stroke();
			break;
		case 'diamond':
			ctx.beginPath();
			ctx.moveTo(cx, y + 1);
			ctx.lineTo(x + s - 1, cy);
			ctx.lineTo(cx, y + s - 1);
			ctx.lineTo(x + 1, cy);
			ctx.closePath();
			ctx.fill();
			break;
		case 'warn':
			ctx.beginPath();
			ctx.moveTo(cx, y + 1);
			ctx.lineTo(x + s - 1, y + s - 2);
			ctx.lineTo(x + 1, y + s - 2);
			ctx.closePath();
			ctx.fill();
			break;
		case 'flag':
			ctx.fillStyle = '#555';
			ctx.fillRect(x + s * 0.2, y + 1, Math.max(1, s / 10), s - 2);
			ctx.fillStyle = c;
			ctx.beginPath();
			ctx.moveTo(x + s * 0.3, y + 1);
			ctx.lineTo(x + s - 1, y + s * 0.3);
			ctx.lineTo(x + s * 0.3, y + s * 0.6);
			ctx.closePath();
			ctx.fill();
			break;
		case 'check':
		case 'excl':
		case 'cross':
			glyph(ic.shape, c, r);
			break;
		case 'ccheck':
		case 'cexcl':
		case 'ccross':
			circle();
			ctx.fill();
			glyph(ic.shape.slice(1) as 'check' | 'excl' | 'cross', '#fff', r * 0.75);
			break;
		case 'star': {
			const pts: [number, number][] = [];
			for (let i = 0; i < 10; i++) {
				const a = -Math.PI / 2 + (i * Math.PI) / 5;
				const rad = i % 2 ? r * 0.45 : r;
				pts.push([cx + rad * Math.cos(a), cy + rad * Math.sin(a)]);
			}
			const path = () => {
				ctx.beginPath();
				pts.forEach(([px, py], i) => (i ? ctx.lineTo(px, py) : ctx.moveTo(px, py)));
				ctx.closePath();
			};
			const gold = '#f2b705';
			ctx.fillStyle = '#e8e8e8';
			path();
			ctx.fill();
			const f = ic.n ?? 0;
			if (f > 0) {
				ctx.save();
				ctx.beginPath();
				ctx.rect(x, y, s * f, s);
				ctx.clip();
				ctx.fillStyle = gold;
				path();
				ctx.fill();
				ctx.restore();
			}
			ctx.strokeStyle = '#c99700';
			ctx.lineWidth = 1;
			path();
			ctx.stroke();
			break;
		}
		case 'bars': {
			const w = s / 5;
			for (let i = 0; i < 4; i++) {
				const bh = ((i + 1) * (s - 2)) / 4;
				ctx.fillStyle = i < (ic.n ?? 0) ? BLUE : '#d9d9d9';
				ctx.fillRect(x + 1 + i * (w + 0.6), y + s - 1 - bh, w, bh);
			}
			break;
		}
		case 'quarter': {
			ctx.lineWidth = 1;
			ctx.strokeStyle = BLACK;
			ctx.fillStyle = '#fff';
			circle();
			ctx.fill();
			const f = (ic.n ?? 0) / 4;
			if (f > 0) {
				ctx.fillStyle = BLACK;
				ctx.beginPath();
				ctx.moveTo(cx, cy);
				ctx.arc(cx, cy, r, -Math.PI / 2, -Math.PI / 2 + f * Math.PI * 2);
				ctx.closePath();
				ctx.fill();
			}
			circle();
			ctx.stroke();
			break;
		}
		case 'boxes': {
			const g = (s - 3) / 2;
			const order = [2, 3, 1, 0]; // bas gauche, bas droite, haut droite, haut gauche
			for (let i = 0; i < 4; i++) {
				const k = order.indexOf(i);
				ctx.fillStyle = k < (ic.n ?? 0) ? BLUE : '#d9d9d9';
				ctx.fillRect(x + 1 + (i % 2) * (g + 1), y + 1 + Math.floor(i / 2) * (g + 1), g, g);
			}
			break;
		}
	}
	ctx.restore();
}
