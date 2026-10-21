// Fin d'une démo publique (contrat de lattisd, docs/collaboration/version-web.md
// § 6.2) : le moteur donne l'heure de fin (Platform.demoDeadline), le préavis
// (demoWarnBefore) et son heure (now), en millisecondes. L'interface corrige
// l'écart de son horloge, montre un bandeau avec compte à rebours pendant le
// préavis, puis, à l'heure, la boîte « Démo terminée » (classeur bloqué, le
// moteur refuse aussi les modifications ; le téléchargement reste permis).

/** Ce que Platform dit de la fin de la démo (champs absents : pas de fin annoncée). */
export interface DemoEndInfo {
	demo?: boolean;
	demoDeadline?: number;
	demoWarnBefore?: number;
	now?: number;
}

/** Fin de la démo, rapportée à l'horloge du navigateur. */
export interface DemoClock {
	/** Heure de fin, horloge du moteur (ms). */
	deadline: number;
	/** Préavis (ms) ; 0 : pas de bandeau. */
	warn: number;
	/** Écart moteur − navigateur (ms), à ajouter à Date.now(). */
	offset: number;
}

/**
 * Lit la fin annoncée ; clientNow : heure du navigateur à la réception de
 * Platform. null hors démo publique ou sans fin annoncée (lattisd plus
 * ancien) : rien de particulier.
 */
export function demoClock(p: DemoEndInfo | null | undefined, clientNow: number): DemoClock | null {
	if (!p?.demo || !p.demoDeadline || !Number.isFinite(p.demoDeadline)) return null;
	const offset = p.now && Number.isFinite(p.now) ? p.now - clientNow : 0;
	return { deadline: p.demoDeadline, warn: Math.max(0, p.demoWarnBefore ?? 0), offset };
}

export type DemoPhase = 'running' | 'warning' | 'ended';

export interface DemoEndState {
	phase: DemoPhase;
	/** Temps restant (ms, jamais négatif). */
	remaining: number;
	/** Minutes restantes, arrondies au-dessus (« dans 2 minutes »), au moins 1. */
	minutes: number;
	/** Compte à rebours « mm:ss ». */
	countdown: string;
}

/** État de la fin de la démo à l'heure clientNow du navigateur. */
export function demoEndState(c: DemoClock, clientNow: number): DemoEndState {
	const remaining = Math.max(0, c.deadline - (clientNow + c.offset));
	const phase: DemoPhase = remaining <= 0 ? 'ended' : c.warn > 0 && remaining <= c.warn ? 'warning' : 'running';
	const secs = Math.ceil(remaining / 1000);
	const countdown = `${String(Math.floor(secs / 60)).padStart(2, '0')}:${String(secs % 60).padStart(2, '0')}`;
	return { phase, remaining, minutes: Math.max(1, Math.ceil(remaining / 60000)), countdown };
}
