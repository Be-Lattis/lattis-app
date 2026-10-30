// Modèle de la fenêtre d'un classeur (routes/+page.svelte et lib/app/*).

/** Où en est la fenêtre : démarrage, récupération, classeur ouvert, démonstration, fermée. */
export type Mode = 'starting' | 'recover' | 'doc' | 'demo' | 'closed';
