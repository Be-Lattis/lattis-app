<script lang="ts">
	// Icône du jeu de Slitable (voir icons.ts). La taille suit size (px) ; la
	// couleur du trait suit color du parent, l'accent suit --icon-accent.
	import { icons, type IconName } from './icons.js';

	interface Props {
		name: IconName;
		size?: number;
		title?: string;
	}

	let { name, size = 24, title }: Props = $props();
</script>

<svg
	class="icon"
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width="1.5"
	stroke-linecap="round"
	stroke-linejoin="round"
	role={title ? 'img' : undefined}
	aria-hidden={title ? undefined : 'true'}
	aria-label={title}
>
	<!-- Contenu statique, défini dans icons.ts : pas de saisie de l'utilisateur. -->
	{@html icons[name]}
</svg>

<style>
	.icon {
		flex: none;
		display: block;
	}
	.icon :global(.a) {
		stroke: var(--icon-accent, currentColor);
	}
	.icon :global(.c) {
		stroke: var(--icon-contrast, #fff);
	}
	.icon :global(.af) {
		fill: var(--icon-accent, currentColor);
		stroke: none;
	}
</style>
