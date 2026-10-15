<script lang="ts">
	// Ruban façon Excel : onglets en haut, groupes de commandes en dessous.
	// SQL et SCRIPT sont des onglets à part entière : ils prennent la place de
	// la grille (précédence sur GRID). Les commandes pas encore réalisées sont
	// affichées désactivées, pour montrer l'organisation cible.

	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';

	export type View = 'grid' | 'sql' | 'script';

	interface Command {
		label: string;
		icon: IconName;
		run?: () => void;
	}
	interface Group {
		title: string;
		commands: Command[];
	}

	interface Props {
		view: View;
		onview: (v: View) => void;
		onbench: () => void;
	}

	let { view, onview, onbench }: Props = $props();

	const tabs = ['Accueil', 'Insertion', 'Données', 'Formules', 'Affichage'] as const;
	let tab = $state<(typeof tabs)[number] | 'SQL' | 'SCRIPT'>('Accueil');

	const ribbons: Record<string, Group[]> = {
		Accueil: [
			{ title: 'Presse-papiers', commands: [{ label: 'Coller', icon: 'paste' }, { label: 'Couper', icon: 'cut' }, { label: 'Copier', icon: 'copy' }] },
			{ title: 'Police', commands: [{ label: 'Gras', icon: 'bold' }, { label: 'Italique', icon: 'italic' }, { label: 'Souligné', icon: 'underline' }, { label: 'Couleur', icon: 'font-color' }] },
			{ title: 'Alignement', commands: [{ label: 'Gauche', icon: 'align-left' }, { label: 'Centrer', icon: 'align-center' }, { label: 'Droite', icon: 'align-right' }] },
			{ title: 'Nombre', commands: [{ label: 'Format', icon: 'number-format' }, { label: 'Monétaire', icon: 'currency' }, { label: 'Pourcentage', icon: 'percent' }] },
			{ title: 'Édition', commands: [{ label: 'Trier et filtrer', icon: 'sort' }, { label: 'Rechercher', icon: 'search' }] }
		],
		Insertion: [
			{ title: 'Tableaux', commands: [{ label: 'Tableau', icon: 'table' }, { label: 'Tableau croisé', icon: 'pivot-table' }] },
			{ title: 'Graphiques', commands: [{ label: 'Barres', icon: 'chart-bar' }, { label: 'Courbes', icon: 'chart-line' }, { label: 'Secteurs', icon: 'chart-pie' }] }
		],
		Données: [
			{ title: 'Trier et filtrer', commands: [{ label: 'Trier', icon: 'sort' }, { label: 'Filtrer', icon: 'filter' }] },
			{ title: 'Outils de données', commands: [{ label: 'Supprimer les doublons', icon: 'remove-duplicates' }, { label: 'Convertir le type', icon: 'convert-type' }, { label: 'Erreurs seulement', icon: 'warning' }] }
		],
		Formules: [
			{ title: 'Bibliothèque', commands: [{ label: 'Insérer une fonction', icon: 'function' }, { label: 'Somme automatique', icon: 'autosum' }, { label: 'REFERENCE', icon: 'reference' }] },
			{ title: 'Audit', commands: [{ label: 'Antécédents', icon: 'precedents' }, { label: 'Dépendants', icon: 'dependents' }, { label: 'Graphe', icon: 'dependency-graph' }] }
		],
		Affichage: [
			{ title: 'Afficher', commands: [{ label: 'Figer les volets', icon: 'freeze-panes' }, { label: 'Zoom', icon: 'zoom' }] },
			{ title: 'Mesures (prototype)', commands: [{ label: 'Mesurer la grille', icon: 'stopwatch', run: () => onbench() }] }
		]
	};

	function select(t: typeof tab) {
		tab = t;
		onview(t === 'SQL' ? 'sql' : t === 'SCRIPT' ? 'script' : 'grid');
	}

	$effect(() => {
		// Revenir à la grille depuis la page (onglet de feuille) remet un onglet de ruban.
		if (view === 'grid' && (tab === 'SQL' || tab === 'SCRIPT')) tab = 'Accueil';
	});
</script>

<header class="ribbon">
	<nav class="tabs">
		<button class="tab file" disabled title="À venir : nouveau, ouvrir, enregistrer (.slitable)">Fichier</button>
		{#each tabs as t (t)}
			<button class="tab" class:on={tab === t} onclick={() => select(t)}>{t}</button>
		{/each}
		<span class="sep"></span>
		<button class="tab special" class:on={tab === 'SQL'} onclick={() => select('SQL')}>SQL</button>
		<button class="tab special" class:on={tab === 'SCRIPT'} onclick={() => select('SCRIPT')}>SCRIPT</button>
	</nav>
	{#if tab !== 'SQL' && tab !== 'SCRIPT'}
		<div class="groups">
			{#each ribbons[tab] as g (g.title)}
				<section class="group">
					<div class="commands">
						{#each g.commands as c (c.label)}
							<button class="cmd" disabled={!c.run} onclick={c.run} title={c.run ? c.label : `${c.label} : à venir`}>
								<Icon name={c.icon} />
								<span class="label">{c.label}</span>
							</button>
						{/each}
					</div>
					<div class="title">{g.title}</div>
				</section>
			{/each}
		</div>
	{/if}
</header>

<style>
	.ribbon {
		background: #f3f4f6;
		border-bottom: 1px solid #d4d7db;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		user-select: none;
	}
	.tabs {
		display: flex;
		align-items: stretch;
		height: 32px;
		background: #217346;
		padding: 0 8px 0 4px;
		gap: 2px;
	}
	.tab {
		font: inherit;
		color: #e8f3ec;
		background: none;
		border: none;
		padding: 0 14px;
		cursor: pointer;
		border-radius: 4px 4px 0 0;
		margin-top: 4px;
	}
	.tab:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.12);
	}
	.tab.on {
		background: #f3f4f6;
		color: #14532d;
		font-weight: 600;
	}
	.tab:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.special {
		letter-spacing: 0.04em;
	}
	.sep {
		flex: 1;
	}
	.groups {
		display: flex;
		height: 78px;
		padding: 4px 6px;
		overflow-x: auto;
	}
	.group {
		display: flex;
		flex-direction: column;
		padding: 0 8px;
		border-right: 1px solid #d4d7db;
	}
	.commands {
		display: flex;
		gap: 2px;
		flex: 1;
	}
	.cmd {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
		min-width: 54px;
		padding: 2px 6px;
		font: inherit;
		font-size: 11px;
		background: none;
		border: 1px solid transparent;
		border-radius: 3px;
		cursor: pointer;
		color: #1f2328;
	}
	.cmd:hover:not(:disabled) {
		background: #e2e8e4;
		border-color: #c5d6cb;
	}
	.cmd:disabled {
		color: #9aa0a6;
		cursor: default;
	}
	.cmd {
		--icon-accent: #217346;
	}
	.cmd:disabled {
		/* Une commande désactivée perd sa couleur d'accent. */
		--icon-accent: currentColor;
	}
	.title {
		text-align: center;
		font-size: 11px;
		color: #6b7280;
	}
</style>
