// Info-bulles de touches (KeyTips) du ruban, comme Excel : Alt (ou F10)
// affiche une lettre sur chaque onglet, puis sur chaque commande de
// l'onglet choisi. Les codes ne sont jamais préfixes l'un de l'autre : une
// touche tapée désigne une commande, ou annonce un code à deux touches.

/** Lettres des onglets du ruban (celles d'Excel quand elles existent). */
export const TAB_TIPS: Record<string, string> = {
	Fichier: 'F',
	Accueil: 'H',
	Insertion: 'N',
	'Mise en page': 'P',
	Données: 'A',
	Formules: 'M',
	Révision: 'R',
	Affichage: 'W',
	'Assistant ✦': 'I',
	SQL: 'Q',
	SCRIPT: 'L',
	Aide: 'Y'
};

/** Codes d'Excel repris pour les commandes qui ont un équivalent (onglet Accueil surtout). */
export const COMMAND_TIPS: Record<string, string> = {
	Annuler: 'Z',
	Rétablir: 'Y',
	Coller: 'V',
	Couper: 'X',
	Copier: 'C',
	Reproduire: 'FP',
	Police: 'FF',
	'Taille de police (points)': 'FS',
	Gras: '1',
	Italique: '2',
	Souligné: '3',
	'Couleur de police': 'FC',
	'Couleur de remplissage': 'H',
	'Police …': 'FN',
	Haut: 'AT',
	Milieu: 'AM',
	Bas: 'AB',
	Gauche: 'AL',
	Centrer: 'AC',
	Droite: 'AR',
	'Renvoyer à la ligne': 'W',
	'Fusionner et centrer': 'M',
	'Alignement …': 'FA',
	'Format de nombre': 'N',
	Monétaire: 'AN',
	Pourcentage: 'P',
	'Ajouter une décimale': '0',
	'Réduire les décimales': '9',
	'Nombre …': 'FM',
	'Mise en forme conditionnelle': 'L',
	'Mettre sous forme de tableau': 'T',
	Insérer: 'I',
	Supprimer: 'D',
	Format: 'O',
	Remplissage: 'FI',
	Effacer: 'E',
	'Trier et filtrer': 'S',
	Rechercher: 'FD',
	'Tableau croisé dynamique': 'V',
	Tableau: 'T',
	'Somme automatique': 'U',
	'Insérer une fonction': 'F',
	'Supprimer les flèches': 'AA',
	Quadrillage: 'VG',
	'Figer les volets': 'F',
	Zoom: 'Q',
	// Mise en page
	Marges: 'M',
	Orientation: 'O',
	Taille: 'SZ',
	"Zone d'impression": 'R',
	'Sauts de page': 'B',
	'Imprimer les titres': 'I',
	Largeur: 'W',
	Hauteur: 'H',
	Échelle: 'SC',
	'Afficher le quadrillage': 'VG',
	'Imprimer le quadrillage': 'PG',
	'Afficher les en-têtes': 'VH',
	'Imprimer les en-têtes': 'PH'
};

/** Lettres et chiffres d'un libellé, sans accents, en majuscules. */
export function keyChars(label: string): string {
	return label
		.normalize('NFD')
		.replace(/[̀-ͯ]/g, '')
		.toUpperCase()
		.replace(/[^A-Z0-9]/g, '');
}

/** Normalise une touche tapée (é → E) ; '' si ce n'est ni une lettre ni un chiffre. */
export function keyOf(key: string): string {
	return key.length === 1 ? keyChars(key) : '';
}

/** Initiales des mots après le premier (« Trier Z → A » → ZA). */
function initials(label: string): string {
	return label
		.split(/[\s'’/-]+/)
		.slice(1)
		.map((w) => keyChars(w)[0] ?? '')
		.join('');
}

const ALPHA = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';

/** Codes à deux touches possibles pour un libellé, du plus parlant au plus arbitraire. */
function* candidates(label: string): Generator<string> {
	const w = keyChars(label);
	const c = w[0] ?? 'X';
	for (const s of initials(label)) yield c + s;
	for (const s of w.slice(1)) yield c + s;
	for (const d of '123456789') yield c + d;
	for (let i = 1; i < w.length; i++) for (const s of w.slice(i + 1)) yield w[i] + s;
	for (const a of ALPHA) for (const b of ALPHA + '0123456789') yield a + b;
}

/**
 * Codes des libellés (même ordre) : le code préféré s'il est libre, sinon
 * l'initiale seule quand aucun autre libellé ne commence par elle, sinon
 * l'initiale et une lettre suivante. Aucun code n'est préfixe d'un autre.
 */
export function assignKeyTips(labels: string[], preferred: Record<string, string> = {}): string[] {
	const out = labels.map(() => '');
	const used: string[] = [];
	const free = (t: string) => !!t && used.every((u) => !u.startsWith(t) && !t.startsWith(u));
	const take = (i: number, t: string) => {
		out[i] = t;
		used.push(t);
	};
	labels.forEach((l, i) => {
		const p = preferred[l];
		if (p && free(p)) take(i, p);
	});
	// Libellés restants, groupés par initiale (dans l'ordre d'apparition).
	const groups = new Map<string, number[]>();
	labels.forEach((l, i) => {
		if (out[i]) return;
		const c = keyChars(l)[0] ?? 'X';
		groups.set(c, [...(groups.get(c) ?? []), i]);
	});
	for (const [c, idx] of groups) {
		if (idx.length === 1 && free(c)) {
			take(idx[0], c);
			continue;
		}
		for (const i of idx) {
			for (const t of candidates(labels[i])) {
				if (free(t)) {
					take(i, t);
					break;
				}
			}
		}
	}
	return out;
}

/** Effet des touches tapées : code atteint (indice) ou -1, et s'il reste des codes qui commencent ainsi. */
export function matchTip(tips: string[], typed: string): { hit: number; live: boolean } {
	return { hit: typed ? tips.indexOf(typed) : -1, live: tips.some((t) => t.startsWith(typed)) };
}
