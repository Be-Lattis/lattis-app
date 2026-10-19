// Accueil sans classeur ouvert : premiers pas et classeur d'exemple
// (audit 6, I8).

export type DemoProgress = { done: number; total: number; cells: number; totalCells: number };

const fr = (n: number) => n.toLocaleString('fr-FR');

/** Avancement de la fabrication d'une démonstration ou de l'exemple. */
export function demoProgressText(label: string, p: DemoProgress | null): string {
	if (!p || p.total <= 0) return `${label}…`;
	return `${label} : ${fr(p.done)} / ${fr(p.total)} lignes` + (p.totalCells ? ` · ${fr(p.cells)} / ${fr(p.totalCells)} cellules` : '');
}

/** Ce qui fait la différence de Lattis, en trois points (cible : page d'aide). */
export const firstSteps: { title: string; text: string; help: string }[] = [
	{
		title: 'Tapez un tableau',
		text: 'quelques lignes avec des en-têtes : Lattis le reconnaît et en fait une vraie table typée.',
		help: 'prise-en-main#tableaux-detectes'
	},
	{
		title: 'Calculez par nom de colonne',
		text: '=[Quantité]*[Prix] s’applique à toute la colonne, sans recopier la formule.',
		help: 'formules#colonnes-calculees'
	},
	{
		title: 'Interrogez en SQL',
		text: 'l’onglet SQL lit vos tableaux comme une base de données.',
		help: 'sql'
	}
];

const KEY = 'lattis.welcome.hidden';

/** Les premiers pas sont-ils affichés (pas masqués par « Ne plus afficher ») ? */
export function stepsShown(store: Pick<Storage, 'getItem'> | null = storage()): boolean {
	try {
		return store?.getItem(KEY) !== '1';
	} catch {
		return true;
	}
}

/** « Ne plus afficher » les premiers pas. */
export function hideSteps(store: Pick<Storage, 'setItem'> | null = storage()) {
	try {
		store?.setItem(KEY, '1');
	} catch {
		// stockage indisponible : masqué pour cette fois seulement
	}
}

function storage(): Storage | null {
	try {
		return typeof localStorage === 'undefined' ? null : localStorage;
	} catch {
		return null;
	}
}
