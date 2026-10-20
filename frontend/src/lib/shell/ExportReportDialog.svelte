<script lang="ts">
	// Détails d'un export .xlsx ou .ods : par feuille, ce qui est exporté et
	// ce qui ne l'est pas (ou seulement en valeurs).
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { exportDetails, type ExportReport } from './exportReport.js';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		report: ExportReport;
		format: string; // « Excel » ou « OpenDocument »
		onclose: () => void;
	}
	let { report, format, onclose }: Props = $props();
	const sheets = $derived(exportDetails(report));
</script>

<div class="back" role="presentation" use:backdrop={() => onclose()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.exportReport.dialog.aria')} tabindex="-1" use:modal={onclose}>
		<h2>{t('shell.exportReport.dialog.title', { format })}</h2>
		<div class="list">
			{#each sheets as s, i (i)}
				<section>
					<h3>{s.name}</h3>
					{#if s.kept.length}<p class="kept">{t('shell.exportReport.dialog.kept', { list: s.kept.join(', ') })}</p>{/if}
					{#if s.lost.length}
						<p class="lost-title">{t('shell.exportReport.dialog.lost')}</p>
						<ul>
							{#each s.lost as l, j (j)}<li>{l}</li>{/each}
						</ul>
					{:else}
						<p class="kept">{t('shell.exportReport.dialog.all')}</p>
					{/if}
				</section>
			{/each}
			{#if report.scripts}
				<section>
					<h3>{t('shell.exportReport.dialog.workbook')}</h3>
					<ul>
						<li>{t('shell.exportReport.dialog.scripts', { n: report.scripts })}</li>
					</ul>
				</section>
			{/if}
		</div>
		<p class="muted">{t('shell.exportReport.dialog.note')}</p>
		<div class="actions"><button class="primary" onclick={onclose}>{t('common.close')}</button></div>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 60;
	}
	.dialog {
		background: #fff;
		border-radius: 6px;
		box-shadow: 0 10px 30px rgba(0, 0, 0, 0.2);
		width: min(560px, 92vw);
		max-height: 80vh;
		display: flex;
		flex-direction: column;
		padding: 18px 20px 14px;
	}
	h2 {
		margin: 0 0 8px;
		font-size: 18px;
		color: #14532d;
	}
	.list {
		overflow: auto;
		min-height: 0;
	}
	h3 {
		margin: 10px 0 2px;
		font-size: 13px;
	}
	p,
	ul {
		margin: 2px 0;
		font-size: 12px;
		line-height: 1.5;
	}
	ul {
		padding-left: 20px;
	}
	.kept {
		color: #1a5c38;
	}
	.lost-title {
		color: #8a4b00;
	}
	.muted {
		color: #6b7280;
		margin-top: 10px;
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
