<script lang="ts">
	// Fichier › Paramètres : les réglages de l'utilisateur, par section
	// (liste déclarative settingSections). Un réglage changé est écrit tout de
	// suite dans le config.json de l'utilisateur et s'applique sans
	// redémarrer ; un réglage imposé par une stratégie est grisé.
	import { modal } from '#lib/ui/focus.js';
	import { backdrop } from '#lib/ui/backdrop.js';
	import { onMount } from 'svelte';
	import { findSetting, parseNumberSetting, settingNote, settingSections, visibleSections } from './settings';
	import { loadSettings, mcpAvailable, resetSettings, setSetting, userSettings } from './settings.svelte.js';
	import { errorText, t } from '#lib/i18n/index.js';

	interface Props {
		onclose: () => void;
		/** Ouvre la fenêtre du serveur MCP (assistants compatibles MCP). */
		onmcp?: () => void;
	}
	let { onclose, onmcp }: Props = $props();

	let sectionId = $state(settingSections[0].id);
	// Une fonctionnalité décochée cache ses réglages.
	const sections = $derived(visibleSections(userSettings.list));
	const section = $derived(sections.find((s) => s.id === sectionId) ?? sections[0] ?? settingSections[0]);
	let error = $state('');
	let busy = $state('');

	onMount(() => {
		loadSettings();
	});

	async function change(path: string, value: unknown) {
		busy = path;
		error = '';
		try {
			await setSetting(path, value);
		} catch (e) {
			error = errorText(e);
			await loadSettings();
		} finally {
			busy = '';
		}
	}

	// « Réinitialiser » : demande d'abord confirmation, dans le pied de la fenêtre.
	let confirmReset = $state(false);
	async function reset() {
		busy = '*';
		error = '';
		try {
			await resetSettings();
			confirmReset = false;
		} catch (e) {
			error = errorText(e);
			await loadSettings();
		} finally {
			busy = '';
		}
	}

	// Fermeture (Échap compris) : le champ texte en cours de saisie s'écrit d'abord (évènement change).
	function close() {
		const a = document.activeElement;
		if (a instanceof HTMLInputElement && (a.type === 'text' || a.type === 'number')) a.blur();
		onclose();
	}
</script>

