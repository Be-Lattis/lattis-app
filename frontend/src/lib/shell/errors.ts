// Erreurs de l'interface : journalisées par le serveur (DocumentService.
// ReportError : console et erreurs.log du poste) avec la pile et les
// dernières actions de l'utilisateur, au lieu d'un écran 500 sans trace.
import { DocumentService } from '#bindings/internal/bridge/index.js';

const MAX_TRAIL = 15;
const trail: string[] = [];

/** Note une action de l'utilisateur (clic sur un bouton, touche, commande). */
export function noteAction(what: string) {
	trail.push(`${new Date().toLocaleTimeString('fr-FR')} ${what}`);
	if (trail.length > MAX_TRAIL) trail.shift();
}

/** Dernières actions, une par ligne (la plus récente en dernier). */
export function lastActions(): string {
	return trail.join('\n');
}

/** Message et pile d'une erreur quelconque. */
export function describe(e: unknown): { message: string; stack: string } {
	if (e instanceof Error) return { message: `${e.name}: ${e.message}`, stack: e.stack ?? '' };
	// Objet qui n'est pas une Error (rejet d'une liaison, évènement d'erreur…) :
	// son message, l'erreur qu'il porte, ou son contenu, plutôt que « [object Object] ».
	if (e && typeof e === 'object') {
		const o = e as { error?: unknown; reason?: unknown; message?: unknown; stack?: unknown };
		if (o.error instanceof Error) return describe(o.error);
		if (o.reason instanceof Error) return describe(o.reason);
		if (typeof o.message === 'string' && o.message) return { message: o.message, stack: typeof o.stack === 'string' ? o.stack : '' };
		try {
			const s = JSON.stringify(e);
			if (s && s !== '{}') return { message: s.slice(0, 2000), stack: '' };
		} catch {
			// objet circulaire : son type, ci-dessous
		}
		return { message: Object.prototype.toString.call(e) + (o.constructor?.name ? ` (${o.constructor.name})` : ''), stack: '' };
	}
	return { message: String(e), stack: '' };
}

/** Journalise une erreur ; rend le chemin du journal (vide s'il manque). */
export async function reportError(where: string, e: unknown): Promise<string> {
	const { message, stack } = describe(e);
	console.error(`[${where}]`, e);
	try {
		return (await DocumentService.ReportError(where, message, stack, lastActions())) ?? '';
	} catch {
		return '';
	}
}

/** Libellé court d'un élément cliqué (texte du bouton, titre ou nom). */
function label(el: Element | null): string {
	const b = el?.closest('button, [role="menuitem"], a, [role="tab"], input, select');
	if (!b) return el?.tagName.toLowerCase() ?? '?';
	const t = (b.getAttribute('aria-label') || b.getAttribute('title') || b.textContent || b.tagName).trim().replace(/\s+/g, ' ');
	return t.slice(0, 60);
}

/** Branche le suivi des actions et des erreurs non rattrapées (une fois). */
export function installErrorReporting() {
	if (typeof window === 'undefined') return;
	window.addEventListener('pointerdown', (e) => noteAction(`clic « ${label(e.target as Element)} »`), true);
	window.addEventListener(
		'keydown',
		(e) => {
			if (e.ctrlKey || e.altKey || e.metaKey || e.key.length > 1) noteAction(`touche ${[e.ctrlKey && 'Ctrl', e.altKey && 'Alt', e.shiftKey && 'Maj', e.key].filter(Boolean).join('+')}`);
		},
		true
	);
	window.addEventListener('error', (e) => void reportError('fenêtre', e.error ?? e.message));
	window.addEventListener('unhandledrejection', (e) => void reportError('promesse', e.reason));
}
