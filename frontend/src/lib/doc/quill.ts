// Quill pour l'onglet Document : polices et tailles libres (en style,
// comme les cellules), et deux objets insérables propres à Lattis :
// - un graphique (bloc) alimenté par un tableau du classeur, dessiné par
//   DocChart.svelte dans le nœud du bloc ;
// - un tableau croisé dynamique (bloc), recalculé comme les graphiques
//   (DocPivot.svelte) ;
// - un bouton (dans la ligne) qui exécute un script de l'onglet SCRIPT, ou
//   l'une de ses fonctions.
// Leur configuration est la valeur de l'objet dans le Delta enregistré.

import Quill from 'quill';
import type { ChartConfig } from '#lib/grid/Sheet.svelte';

/** Bouton d'un onglet Document. */
export interface DocButtonConfig {
	label: string;
	script: string;
	function?: string;
	color?: string;
	textColor?: string;
	width?: number;
	height?: number;
}

export const CHART_BLOT = 'lattisChart';
export const BUTTON_BLOT = 'lattisButton';
export const PIVOT_BLOT = 'lattisPivot';
/** Contrôle du classeur (champ, case, liste, fichier) : { id, width?, height? }. */
export const CONTROL_BLOT = 'lattisControl';

/** Élément copié ou coupé (clic droit › Copier), collable dans tout onglet Document. */
export const docClip: { item: { type: string; value: unknown } | null } = { item: null };

let registered = false;

function readConfig<T>(node: HTMLElement): T {
	try {
		return JSON.parse(node.dataset.config ?? '{}') as T;
	} catch {
		return {} as T;
	}
}

/** Libellé affiché d'un bouton. */
export function buttonText(cfg: DocButtonConfig): string {
	return cfg.label || cfg.function || cfg.script || 'Bouton';
}

/** Dessine le bouton dans son nœud (aussi après une modification). */
export function paintButton(node: HTMLElement, cfg: DocButtonConfig) {
	node.dataset.config = JSON.stringify(cfg);
	node.textContent = buttonText(cfg);
	node.style.background = cfg.color ?? '';
	node.style.color = cfg.textColor ?? '';
	if (cfg.width) node.style.width = cfg.width + 'px';
	if (cfg.height) node.style.height = cfg.height + 'px';
	node.title = (cfg.script ? `Exécute ${cfg.function ? `${cfg.script} › ${cfg.function}()` : cfg.script}` : 'Aucune action liée') + ' · clic droit : modifier';
}

export function setupQuill(): typeof Quill {
	if (registered) return Quill;
	registered = true;
	const Font = Quill.import('attributors/style/font') as { whitelist: string[] | null };
	Font.whitelist = null;
	const Size = Quill.import('attributors/style/size') as { whitelist: string[] | null };
	Size.whitelist = null;
	Quill.register(Font as never, true);
	Quill.register(Size as never, true);
	const Align = Quill.import('attributors/style/align');
	const Color = Quill.import('attributors/style/color');
	const Background = Quill.import('attributors/style/background');
	Quill.register(Align as never, true);
	Quill.register(Color as never, true);
	Quill.register(Background as never, true);

	const BlockEmbed = Quill.import('blots/block/embed') as unknown as {
		new (...a: unknown[]): object;
		create(v?: unknown): HTMLElement;
	};
	class ChartBlot extends BlockEmbed {
		static blotName = CHART_BLOT;
		static tagName = 'DIV';
		static className = 'ql-lattis-chart';
		static create(value: ChartConfig) {
			const node = super.create() as HTMLElement;
			node.setAttribute('contenteditable', 'false');
			node.dataset.config = JSON.stringify(value);
			return node;
		}
		static value(node: HTMLElement) {
			return readConfig<ChartConfig>(node);
		}
	}

	class PivotBlot extends BlockEmbed {
		static blotName = PIVOT_BLOT;
		static tagName = 'DIV';
		static className = 'ql-lattis-pivot';
		static create(value: unknown) {
			const node = super.create() as HTMLElement;
			node.setAttribute('contenteditable', 'false');
			node.dataset.config = JSON.stringify(value);
			return node;
		}
		static value(node: HTMLElement) {
			return readConfig<Record<string, unknown>>(node);
		}
	}

	const Embed = Quill.import('blots/embed') as unknown as {
		new (...a: unknown[]): object;
		create(v?: unknown): HTMLElement;
	};
	class ButtonBlot extends Embed {
		static blotName = BUTTON_BLOT;
		static tagName = 'SPAN';
		static className = 'ql-lattis-button';
		static create(value: DocButtonConfig) {
			const node = super.create() as HTMLElement;
			paintButton(node, value);
			return node;
		}
		static value(node: HTMLElement) {
			return readConfig<DocButtonConfig>(node);
		}
	}
	class ControlBlot extends Embed {
		static blotName = CONTROL_BLOT;
		static tagName = 'SPAN';
		static className = 'ql-lattis-control';
		static create(value: unknown) {
			const node = super.create() as HTMLElement;
			node.dataset.config = JSON.stringify(value);
			return node;
		}
		static value(node: HTMLElement) {
			return readConfig<Record<string, unknown>>(node);
		}
	}
	Quill.register(ControlBlot as never, true);
	Quill.register(ChartBlot as never, true);
	Quill.register(PivotBlot as never, true);
	Quill.register(ButtonBlot as never, true);
	return Quill;
}
