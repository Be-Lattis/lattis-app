<script lang="ts">
	// Tableaux du classeur publiés seuls comme source du Data Lake (pipeline
	// alimenté par ce classeur, sans le publier) : état et « Mettre à jour »,
	// dans les boîtes Publier sur un serveur et Gérer l'accès. Rien
	// d'affiché s'il n'y en a pas.
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { hostOf } from './lattis';
	import { pushSummary, sourceStatus, type SourceLink } from './source';
	import { errorText, t } from '#lib/i18n/index.js';

	let links = $state<SourceLink[]>([]);
	let busy = $state('');
	let said = $state('');
	const err = errorText;

	onMount(load);

	async function load() {
		links = ((await DocumentService.TableSources().catch(() => null)) ?? []) as unknown as SourceLink[];
	}

	async function push(l: SourceLink) {
		busy = l.tableName;
		said = '';
		try {
			const got = (await DocumentService.PushTableSource(l.tableName)) as unknown as SourceLink;
			said = pushSummary(l.tableName, got.lastResult, got.blanked ?? 0);
		} catch (e) {
			said = err(e);
		} finally {
			busy = '';
			await load();
		}
	}
</script>

{#if links.length}
	<h3>{t('remote.tableSources.title')}</h3>
	<ul class="sources">
		{#each links as l (l.tbl)}
			<li>
				<span class="name">
					<strong>{l.tableName || l.name}</strong><span class="pipe" title={t('remote.tableSources.pipeline.tooltip')}>{t('remote.tableSources.pipeline.label')}</span>
					<span class="muted">donnees.{l.sqlName} · {hostOf(l.server)}{l.onSave ? t('remote.tableSources.onSave') : ''}</span>
					<span class="muted" class:failed={!!l.lastError}>{sourceStatus(l)}</span>
				</span>
				<button disabled={!!busy || !l.tableName} onclick={() => push(l)}>{busy === l.tableName ? t('remote.tableSources.sending') : t('remote.tableSources.update')}</button>
			</li>
		{/each}
	</ul>
	<p class="muted">{t('remote.tableSources.help')}</p>
	{#if said}<p class="said" role="status">{said}</p>{/if}
{/if}

<style>
	h3 {
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #6b7280;
		margin: 14px 0 6px;
	}
	.sources {
		list-style: none;
		padding: 0;
		margin: 0;
	}
	.sources li {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 5px 0;
		border-bottom: 1px solid #f0f1f3;
	}
	.name {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-width: 0;
	}
	.pipe {
		align-self: flex-start;
		background: #ede9fe;
		color: #5b21b6;
		border-radius: 3px;
		font-size: 11px;
		padding: 0 5px;
		margin-top: 2px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.failed {
		color: #b91c1c;
	}
	.said {
		color: #217346;
	}
	button {
		font: inherit;
		padding: 4px 12px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	button:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
