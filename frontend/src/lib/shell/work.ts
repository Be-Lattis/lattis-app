// Opérations longues du moteur (évènement work:step, internal/work) : la
// barre d'état dit ce qui se passe (« Suppression de la colonne Prix :
// réécriture du tableau ») au lieu de « Traitement… », et propose
// « Annuler » quand l'opération peut être interrompue sans rien laisser.

import { writable, type Readable } from 'svelte/store';
import { fmtInteger, hasKey, t, tAny } from '#lib/i18n/index.js';

/** État d'une opération (work.Step côté Go). */
export interface WorkStep {
	id: number;
	op?: string;
	name?: string;
	phase?: string;
	done?: number;
	total?: number;
	cancel?: boolean;
	end?: boolean;
}

/** Actions dont le libellé cite ce qu'elles visent ({name} : la colonne, le classeur publié). */
const NAMED = new Set(['deleteColumn', 'sort', 'setType', 'publish', 'republish']);

/** Opérations en cours, par identifiant (fonctions pures, testées). */
export class WorkList {
	private steps = new Map<number, WorkStep>();

	apply(s: WorkStep) {
		if (s.end) this.steps.delete(s.id);
		else this.steps.set(s.id, s);
	}

	/**
	 * L'opération à montrer : la plus ancienne qu'on peut annuler (une action
	 * de l'utilisateur), sinon la plus récente (lecture, mise en page).
	 */
	current(): WorkStep | null {
		let best: WorkStep | null = null;
		for (const s of this.steps.values()) {
			if (!best) best = s;
			else if (!!s.cancel !== !!best.cancel) best = s.cancel ? s : best;
			else if (s.cancel ? s.id < best.id : s.id > best.id) best = s;
		}
		return best;
	}

	get size() {
		return this.steps.size;
	}
}

/** Texte de la barre d'état pour une opération (court). */
export function workText(s: WorkStep): string {
	const opKey = `shell.work.op.${s.op}`;
	const op = s.op && hasKey(opKey) ? tAny(opKey, NAMED.has(s.op) ? { name: s.name ?? '' } : undefined) : '';
	const phase = phaseText(s);
	// Étape seule (action sans nom) : avec une majuscule, comme un libellé.
	const lone = phase && capital(phase);
	return (op && phase ? t('shell.work.both', { op, phase }) : op || lone || t('shell.busy.label')) + countText(s);
}

/**
 * Étape seule, avec une majuscule, et son avancement (« Envoi des lignes
 * 2 300 000 / 10 000 000 ») : pour une boîte qui dit déjà l'action
 * (publication). '' quand l'étape n'a pas de libellé.
 */
export function workPhaseText(s: WorkStep): string {
	const phase = phaseText(s);
	return phase ? capital(phase) + countText(s) : '';
}

function phaseText(s: WorkStep): string {
	const key = `shell.work.phase.${s.phase}`;
	return s.phase && hasKey(key) ? tAny(key) : '';
}

function capital(s: string): string {
	return s[0].toLocaleUpperCase() + s.slice(1);
}

function countText(s: WorkStep): string {
	if (s.total && s.total > 0) return ` ${fmtInteger(Math.min(s.done ?? 0, s.total))} / ${fmtInteger(s.total)}`;
	if (s.done && s.done > 0) return ` ${fmtInteger(s.done)}`;
	return '';
}

/** Part faite (0..1), ou null quand le total n'est pas connu. */
export function workRatio(s: WorkStep): number | null {
	return s.total && s.total > 0 ? Math.min(1, Math.max(0, (s.done ?? 0) / s.total)) : null;
}

const list = new WorkList();
const state = writable<WorkStep | null>(null);
/** Opération montrée dans la barre d'état (null : aucune). */
export const work: Readable<WorkStep | null> = { subscribe: state.subscribe };

/** Reçoit un évènement work:step. */
export function onWorkStep(s: WorkStep | undefined | null) {
	if (!s || typeof s.id !== 'number') return;
	list.apply(s);
	state.set(list.current());
}
