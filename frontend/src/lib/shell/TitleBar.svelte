<script lang="ts">
	// Barre de titre dessinée par l'application (fenêtre sans cadre, voir
	// main.go) : logo, accès rapide, nom du classeur, boutons de la fenêtre.
	//
	// Windows : les zones sont déclarées au système par
	// --wails-non-client-region. La barre se comporte comme une vraie barre de
	// titre (glisser, double-clic, Win+flèches, menu système au clic droit) et
	// le survol de « Agrandir » ouvre les dispositions d'ancrage de Windows 11.
	// Le clic sur les boutons est renvoyé à la page, d'où les onclick.
	// Linux : ces régions n'existent pas ; --wails-draggable fait glisser la
	// fenêtre et le double-clic est traité ici.
	// macOS : on garde les feux tricolores natifs (barre masquée, contenu
	// pleine hauteur) ; la barre laisse leur place à gauche.
	import { onMount } from 'svelte';
	import { System, Window } from '@wailsio/runtime';
	import Icon from '#lib/icons/Icon.svelte';
	import type { IconName } from '#lib/icons/icons.js';

	interface QuickAction {
		label: string;
		icon: IconName;
		run?: () => void;
	}

	interface Props {
		title: string;
		dirty?: boolean;
		actions?: QuickAction[];
	}

	let { title, dirty = false, actions = [] }: Props = $props();

	let mac = $state(false);
	let linux = $state(false);
	let maximised = $state(false);

	const refresh = () =>
		Window.IsMaximised()
			.then((m) => (maximised = m))
			.catch(() => {});

	onMount(() => {
		mac = System.IsMac();
		linux = System.IsLinux();
		refresh();
		// Agrandir, restaurer ou ancrer redimensionne la page : on relit l'état.
		window.addEventListener('resize', refresh);
		return () => window.removeEventListener('resize', refresh);
	});

	const ignore = () => {};
	const minimise = () => Window.Minimise().catch(ignore);
	const toggle = () => Window.ToggleMaximise().then(refresh, ignore);
	const close = () => Window.Close().catch(ignore);

	function dblclick() {
		// Sous Windows le système gère le double-clic sur la zone de titre.
		if (linux) toggle();
	}
</script>

<div class="titlebar" class:mac>
	<!-- Zones de titre et boutons ne doivent pas se chevaucher : chaque zone
	     déclarée est un rectangle pour le système. -->
	<div class="caption logo" role="presentation" ondblclick={dblclick}><Icon name="logo" size={18} /></div>
	<div class="quick">
		{#each actions as a (a.label)}
			<button class="quick-btn" disabled={!a.run} onclick={a.run} title={a.run ? a.label : `${a.label} : à venir`}>
				<Icon name={a.icon} size={16} />
			</button>
		{/each}
	</div>
	<div class="caption title" role="presentation" ondblclick={dblclick}>
		<span class="name">{title}</span>{#if dirty}<span class="muted" title="Modifications non enregistrées">&nbsp;•</span>{/if}
		<span class="muted">{title ? '\u00a0– Slitable' : 'Slitable'}</span>
	</div>
	{#if !mac}
		<div class="controls">
			<button class="ctl min" title="Réduire" onclick={minimise}><Icon name="win-minimize" size={16} /></button>
			<button class="ctl max" title={maximised ? 'Niveau inférieur' : 'Agrandir'} onclick={toggle}>
				<Icon name={maximised ? 'win-restore' : 'win-maximize'} size={16} />
			</button>
			<button class="ctl close" title="Fermer" onclick={close}><Icon name="win-close" size={16} /></button>
		</div>
	{/if}
</div>

<style>
	.titlebar {
		display: flex;
		height: 32px;
		background: #1b5e3a;
		color: #fff;
		font: 12px 'Segoe UI', system-ui, -apple-system, sans-serif;
		user-select: none;
	}
	.caption {
		display: flex;
		align-items: center;
		--wails-non-client-region: caption;
		--wails-draggable: drag;
	}
	.logo {
		padding: 0 8px 0 10px;
		--icon-accent: #fff;
		--icon-contrast: #1b5e3a;
	}
	.mac .logo {
		padding-left: 80px; /* feux tricolores */
	}
	.quick {
		display: flex;
		align-items: center;
		gap: 1px;
	}
	.quick-btn,
	.ctl {
		display: flex;
		align-items: center;
		justify-content: center;
		border: none;
		background: none;
		color: inherit;
		padding: 0;
		cursor: default;
	}
	.quick-btn {
		width: 28px;
		height: 26px;
		border-radius: 4px;
		--icon-accent: #bfe3cc;
	}
	.quick-btn:hover:not(:disabled) {
		background: rgba(255, 255, 255, 0.15);
	}
	.quick-btn:disabled {
		opacity: 0.45;
	}
	.title {
		flex: 1;
		min-width: 0;
		justify-content: center;
		white-space: nowrap;
		padding: 0 8px;
	}
	.name {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.muted {
		opacity: 0.75;
	}
	.controls {
		display: flex;
	}
	.ctl {
		width: 46px;
		height: 32px;
	}
	.ctl:hover {
		background: rgba(255, 255, 255, 0.15);
	}
	.ctl:active {
		background: rgba(255, 255, 255, 0.25);
	}
	.ctl.close:hover {
		background: #c42b1c;
	}
	.ctl.close:active {
		background: #b22a1b;
	}
	.min {
		--wails-non-client-region: minimize;
	}
	.max {
		--wails-non-client-region: maximize;
	}
	.close {
		--wails-non-client-region: close;
	}
</style>
