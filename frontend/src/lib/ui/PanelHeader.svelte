<script lang="ts">
	// En-tête commun des volets de droite : titre (et sous-titre éventuel),
	// lien « ? » vers la page d'aide du volet, croix de fermeture (Échap).
	import type { Snippet } from 'svelte';
	import HelpLink from '#lib/help/HelpLink.svelte';

	interface Props {
		title: string;
		/** Page d'aide du volet (lien « ? »). */
		help?: string;
		helpLabel?: string;
		onclose: () => void;
		/** Sous le titre (source d'un TCD…). */
		children?: Snippet;
	}

	let { title, help, helpLabel = 'Aide', onclose, children }: Props = $props();
</script>

<header class="panel-header">
	<div class="titles">
		<strong>{title}</strong>
		{@render children?.()}
	</div>
	{#if help}<HelpLink target={help} label={helpLabel} />{/if}
	<button class="x" title="Fermer le volet (Échap)" aria-label="Fermer le volet" onclick={onclose}>✕</button>
</header>

<style>
	.panel-header {
		display: flex;
		align-items: flex-start;
		gap: 6px;
		font-size: 13px;
	}
	.titles {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-top: 2px;
	}
	.x {
		flex-shrink: 0;
		border: none;
		background: none;
		font: inherit;
		font-size: 14px;
		line-height: 1;
		padding: 3px 6px;
		border-radius: 3px;
		color: #374151;
		cursor: pointer;
	}
	.x:hover {
		background: #e3e5e8;
	}
</style>