<div class="back" role="presentation" use:backdrop={() => close()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label={t('shell.settings.dialog.title')}
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') {
				if (confirmReset) confirmReset = false;
				else close();
			}
		}}
	>
		<header>{t('shell.settings.dialog.title')}</header>
		<div class="main">
			<nav aria-label={t('shell.settings.dialog.sections')}>
				{#each sections as s (s.id)}
					<button class:on={s.id === section.id} aria-current={s.id === section.id ? 'page' : undefined} onclick={() => (sectionId = s.id)}>{s.title}</button>
				{/each}
			</nav>
			<section>
				<h2>{section.title}</h2>
				{#each section.settings as d (d.path)}
					{@const st = findSetting(userSettings.list, d.path)}
					{@const note = settingNote(st)}
					<div class="setting">
						{#if d.kind === 'bool'}
							<label class:off={st?.locked}>
								<input
									type="checkbox"
									checked={st?.value === true}
									disabled={!st || st.locked || busy === d.path}
									onchange={(e) => change(d.path, e.currentTarget.checked)}
								/>
								<span>{d.label}</span>
							</label>
						{:else if d.kind === 'choice'}
							<p class="label" class:off={st?.locked}>{d.label}</p>
							{#each d.choices ?? [] as c (c.value)}
								<label class="choice" class:off={st?.locked}>
									<input
										type="radio"
										name={d.path}
										value={c.value}
										checked={st?.value === c.value}
										disabled={!st || st.locked || busy === d.path}
										onchange={() => change(d.path, c.value)}
									/>
									{#if c.swatch}<span class="swatch" aria-hidden="true">{#each c.swatch as col, i (i)}<span style:background={col}></span>{/each}</span>{/if}
									<span>{c.label}</span>
								</label>
							{/each}
						{:else if d.kind === 'text'}
							<label class="text" class:off={st?.locked}>
								<span>{d.label}</span>
								<input
									type="text"
									spellcheck="false"
									value={typeof st?.value === 'string' ? st.value : ''}
									placeholder={d.placeholder}
									disabled={!st || st.locked || busy === d.path}
									onchange={(e) => change(d.path, e.currentTarget.value.trim())}
								/>
							</label>
						{:else if d.kind === 'number'}
							<label class="text number" class:off={st?.locked}>
								<span>{d.label}</span>
								<input
									type="number"
									min={d.min}
									max={d.max}
									step="1"
									value={typeof st?.value === 'number' ? st.value : ''}
									disabled={!st || st.locked || busy === d.path}
									onchange={(e) => {
										const v = parseNumberSetting(d, e.currentTarget.value);
										if (typeof v === 'number') change(d.path, v);
										else {
											error = v;
											loadSettings();
										}
									}}
								/>
								{#if d.unit}<span class="unit">{d.unit}</span>{/if}
							</label>
						{/if}
						{#if d.help}<p class="muted">{d.help}</p>{/if}
						{#if note}<p class="note">{note}</p>{/if}
					</div>
				{/each}
				{#if error}<p class="error">{error}</p>{/if}
			</section>
		</div>
		{#if confirmReset}
			<footer class="confirm">
				<div class="ask" role="status">
					<strong>{t('shell.settings.dialog.resetConfirm')}</strong>
					<span class="muted">{t('shell.settings.dialog.resetDetail')}</span>
				</div>
				<button class="primary" disabled={busy !== ''} onclick={reset}>{t('shell.settings.dialog.resetOk')}</button>
				<button disabled={busy !== ''} onclick={() => (confirmReset = false)}>{t('common.cancel')}</button>
			</footer>
		{:else}
			<footer>
				<button class="link" onclick={() => (confirmReset = true)}>{t('shell.settings.dialog.reset')}</button>
				{#if onmcp && mcpAvailable()}<button class="mcp" title={t('shell.settings.dialog.mcpTooltip')} onclick={onmcp}>{t('shell.settings.dialog.mcp')}</button>{/if}
				<button class="primary" onclick={close}>{t('common.close')}</button>
			</footer>
		{/if}
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: var(--z-dialog);
		background: var(--backdrop);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 8vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 620px;
		max-width: calc(100vw - 32px);
		max-height: 84vh;
		display: flex;
		flex-direction: column;
		outline: none;
	}
	header {
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	.main {
		display: flex;
		min-height: 220px;
		overflow: auto;
	}
	nav {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 8px;
		width: 160px;
		flex: none;
		border-right: 1px solid #e3e5e8;
		background: #f8f9fa;
	}
	nav button {
		text-align: left;
		border: none;
		background: none;
		padding: 5px 10px;
		border-radius: 3px;
	}
	nav button:hover {
		background: #eceef0;
	}
	nav button.on {
		background: #e3efe8;
		color: #185c37;
		font-weight: 600;
	}
	section {
		flex: 1;
		min-width: 0;
		padding: 10px 16px;
	}
	h2 {
		font-size: 14px;
		font-weight: 600;
		margin: 0 0 10px;
	}
	.setting {
		margin-bottom: 12px;
	}
	label {
		display: flex;
		align-items: center;
		gap: 8px;
		font-weight: 600;
	}
	label.off,
	.label.off {
		color: #6b7280;
	}
	.label {
		margin: 0 0 4px;
		font-weight: 600;
	}
	label.choice {
		font-weight: normal;
		margin: 2px 0 2px 8px;
	}
	/* Aperçu d'un thème : ses deux couleurs, comme dans Affichage › Thème. */
	.swatch {
		display: inline-flex;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		overflow: hidden;
	}
	.swatch span {
		width: 12px;
		height: 14px;
	}
	/* Réglage texte (adresse…) : libellé au-dessus, champ sur toute la largeur. */
	label.text {
		flex-direction: column;
		align-items: stretch;
		gap: 4px;
	}
	label.text input {
		font: inherit;
		font-weight: normal;
		padding: 3px 6px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
	}
	label.text input:focus {
		outline: none;
		border-color: #217346;
	}
	/* Nombre : libellé, champ court et unité sur une ligne. */
	label.text.number {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}
	label.text.number span:first-child {
		flex: 1;
	}
	label.text.number input {
		width: 90px;
		text-align: right;
	}
	p {
		margin: 4px 0 0 24px;
	}
	.muted {
		color: #6b7280;
		font-size: 12px;
	}
	.note {
		color: #92400e;
		font-size: 12px;
	}
	.error {
		color: #c0392b;
		margin-left: 0;
	}
	footer {
		display: flex;
		justify-content: flex-end;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
	}
	/* Réinitialiser : discret, à gauche du pied. */
	button.link {
		margin-right: auto;
		border: none;
		background: none;
		padding: 3px 0;
		color: #6b7280;
		font-size: 12px;
	}
	button.link:hover {
		color: #c0392b;
		text-decoration: underline;
	}
	.ask {
		display: flex;
		flex-direction: column;
		margin-right: auto;
		min-width: 0;
	}
	button {
		font: inherit;
		padding: 3px 12px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
</style>
