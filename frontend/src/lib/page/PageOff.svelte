<script lang="ts">
	// Onglet « page web » quand les pages web sont coupées (par défaut ;
	// toujours dans la version web et la démo publique) : la page n'est ni
	// exécutée ni modifiable, aucun cadre n'est créé. L'onglet et son contenu
	// restent dans le classeur ; on peut le supprimer comme un autre onglet.
	import Icon from '#lib/icons/Icon.svelte';
	import { t } from '#lib/i18n/index.js';

	let { name }: { name: string } = $props();
</script>

<div class="page">
	<div class="toolbar"><span class="title"><Icon name="web" size={14} />{name}</span></div>
	<div class="off" role="status">
		<p>{t('doc.page.off')}</p>
	</div>
</div>

<style>
	.page {
		display: flex;
		flex-direction: column;
		height: 100%;
		min-height: 0;
		font: 13px 'Segoe UI', system-ui, -apple-system, sans-serif;
		color: #1f2328;
		background: #fff;
	}
	.toolbar {
		display: flex;
		align-items: center;
		padding: 6px 10px;
		border-bottom: 1px solid #e3e5e8;
	}
	.title {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-weight: 600;
		--icon-accent: #217346;
	}
	.off {
		flex: 1;
		padding: 24px;
		background: #fafafa;
	}
	.off p {
		margin: 0;
		max-width: 640px;
		color: #4b5563;
	}
</style>
