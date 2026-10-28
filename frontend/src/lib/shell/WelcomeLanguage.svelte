<script lang="ts">
	// Accueil du premier lancement : choix de la langue de l'interface
	// (réglage ui.language), appliqué tout de suite. Boutons radio : Tab
	// entre dans le groupe, les flèches changent de langue. Masqué quand
	// l'organisation impose la langue.
	import { errorText, t } from '#lib/i18n/index.js';
	import { findSetting, settingImposed } from './settings';
	import { setSetting, userSettings } from './settings.svelte.js';
	import { languageChoices, languageValue } from './welcome';

	const setting = $derived(findSetting(userSettings.list, 'ui.language'));
	const current = $derived(languageValue(setting?.value));
	let error = $state('');

	async function choose(v: string) {
		error = '';
		try {
			await setSetting('ui.language', v);
		} catch (e) {
			error = errorText(e);
		}
	}
</script>

{#if setting && !settingImposed(setting)}
	<fieldset class="lang">
		<legend>{t('shell.welcome.first.language')}</legend>
		<div class="choices">
			{#each languageChoices() as c (c.value)}
				<label class:checked={current === c.value}>
					<input type="radio" name="welcome-language" value={c.value} checked={current === c.value} onchange={() => choose(c.value)} />
					<span>{c.label}</span>
				</label>
			{/each}
		</div>
		<p class="hint">{t('shell.welcome.first.languageHint')}</p>
		{#if error}<p class="error" role="alert">{error}</p>{/if}
	</fieldset>
{/if}

<style>
	.lang {
		border: none;
		margin: 0;
		padding: 0;
		min-width: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 6px;
	}
	legend {
		padding: 0;
		margin-bottom: 6px;
		color: #6b7280;
		font-size: 12px;
		text-align: center;
	}
	.choices {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 6px;
	}
	label {
		display: inline-flex;
		align-items: center;
		padding: 3px 12px;
		border: 1px solid #c9cdd2;
		border-radius: 999px;
		background: #fff;
		cursor: pointer;
		user-select: none;
	}
	label:hover {
		border-color: #217346;
	}
	label.checked {
		background: #217346;
		border-color: #217346;
		color: #fff;
	}
	/* Bouton radio natif hors de la vue, toujours atteint au clavier. */
	input {
		position: absolute;
		opacity: 0;
		width: 1px;
		height: 1px;
		margin: 0;
	}
	label:has(input:focus-visible) {
		outline: 2px solid #217346;
		outline-offset: 2px;
	}
	.hint {
		margin: 0;
		color: #6b7280;
		font-size: 12px;
		text-align: center;
	}
	.error {
		margin: 0;
		color: #c0392b;
		font-size: 12px;
	}
</style>
