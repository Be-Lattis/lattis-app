<script lang="ts">
	// Fichier › Paramètres : les réglages de l'utilisateur, par section
	// (liste déclarative settingSections). Un réglage changé est écrit tout de
	// suite dans le config.json de l'utilisateur et s'applique sans
	// redémarrer ; un réglage imposé par une stratégie est grisé.
	import { modal } from '#lib/ui/focus.js';
	import { onMount } from 'svelte';
	import { findSetting, settingNote, settingSections, visibleSections } from './settings';
	import { loadSettings, setSetting, userSettings } from './settings.svelte.js';

	interface Props {
		onclose: () => void;
	}
	let { onclose }: Props = $props();

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
			error = String(e instanceof Error ? e.message : e);
			await loadSettings();
		} finally {
			busy = '';
		}
	}
</script>

<div class="back" role="presentation" onclick={(e) => e.target === e.currentTarget && onclose()}>
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Paramètres"
		tabindex="-1"
		use:modal
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') onclose();
		}}
	>
		<header>Paramètres</header>
		<div class="main">
			<nav aria-label="Sections">
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
									<span>{c.label}</span>
								</label>
							{/each}
						{/if}
						{#if d.help}<p class="muted">{d.help}</p>{/if}
						{#if note}<p class="note">{note}</p>{/if}
					</div>
				{/each}
				{#if error}<p class="error">{error}</p>{/if}
			</section>
		</div>
		<footer><button class="primary" onclick={onclose}>Fermer</button></footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 50;
		background: rgba(0, 0, 0, 0.25);
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
		padding: 10px 16px;
		border-top: 1px solid #e3e5e8;
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
