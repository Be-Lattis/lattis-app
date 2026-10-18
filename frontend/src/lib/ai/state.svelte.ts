// Onglet IA : état partagé, gardé quand on passe d'un onglet à l'autre
// (le composant AiView est recréé, la conversation reste).
import { DocumentService } from '#bindings/internal/bridge/index.js';
import type { AIStatus } from '#bindings/internal/bridge/models.js';
import { Events } from '@wailsio/runtime';
import { applyEvent, dropRun, endRun, failedBeforeReply, startRun, type ChatEvent, type Item } from './chat';

export const chat = $state<{ items: Item[]; busy: boolean; draft: string; error: string; status: AIStatus | null }>({
	items: [],
	busy: false,
	draft: '',
	error: '',
	status: null
});

let run = 0;
let listening = false;

function listen() {
	if (listening) return;
	listening = true;
	Events.On('ai:event', (e: { data: ChatEvent }) => {
		chat.items = applyEvent(chat.items, e.data);
	});
}

/** Relit l'état de l'onglet (réglages, fournisseurs, clés). */
export async function refreshStatus() {
	try {
		chat.status = await DocumentService.AIState();
	} catch {
		// Pas de pont (tests).
	}
}

/** Envoie un message ; la réponse arrive par ai:event. */
export async function send(text: string) {
	text = text.trim();
	if (!text || chat.busy) return;
	listen();
	const r = ++run;
	chat.items = startRun(chat.items, r, text);
	chat.draft = '';
	chat.error = '';
	chat.busy = true;
	let error = '';
	try {
		await DocumentService.AISend(r, text);
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

/** Nouvelle conversation. */
export async function reset() {
	if (chat.busy) return;
	await DocumentService.AIReset();
	chat.items = [];
	chat.error = '';
	await refreshStatus();
}
