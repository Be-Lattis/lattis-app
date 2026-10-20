// Texte d'une erreur à montrer. Une erreur du moteur Go née d'un message du
// catalogue (internal/msg) arrive avec une cause { id, args, message } :
// message est le texte dans la langue de la session (bridge.MarshalError
// sur le bureau, transport web). Les autres erreurs gardent leur texte.

interface MsgCause {
	id: string;
	args?: Record<string, unknown>;
	message: string;
}

/** Cause d'un message traduit portée par une erreur d'appel au moteur, s'il y en a une. */
export function msgCause(e: unknown): MsgCause | null {
	let c = e && typeof e === 'object' ? (e as { cause?: unknown }).cause : undefined;
	if (typeof c === 'string') {
		try {
			c = JSON.parse(c);
		} catch {
			return null;
		}
	}
	if (c && typeof c === 'object' && typeof (c as MsgCause).id === 'string' && typeof (c as MsgCause).message === 'string') return c as MsgCause;
	return null;
}

/**
 * Texte d'une erreur pour l'utilisateur : le message traduit d'une erreur
 * du moteur, sinon le message de l'erreur (à la place de
 * e instanceof Error ? e.message : String(e)).
 */
export function errorText(e: unknown): string {
	const c = msgCause(e);
	if (c) return c.message;
	if (e instanceof Error) return e.message;
	return String(e);
}
