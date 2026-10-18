// Assistant IA : état partagé du volet (conversation, état du backend),
// gardé quand le volet se ferme et se rouvre. La conversation se relit
// depuis le backend (AITranscript) à l'ouverture du volet et sur ai:reset
// (classeur ouvert, nouveau ou fermé, conversation gardée dans le classeur).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { AIStatus } from '#bindings/internal/bridge/models.js';
import { Events } from '@wailsio/runtime';
import { applyEvent, dropRun, endRun, failedBeforeReply, markUndone, maxRun, replay, startRun, type ChatContext, type ChatEvent, type Item, type LogEntry } from './chat';

export const chat = $state<{
	items: Item[];
	busy: boolean;
	draft: string;
	error: string;
	status: AIStatus | null;
	/** Volet ouvert (bouton « Assistant ✦ », Alt+I, clic droit). */
	open: boolean;
	/** Demande de focus sur la zone de saisie (incrémentée). */
	focus: number;
	/** Messages envoyés, pour la flèche haut. */
	sent: string[];
}>({
	items: [],
	busy: false,
	draft: '',
	error: '',
	status: null,
	open: false,
	focus: 0,
	sent: []
});

let run = 0;
let listening = false;

function listen() {
	if (listening) return;
	listening = true;
	try {
		Events.On('ai:event', (e: { data: ChatEvent }) => {
			const ev = e.data;
			// Usage du tour : le coût affiché suit sans attendre la fin de la réponse.
			if (ev.kind === 'usage' && ev.usage && chat.status?.usage) {
				chat.status.usage = { ...chat.status.usage, cost: chat.status.usage.cost + (ev.usage.cost || 0) };
			}
			chat.items = applyEvent(chat.items, ev);
		});
		Events.On('ai:reset', () => {
			load();
		});
	} catch {
		// Pas de pont (tests).
	}
}

/** Relit l'état de l'assistant (réglages, fournisseurs, clés, coût). */
export async function refreshStatus() {
	try {
		chat.status = await DocumentService.AIState();
	} catch {
		// Pas de pont (tests).
	}
}

/** Relit l'état et la conversation (ouverture du volet, ai:reset). */
export async function load() {
	listen();
	await refreshStatus();
	try {
		const log = ((await DocumentService.AITranscript()) ?? []) as LogEntry[];
		// Une réponse en cours dans ce volet garde son affichage (ses évènements arrivent).
		if (chat.busy) return;
		chat.items = replay(log, !!chat.status?.busy);
		run = Math.max(run, maxRun(chat.items));
		chat.sent = chat.items.filter((it) => it.role === 'user').map((it) => (it as { text: string }).text);
	} catch {
		// Backend sans conversation à relire : l'affichage reste.
	}
}

/** Envoie un message avec ce que l'utilisateur regarde ; la réponse arrive par ai:event. */
export async function send(text: string, view: ChatContext) {
	text = text.trim();
	if (!text || chat.busy) return;
	listen();
	const r = ++run;
	chat.items = startRun(chat.items, r, text);
	chat.sent = [...chat.sent.slice(-49), text];
	chat.draft = '';
	chat.error = '';
	chat.busy = true;
	let error = '';
	try {
		await DocumentService.AISend(r, text, view);
	} catch (e) {
		error = String(e instanceof Error ? e.message : e);
	} finally {
		chat.busy = false;
	}
	// Rien n'a été répondu : le message revient dans la zone de saisie (le
	// backend l'a retiré de la conversation), l'erreur s'affiche au-dessus.
	if (error && failedBeforeReply(chat.items, r)) {
		chat.items = dropRun(chat.items, r);
		chat.error = error;
		if (!chat.draft) chat.draft = text;
	} else {
		chat.items = endRun(chat.items, r, error || undefined);
	}
	await refreshStatus();
}

/** Interrompt la réponse en cours. */
export function stop() {
	DocumentService.AIStop().catch(() => {});
}

/** Annule les modifications d'une réponse ; lève l'erreur du backend (d'autres modifications ont suivi…). */
export async function undoRun(r: number) {
	const st = await DocumentService.AIUndoRun(r);
	chat.items = markUndone(chat.items, r);
	return st;
}

/** Nouvelle conversation. */
export async function reset() {
	if (chat.busy) return;
	await DocumentService.AIReset();
	chat.items = [];
	chat.error = '';
	await refreshStatus();
}

/** Ouvre le volet et met le curseur dans la zone de saisie. */
export function openAssistant() {
	chat.open = true;
	chat.focus++;
}
