// Avancement d'un enregistrement long (gros classeur) dans la barre d'état.

import { t } from '#lib/i18n/index.js';

export type SaveProgress = { phase: string; done: number; total: number };

/** Texte de la barre d'état pendant l'enregistrement ('' : rien à dire). */
export function saveProgressText(p: SaveProgress | null): string {
	switch (p?.phase) {
		case 'snapshot':
			return t('shell.saveProgress.snapshot');
		case 'scrub':
			return t('shell.saveProgress.scrub');
		case 'write': {
			const pct = p.total > 0 ? Math.min(100, Math.floor((p.done * 100) / p.total)) : 0;
			return t('shell.saveProgress.write', { pct });
		}
		case 'verify':
			return t('shell.saveProgress.verify');
		default:
			return '';
	}
}

/**
 * Exécute un enregistrement en affichant son avancement (show) s'il dure plus
 * de `delay` ms ; poll interroge le moteur. show('') à la fin.
 */
export async function withSaveProgress<T>(
	run: () => Promise<T>,
	poll: () => Promise<SaveProgress | null>,
	show: (text: string) => void,
	delay = 400,
	every = 250
): Promise<T> {
	let active = true;
	let shown = false;
	let timer: ReturnType<typeof setTimeout> | undefined;
	const tick = async () => {
		if (!active) return;
		const text = saveProgressText(await poll().catch(() => null));
		if (!active) return;
		if (text) {
			shown = true;
			show(text);
		}
		timer = setTimeout(tick, every);
	};
	timer = setTimeout(tick, delay);
	try {
		return await run();
	} finally {
		active = false;
		clearTimeout(timer);
		if (shown) show('');
	}
}
