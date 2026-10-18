<script lang="ts">
	// Assistant externe : ce que Claude Desktop ou Claude Code fait dans le
	// classeur par le serveur MCP. État de la connexion, fil des actions par
	// série (appels d'un même client sans pause de plus de deux minutes),
	// « Annuler ces modifications » par série tant qu'aucune autre action ne
	// l'a suivie, et l'option « Demander avant de modifier »
	// (mcp.confirmWrites). La conversation reste dans Claude : Lattis ne voit
	// que les actions. Le fil vit dans external.svelte.ts et survit au
	// démontage du composant.
	import { onMount } from 'svelte';
	import Icon from '#lib/icons/Icon.svelte';
	import { boolSetting, findSetting, settingNote } from '#lib/shell/settings.js';
	import { setSetting, userSettings } from '#lib/shell/settings.svelte.js';
	import { connectionLabel, seriesSummary, seriesTime, undoTitle, writeCount } from './external';
	import { external, listenExternal, refreshClients, refreshUndo, undoSeries } from './external.svelte.js';

	interface Props {
		/** Va à la feuille ou au tableau visé par une action (facultatif : sans lui, la cible n'est pas cliquable). */
		ongoto?: (target: string) => void;
	}
	let { ongoto }: Props = $props();

	let error = $state('');
	let undoing = $state(0);

	onMount(() => {
		listenExternal();
		refreshClients();
		refreshUndo();
	});

	const confirmSetting = $derived(findSetting(userSettings.list, 'mcp.confirmWrites'));
	const confirmWrites = $derived(boolSetting(userSettings.list, 'mcp.confirmWrites', external.clients?.confirmWrites ?? false));
	const confirmNote = $derived(settingNote(confirmSetting));

	async function toggleConfirm(on: boolean) {
		error = '';
		try {
			await setSetting('mcp.confirmWrites', on);
			if (external.clients) external.clients.confirmWrites = on;
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	async function undo(id: number) {
		undoing = id;
		error = '';
		try {
			await undoSeries(id);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		} finally {
			undoing = 0;
		}
	}
</script>

<div class="external">
	<div class="status" class:on={!!external.connected}>
		<span class="dot"></span>
		<strong>{connectionLabel(external.connected)}</strong>
	</div>
	{#if !external.connected && !external.series.length}
		<p class="muted">Ouvrez Claude Desktop ou Claude Code et demandez par exemple «&nbsp;Que contient mon classeur Lattis&nbsp;?&nbsp;» : chaque action apparaîtra ici.</p>
	{/if}

	<label class="switch" title={confirmNote}>
		<input type="checkbox" checked={confirmWrites} disabled={!!confirmSetting?.locked} onchange={(e) => toggleConfirm(e.currentTarget.checked)} />
		<span>Demander avant de modifier</span>
	</label>
	<p class="hint">{confirmNote || 'Lattis demande votre accord avant chaque modification du classeur ; les lectures passent sans demande.'}</p>

	{#if error || external.error}<p class="error">{error || external.error}</p>{/if}

	<div class="feed">
		{#each external.series as s (s.id)}
			{@const st = external.undo[s.id]}
			{@const canUndo = writeCount(s) > 0 && !!st?.undo}
			<section class="series">
				<header>
					<span class="who">{s.client}</span>
					<span class="muted">{seriesTime(s)}{seriesSummary(s) ? ` · ${seriesSummary(s)}` : ''}</span>
					<span class="spacer"></span>
					{#if writeCount(s) > 0}
						<!-- Infobulle sur l'enveloppe : un bouton désactivé n'en montre pas partout. -->
						<span title={undoTitle(s, st)}>
							<button disabled={!canUndo || undoing !== 0} onclick={() => undo(s.id)}>
								<Icon name="undo" size={14} />{undoing === s.id ? 'Annulation…' : 'Annuler ces modifications'}
							</button>
						</span>
					{/if}
				</header>
				<ul>
					{#each s.items as a (a.id)}
						<li class:error={a.error} class:write={a.write} title={a.tool}>
							<span class="pill"></span>
							<span class="label">{a.label}</span>
							{#if a.target && ongoto}
								<button class="link" title="Aller à « {a.target} »" onclick={() => ongoto?.(a.target)}>Aller à</button>
							{/if}
							{#if a.error && a.detail}<span class="detail">{a.detail}</span>{/if}
						</li>
					{/each}
				</ul>
			</section>
		{/each}
	</div>
</div>

<style>
	.external {
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-height: 0;
		--icon-accent: #217346;
	}
	p {
		margin: 0;
	}
	.status {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.dot {
		width: 9px;
		height: 9px;
		border-radius: 50%;
		background: #9ca3af;
		flex: none;
	}
	.status.on .dot {
		background: #217346;
	}
	.muted,
	.hint {
		color: #6b7280;
	}
	.hint {
		font-size: 12px;
		margin-top: -4px;
	}
	.switch {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.error {
		color: #b91c1c;
	}
	.feed {
		display: flex;
		flex-direction: column;
		gap: 10px;
		overflow: auto;
		min-height: 0;
	}
	.series {
		border-top: 1px solid #e3e5e8;
		padding-top: 6px;
	}
	.series header {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
		font-size: 12px;
	}
	.who {
		font-weight: 600;
	}
	.spacer {
		flex: 1;
	}
	ul {
		list-style: none;
		margin: 4px 0 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	li {
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: 6px;
		font-size: 12px;
		color: #4b5563;
	}
	li.write {
		color: #1f2937;
	}
	.pill {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: #217346;
		flex: none;
		align-self: center;
	}
	li.error {
		color: #b91c1c;
	}
	li.error .pill {
		background: #b91c1c;
	}
	.label {
		overflow-wrap: anywhere;
	}
	.detail {
		flex-basis: 100%;
		padding-left: 14px;
		color: #6b7280;
		overflow-wrap: anywhere;
	}
	button {
		font: inherit;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.55;
		cursor: default;
	}
	button.link {
		border: none;
		background: none;
		padding: 0;
		color: #185c37;
		text-decoration: underline;
	}
</style>
