<script lang="ts">
	// Bandeau du classeur partagé ouvert (§5.4, §7, §10) : « Nouvelle version
	// disponible » (Recharger), serveur injoignable (texte lisible, erreur
	// technique dans « Détails » ; Réessayer, Travailler sur une copie
	// locale, Détacher du serveur pour le propriétaire), classeur supprimé du
	// serveur ou plus accessible (Enregistrer une copie locale…, Retirer des
	// récents), « Classeur publié en lecture seule », conflits d'un classeur
	// collaboratif (Voir).
	import Icon from '#lib/icons/Icon.svelte';
	import type { State } from '#bindings/internal/remote/workbook/models.js';
	import { banner, type BannerAction } from './share';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		share: State | null;
		busy?: boolean;
		/** Nouvel essai de connexion en cours (« Réessayer »). */
		retrying?: boolean;
		onreload: () => void;
		onlogin: () => void;
		onconflicts?: () => void;
		onretry?: () => void;
		oncopy?: () => void;
		ondetach?: () => void;
		/** Classeur disparu du serveur : « Retirer des récents ». */
		onforget?: () => void;
	}
	let { share, busy = false, retrying = false, onreload, onlogin, onconflicts, onretry, oncopy, ondetach, onforget }: Props = $props();
	const b = $derived(banner(share));
	let details = $state(false);

	const handlers = $derived<Record<BannerAction, (() => void) | undefined>>({
		reload: onreload,
		login: onlogin,
		conflicts: onconflicts,
		retry: onretry,
		copy: oncopy,
		detach: ondetach,
		forget: onforget
	});

	function label(a: BannerAction, text: string): string {
		if (a === 'reload' && busy) return t('remote.banner.reloading');
		if (a === 'retry' && retrying) return t('remote.banner.connecting');
		return text;
	}
</script>

{#if b}
	<div class="banner {b.kind}" role="status">
		<div class="line">
			<Icon name={b.kind === 'new' ? 'refresh' : b.kind === 'conflict' || b.kind === 'gone' ? 'warning' : b.kind === 'offline' ? 'cloud-off' : 'lock'} size={14} />
			<span class="text" title={b.detail ? `${b.text}\n\n${b.detail}` : b.text}>{b.text}</span>
			{#if b.detail}
				<button class="link" aria-expanded={details} onclick={() => (details = !details)}>{details ? t('remote.banner.hideDetails') : t('remote.banner.details')}</button>
			{/if}
			{#each b.actions ?? [] as a (a.action)}
				{#if handlers[a.action]}
					<button disabled={(a.action === 'reload' && busy) || (a.action === 'retry' && retrying)} onclick={handlers[a.action]}>{label(a.action, a.label)}</button>
				{/if}
			{/each}
		</div>
		{#if details && b.detail}
			<div class="detail">{b.detail}</div>
		{/if}
	</div>
{/if}

<style>
	.banner {
		padding: 3px 12px;
		font-size: 12px;
		border-bottom: 1px solid #e3e5e8;
		background: #f3f6f4;
		color: #374151;
		--icon-accent: #217346;
	}
	.line {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.banner.new {
		background: #e8f3ec;
		color: #14532d;
	}
	.banner.conflict {
		background: #fdf0e3;
		color: #7c3a03;
		--icon-accent: #d97706;
	}
	.banner.offline,
	.banner.gone {
		background: #fdf3e1;
		color: #7c4a03;
		--icon-accent: #b45309;
	}
	.text {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.detail {
		margin: 2px 0 2px 22px;
		font-family: ui-monospace, Consolas, monospace;
		font-size: 11px;
		white-space: pre-wrap;
		word-break: break-word;
		user-select: text;
		opacity: 0.85;
	}
	button {
		font: inherit;
		padding: 1px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
		white-space: nowrap;
	}
	button.link {
		border: none;
		background: none;
		padding: 1px 4px;
		color: inherit;
		text-decoration: underline;
	}
	button:disabled {
		opacity: 0.55;
	}
</style>
