<script lang="ts">
	// Fichier › Exporter › Script SQL : dialecte de la base visée et contenu
	// (classeur, feuille active ou tableau actif). Le script reprend les
	// tableaux et les tableaux croisés en valeurs (internal/transfer/sqlexport.go).
	import { backdrop } from '#lib/ui/backdrop.js';
	import { modal } from '#lib/ui/focus.js';
	import Select from '#lib/ui/Select.svelte';
	import { t } from '#lib/i18n/index.js';

	export type SqlDialect = 'standard' | 'postgresql' | 'mysql' | 'sqlserver' | 'sqlite';
	export type SqlScope = 'workbook' | 'sheet' | 'table';

	interface Props {
		/** Tableau actif (portée « tableau »), absent sinon. */
		table?: string;
		onexport: (dialect: SqlDialect, scope: SqlScope) => void;
		onclose: () => void;
	}
	let { table, onexport, onclose }: Props = $props();

	let dialect = $state<SqlDialect>('standard');
	let scope = $state<SqlScope>('workbook');
	const dialects = $derived<{ value: SqlDialect; label: string }[]>([
		{ value: 'standard', label: t('shell.sqlExport.standard') },
		{ value: 'postgresql', label: 'PostgreSQL' },
		{ value: 'mysql', label: 'MySQL' },
		{ value: 'sqlserver', label: 'SQL Server' },
		{ value: 'sqlite', label: 'SQLite' }
	]);
</script>

<div class="back" role="presentation" use:backdrop={() => onclose()}>
	<div class="dialog" role="dialog" aria-modal="true" aria-label={t('shell.sqlExport.title')} tabindex="-1" use:modal={onclose}>
		<h2>{t('shell.sqlExport.title')}</h2>
		<label class="field">
			<span>{t('shell.sqlExport.dialect')}</span>
			<Select bind:value={dialect} options={dialects} label={t('shell.sqlExport.dialect')} width="220px" />
		</label>
		<fieldset>
			<legend>{t('shell.sqlExport.scope')}</legend>
			<label><input type="radio" name="sql-scope" value="workbook" bind:group={scope} />{t('shell.sqlExport.workbook')}</label>
			<label><input type="radio" name="sql-scope" value="sheet" bind:group={scope} />{t('shell.sqlExport.sheet')}</label>
			{#if table}
				<label><input type="radio" name="sql-scope" value="table" bind:group={scope} />{t('shell.sqlExport.table', { name: table })}</label>
			{/if}
		</fieldset>
		<p class="hint">{t('shell.sqlExport.hint')}</p>
		<div class="actions">
			<button onclick={onclose}>{t('common.cancel')}</button>
			<button class="primary" onclick={() => onexport(dialect, scope)}>{t('shell.sqlExport.export')}</button>
		</div>
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
		width: min(380px, 92vw);
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 18px 20px 14px;
		font-size: 13px;
	}
	h2 {
		margin: 0 0 4px;
		font-size: 18px;
		color: #14532d;
	}
	.field {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}
	fieldset {
		border: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	legend {
		padding: 0;
		margin-bottom: 4px;
	}
	fieldset label {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.hint {
		margin: 0;
		color: #6b7280;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 4px;
	}
	.actions button {
		font: inherit;
		border: 1px solid #d4d7db;
		background: #fff;
		border-radius: 3px;
		padding: 4px 15px;
		cursor: pointer;
	}
	.actions .primary {
		background: #217346;
		color: #fff;
		border-color: #217346;
	}
	.actions .primary:hover {
		background: #1a5c38;
		border-color: #1a5c38;
	}
</style>
