<script lang="ts">
	// Bandeau du classeur partagé ouvert (§5.4, §10) : « Nouvelle version
	// disponible » (Recharger), « Hors ligne, lecture seule », « Classeur
	// publié en lecture seule ».
	import Icon from '#lib/icons/Icon.svelte';
	import type { State } from '#bindings/internal/remote/workbook/models.js';
	import { banner } from './share';

	interface Props {
		share: State | null;
		busy?: boolean;
		onreload: () => void;
		onlogin: () => void;
	}
	let { share, busy = false, onreload, onlogin }: Props = $props();
	const b = $derived(banner(share));
</script>

{#if b}
	<div class="banner {b.kind}" role="status">
		<Icon name={b.kind === 'new' ? 'refresh' : 'lock'} size={14} />
		<span class="text">{b.text}</span>
		{#if b.action === 'reload'}
			<button disabled={busy} onclick={onreload}>{busy ? 'Rechargement…' : b.actionLabel}</button>
		{:else if b.action === 'login'}
			<button onclick={onlogin}>{b.actionLabel}</button>
		{/if}
	</div>
{/if}

<style>
	.banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 3px 12px;
		font-size: 12px;
		border-bottom: 1px solid #e3e5e8;
		background: #f3f6f4;
		color: #374151;
		--icon-accent: #217346;
	}
	.banner.new {
		background: #e8f3ec;
		color: #14532d;
	}
	.banner.offline {
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
	button {
		font: inherit;
		padding: 1px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.55;
	}
</style>
