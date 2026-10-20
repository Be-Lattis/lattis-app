<script lang="ts">
	// Audience d'une donnée du Data Lake (boîtes de partage) : héritée du
	// dossier ou non, les personnes et équipes déjà là (pastilles, à retirer
	// d'un clic, et à garder de nouveau), et celles à ajouter, chacune avec
	// son filtre des lignes visibles.
	import type { RightsColumn } from '#bindings/internal/bridge/models.js';
	import Icon from '#lib/icons/Icon.svelte';
	import Select from '#lib/ui/Select.svelte';
	import type { AudienceEntry, DataAudience } from './data';
	import FilterEditor from './FilterEditor.svelte';
	import { t } from '#lib/i18n/index.js';

	interface Props {
		current: DataAudience[];
		/** Entrées de current à retirer (identifiants). */
		removed: Set<string>;
		/** Entrées à ajouter. */
		adds: AudienceEntry[];
		inherit: boolean;
		teams: { id: string; name: string }[];
		/** Colonnes du filtre des lignes (nom et type). */
		columns: RightsColumn[];
		/** Nom du dossier (« Ventes › Europe »), pour « Hériter de l'audience de… ». */
		folder: string;
		/** Tableau partagé (demande à l'assistant). */
		table: string;
		/** Valeurs distinctes d'une colonne (suggestions du filtre). */
		values?: (column: string) => Promise<string[]>;
		disabled?: boolean;
	}
	let { current, removed = $bindable(), adds = $bindable(), inherit = $bindable(), teams, columns, folder, table, values, disabled = false }: Props = $props();

	function toggleRemoved(id: string) {
		const s = new Set(removed);
		if (s.has(id)) s.delete(id);
		else s.add(id);
		removed = s;
	}

	const who = (a: AudienceEntry) => a.email.trim() || teams.find((x) => x.id === a.team)?.name || '';
</script>

<div class="aud">
	<label class="check">
		<input type="checkbox" bind:checked={inherit} {disabled} />
		<span>{t('remote.audience.inherit')} <strong>{folder}</strong></span>
	</label>

	{#if current.length}
		<ul class="chips" aria-label={t('remote.audience.current')}>
			{#each current as a (a.id)}
				{@const gone = removed.has(a.id)}
				<li class="chip" class:gone title={a.predicate ? t('remote.audience.rowsWhere', { predicate: a.predicate }) : t('remote.audience.allRows')}>
					<Icon name="user" size={13} />
					<span class="who">{a.label}</span>
					{#if a.predicate}<span class="filt">{t('remote.audience.filtered')}</span>{/if}
					<button
						type="button"
						class="x"
						{disabled}
						aria-label={gone ? t('remote.audience.keepName', { name: a.label }) : t('remote.audience.removeName', { name: a.label })}
						title={gone ? t('remote.audience.keep') : t('remote.audience.remove')}
						onclick={() => toggleRemoved(a.id)}>{gone ? '↺' : '✕'}</button
					>
				</li>
			{/each}
		</ul>
	{/if}

	{#each adds as a, i (a)}
		<div class="add">
			<div class="row">
				<input class="email" placeholder={t('remote.audience.email.placeholder')} type="email" bind:value={a.email} disabled={disabled || !!a.team} aria-label={t('remote.audience.email.aria')} />
				{#if teams.length}
					<span class="or">{t('remote.audience.or')}</span>
					<Select label={t('remote.audience.team')} value={a.team} options={[{ value: '', label: t('remote.audience.aTeam') }, ...teams.map((x) => ({ value: x.id, label: x.name }))]} disabled={disabled || !!a.email.trim()} onchange={(v) => (a.team = v)} />
				{/if}
				<button type="button" class="x" {disabled} onclick={() => (adds = adds.filter((_, k) => k !== i))} aria-label={t('remote.audience.dontAdd')} title={t('remote.audience.dontAdd')}>✕</button>
			</div>
			<FilterEditor
				label={t('remote.audience.visibleIf')}
				{columns}
				value={a.predicate}
				{disabled}
				teams
				{values}
				assist={t('remote.audience.assist', { table, who: who(a) || t('remote.audience.someone') })}
				onchange={(sql, e) => {
					a.predicate = sql ?? a.predicate;
					a.error = e;
				}}
			/>
		</div>
	{/each}

	<button type="button" class="link" {disabled} onclick={() => (adds = [...adds, { email: '', team: '', predicate: '', error: '' }])}>
		<Icon name="plus" size={13} />{t('remote.audience.add')}
	</button>
</div>

<style>
	.aud {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
		min-width: 0;
	}
	.check {
		display: flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
	}
	.check input {
		margin: 0;
		accent-color: #217346;
	}
	.check strong {
		font-weight: 600;
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		list-style: none;
		padding: 0;
		margin: 0;
	}
	.chip {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		max-width: 280px;
		padding: 1px 2px 1px 8px;
		border: 1px solid #d4d7db;
		border-radius: 12px;
		background: #f6f7f8;
		font-size: 12px;
	}
	.chip.gone {
		background: #fff;
		color: #9ca3af;
		border-style: dashed;
	}
	.chip.gone .who {
		text-decoration: line-through;
	}
	.who {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.filt {
		font-size: 11px;
		color: #5b21b6;
		background: #ede9fe;
		border-radius: 3px;
		padding: 0 4px;
	}
	.x {
		border: none;
		background: none;
		color: #6b7280;
		cursor: pointer;
		font-size: 12px;
		line-height: 1;
		padding: 3px 5px;
		border-radius: 10px;
	}
	.x:hover:not(:disabled) {
		color: #b91c1c;
		background: #fdf0ef;
	}
	.add {
		align-self: stretch;
		border: 1px solid #e3e5e8;
		border-radius: 4px;
		padding: 6px 8px 2px;
	}
	.row {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.email {
		flex: 1;
		min-width: 0;
		font: inherit;
		height: 26px;
		box-sizing: border-box;
		border: 1px solid #c9cdd2;
		border-radius: 4px;
		padding: 2px 6px;
	}
	.email:focus {
		outline: none;
		border-color: #217346;
		box-shadow: 0 0 0 2px rgba(33, 115, 70, 0.18);
	}
	.email:disabled {
		background: #f6f7f8;
	}
	.or {
		color: #6b7280;
		font-size: 12px;
	}
	.link {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: none;
		background: none;
		color: #217346;
		cursor: pointer;
		padding: 2px 0;
		font: inherit;
	}
	.link:hover:not(:disabled) {
		text-decoration: underline;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
