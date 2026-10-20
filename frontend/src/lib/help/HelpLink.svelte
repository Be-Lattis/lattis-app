<script lang="ts">
	// Lien « ? » d'un en-tête de volet : ouvre la page d'aide du volet.
	import { openHelp } from './state.svelte';
	import { t } from '#lib/i18n/index.js';

	let { target, label }: { target: string; label?: string } = $props();
</script>

<button class="help-link" title={label ?? t('help.title')} aria-label={label ?? t('help.title')} onclick={() => openHelp(target)}>?</button>

<style>
	.help-link {
		flex-shrink: 0;
		width: 20px;
		height: 20px;
		padding: 0;
		margin-left: auto;
		margin-right: 4px;
		border: 1px solid #c9cdd2;
		border-radius: 50%;
		background: #fff;
		color: #217346;
		font-family: inherit;
		font-size: 12px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
	}
	.help-link:hover {
		background: #e3efe7;
		border-color: #217346;
	}
</style>
