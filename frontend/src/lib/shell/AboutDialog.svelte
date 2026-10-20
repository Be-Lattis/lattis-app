<script lang="ts">
	// Fichier › À propos de Lattis : version, licence (MPL-2.0), code source,
	// marque ; en édition web, le code source du serveur (AGPL-3.0).
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { onMount } from 'svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { openOutside } from '#lib/ai/links.js';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		onclose: () => void;
	}
	let { onclose }: Props = $props();

	let about = $state<{ version: string; license: string; source: string; server?: string; serverSource?: string } | null>(null);
	onMount(async () => {
		try {
			about = await DocumentService.About();
		} catch {
			about = { version: '?', license: 'MPL-2.0', source: 'https://github.com/Be-Lattis/lattis-app' };
		}
	});
	const link = (e: MouseEvent, url: string) => {
		e.preventDefault();
		openOutside(url);
	};
</script>

<div class="back" role="presentation" use:backdrop={() => onclose()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.about.title')} tabindex="-1" use:modal={onclose}>
		<h2>Lattis</h2>
		<p class="muted">{t('shell.about.tagline', { version: about?.version ?? '…' })}</p>
		<p>
			{t('shell.about.licensePrefix')} <a href="https://www.mozilla.org/MPL/2.0/" onclick={(e) => link(e, 'https://www.mozilla.org/MPL/2.0/')}>Mozilla Public License 2.0</a>.
			{#if about}<a href={about.source} onclick={(e) => link(e, about!.source)}>{t('shell.about.source')}</a>{/if}
		</p>
		{#if about?.serverSource}
			<p>
				{t('shell.about.server', { server: about.server ?? '' })}
				<a href={about.serverSource} onclick={(e) => link(e, about!.serverSource!)}>{t('shell.about.serverSource')}</a>.
			</p>
		{/if}
		<p class="muted">{t('shell.about.trademarks')}</p>
		<div class="actions"><button class="primary" onclick={onclose}>{t('common.close')}</button></div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: var(--backdrop);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: calc(var(--z-dialog) + 10);
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(460px, 92vw);
		padding: 18px 20px 14px;
	}
	h2 {
		margin: 0 0 4px;
		font-size: 20px;
		color: #14532d;
	}
	p {
		margin: 8px 0;
		line-height: 1.45;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	a {
		color: #217346;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		margin-top: 12px;
	}
	.primary {
		font: inherit;
		background: #217346;
		color: #fff;
		border: 1px solid #217346;
		border-radius: 3px;
		padding: 4px 15px;
		cursor: pointer;
	}
	.primary:hover {
		background: #1a5c38;
		border-color: #1a5c38;
	}
</style>
