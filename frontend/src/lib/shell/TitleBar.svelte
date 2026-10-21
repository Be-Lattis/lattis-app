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
	// Web : pas de fenêtre à réduire, agrandir ou fermer (onglet du
	// navigateur) ; ni boutons, ni double-clic.
	import { onMount } from 'svelte';
	import { System, Window } from '@wailsio/runtime';
	import Icon from '#lib/icons/Icon.svelte';
	import { isWeb } from '#lib/web/platform.js';
	import type { IconName } from '#lib/icons/icons.js';
	import AccountMenu from '#lib/remote/AccountMenu.svelte';
	import { platform } from './settings.svelte.js';
	import { windowTitle } from './doctitle.js';
	import { t } from '#lib/i18n/index.js';

	interface QuickAction {
		label: string;
		icon: IconName;
		run?: () => void;
	}

	interface Props {
		title: string;
		dirty?: boolean;
		actions?: QuickAction[];
		/** Classeur ouvert depuis un serveur : « en ligne sur LPO » (vide : fichier local). */
		badge?: string;
		/** Infobulle du badge (adresse du classeur sur le serveur). */
		badgeTitle?: string;
		offline?: boolean;
		/** Classeur partagé : rôle de l'utilisateur (« Éditeur ») et ce qu'il permet. */
		role?: { label: string; detail: string } | null;
		/** Clic sur la pastille du rôle (Gérer l'accès, ou le détail en barre d'état). */
		onrole?: () => void;
	}

	let { title, dirty = false, actions = [], badge = '', badgeTitle = '', offline = false, role = null, onrole }: Props = $props();

	// Titre de la fenêtre du système (barre des tâches, Alt+Tab) ; onglet du
	// navigateur pour la version web.
	$effect(() => {
		const t = windowTitle(title, badge);
		if (isWeb) document.title = t;
		else Window.SetTitle(t).catch(() => {});
	});

	let mac = $state(false);
	let linux = $state(false);
	let maximised = $state(false);

	const refresh = () =>
		Window.IsMaximised()
			.then((m) => (maximised = m))
			.catch(() => {});

	onMount(() => {
		if (isWeb) return;
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

	// Double-clic sur la zone de titre : agrandir ou restaurer. Quand le
	// système traite la zone de titre (Windows), le double-clic n'arrive pas à
	// la page : il n'y a jamais de double bascule.
	const dblclick = () => (isWeb ? undefined : toggle());
</script>

<div class="titlebar" class:mac>
	<!-- Zones de titre et boutons ne doivent pas se chevaucher : chaque zone
	     déclarée est un rectangle pour le système. -->
	<div class="caption logo" role="presentation" ondblclick={dblclick}><Icon name="logo" size={18} /></div>
	<div class="quick">
		{#each actions as a, aIndex (aIndex)}
			<button class="quick-btn" disabled={!a.run} onclick={a.run} title={a.run ? a.label : t('shell.titleBar.comingSoon', { action: a.label })}>
				<Icon name={a.icon} size={16} />
			</button>
		{/each}
	</div>
	<div class="caption title" role="presentation" ondblclick={dblclick}>
		<span class="name">{title}</span>{#if dirty}<span class="muted" title={t('shell.titleBar.unsaved')}>&nbsp;•</span>{/if}
		<!-- Classeur partagé : une seule pastille « en ligne sur … | Rôle », qui ouvre l'accès (demande de Louis, 21/10). -->
		{#if badge && role}
			<button class="badge role" class:offline title={[badgeTitle, role.detail].filter(Boolean).join(' · ')} onclick={onrole}
				><Icon name={offline ? 'cloud-off' : 'cloud'} size={14} />{badge}<span class="sep" aria-hidden="true"></span>{role.label}</button
			>
		{:else if badge}<span class="badge" class:offline title={badgeTitle || undefined}><Icon name={offline ? 'cloud-off' : 'cloud'} size={14} />{badge}</span>
		{:else if role}<button class="badge role" title={role.detail} onclick={onrole}>{role.label}</button>{/if}
		<!-- Démo publique : rien n'est gardé (Ctrl+S télécharge le classeur). -->
		{#if platform.demo}<span class="badge demo" title={t('shell.titleBar.demoDetail')}>{t('shell.titleBar.demo')}</span>{/if}
		<span class="muted">{title ? '\u00a0– Lattis' : 'Lattis'}</span>
	</div>
	<AccountMenu />
	{#if !mac && !isWeb}
		<div class="controls">
			<button class="ctl min" title={t('shell.titleBar.minimize')} onclick={minimise}><Icon name="win-minimize" size={16} /></button>
			<button class="ctl max" title={maximised ? t('shell.titleBar.restore') : t('shell.titleBar.maximize')} onclick={toggle}>
				<Icon name={maximised ? 'win-restore' : 'win-maximize'} size={16} />
			</button>
			<button class="ctl close" title={t('shell.titleBar.close')} onclick={close}><Icon name="win-close" size={16} /></button>
		</div>
	{/if}
</div>
{#if !mac && !isWeb && !maximised}
	<!-- Bords de la fenêtre sans cadre : la barre de défilement de la grille, contre le bord, ne
	     transmet pas la souris à la page, et Wails n'y voyait pas la poignée de redimensionnement. -->
	<div class="edge right" aria-hidden="true"></div>
	<div class="edge bottom" aria-hidden="true"></div>
	<div class="edge left" aria-hidden="true"></div>
{/if}

<style>
	.edge {
		position: fixed;
		z-index: 2147483000;
		background: transparent;
	}
	.edge.right {
		top: 0;
		right: 0;
		bottom: 0;
		width: 5px;
	}
	.edge.left {
		top: 0;
		left: 0;
		bottom: 0;
		width: 5px;
	}
	.edge.bottom {
		left: 0;
		right: 0;
		bottom: 0;
		height: 5px;
	}
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
	.badge {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		flex-shrink: 0;
		margin-left: 8px;
		padding: 1px 7px 1px 5px;
		border-radius: 9px;
		background: rgba(255, 255, 255, 0.16);
		font-size: 11px;
		--icon-accent: #fff;
	}
	.badge.role {
		border: none;
		color: inherit;
		font: inherit;
		font-size: 11px;
		padding: 1px 7px;
		cursor: pointer;
		background: rgba(255, 255, 255, 0.1);
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.35);
	}
	.badge.role:hover {
		background: rgba(255, 255, 255, 0.22);
	}
	.badge .sep {
		align-self: stretch;
		width: 1px;
		margin: 2px 3px;
		background: rgba(255, 255, 255, 0.45);
	}
	.badge.demo {
		padding: 1px 7px;
		background: #f2c14e;
		color: #3d2e00;
		font-weight: 600;
	}
	.badge.offline {
		background: rgba(0, 0, 0, 0.22);
		opacity: 0.85;
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
