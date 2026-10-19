<script lang="ts">
	// Aperçu approché de la page /f/<code> (§3.5) : rendu de la même
	// définition que le serveur, en largeur de téléphone ou d'ordinateur.
	// Rien n'est envoyé ; « Ouvrir l'aperçu dans le navigateur » montre la
	// vraie page.
	import { accentColor, listValues, pagesOf, reducedMarkdown, type FormColumn, type FormDefinition, type FormField } from './forms';

	interface Props {
		def: FormDefinition;
		columns: FormColumn[];
		device: 'phone' | 'desktop';
		serverName: string;
		logo?: string;
		banner?: string;
	}
	let { def, columns, device, serverName, logo = '', banner = '' }: Props = $props();

	const en = $derived(def.lang === 'en');
	const T = $derived(
		en
			? { required: 'required', next: 'Next', prev: 'Previous', pos: 'My location', another: 'Submit another response', yes: 'Yes', no: 'No', choose: 'Choose…', served: 'Form served by' }
			: { required: 'obligatoire', next: 'Suivant', prev: 'Précédent', pos: 'Ma position', another: 'Envoyer une autre réponse', yes: 'Oui', no: 'Non', choose: 'Choisir…', served: 'Formulaire servi par' }
	);
	const pages = $derived(pagesOf(def.fields, def.appearance.paged));
	let page = $state(0);
	let thanks = $state(false);
	const cur = $derived(Math.min(page, pages.length - 1));
	const accent = $derived(accentColor(def.appearance.accent));
	const anyRequired = $derived(def.fields.some((f) => f.required && !f.hidden));

	function label(f: FormField): string {
		return f.label || columns.find((c) => c.columnId === f.column)?.name || (f.kind === 'position' ? 'Position' : '');
	}
	function fixed(f: FormField): string {
		const d = f.default;
		if (!d) return '';
		if (d.fixed !== undefined) return String(d.fixed);
		const now = new Date();
		const p = (n: number) => String(n).padStart(2, '0');
		switch (d.token) {
			case 'today':
				return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
			case 'now':
				return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}T${p(now.getHours())}:${p(now.getMinutes())}`;
			case 'year':
				return String(now.getFullYear());
			case 'respondent':
				return 'vous@exemple.fr';
		}
		return '';
	}
</script>

<div class="fp-frame" class:phone={device === 'phone'} style:--fp-accent={accent}>
	<div class="fp-page" lang={def.lang}>
		{#if banner}<img class="fp-banner" src={banner} alt="" />{/if}
		<header class="fp-head">
			{#if logo}<img class="fp-logo" src={logo} alt="" />{/if}
			<h1>{def.appearance.title || 'Sans titre'}</h1>
		</header>
		{#if thanks}
			<div class="fp-thanks">
				<!-- Markdown réduit, échappé (reducedMarkdown). -->
				{@html reducedMarkdown(def.appearance.thanks || 'Merci !', false)}
				{#if def.appearance.another}<button type="button" class="fp-link" onclick={() => (thanks = false)}>{T.another}</button>{/if}
			</div>
		{:else}
			{#if def.appearance.intro && cur === 0}
				<!-- Markdown réduit, échappé (reducedMarkdown). -->
				<div class="fp-intro">{@html reducedMarkdown(def.appearance.intro)}</div>
			{/if}
			{#if def.appearance.paged && pages.length > 1}
				<div class="fp-progress" role="progressbar" aria-valuemin={1} aria-valuemax={pages.length} aria-valuenow={cur + 1}>
					<span style:width="{((cur + 1) / pages.length) * 100}%"></span>
				</div>
			{/if}
			{#if anyRequired && cur === 0}<p class="fp-note">* {T.required}</p>{/if}
			{#each pages[cur] ?? [] as f, i (i)}
				{#if f.kind === 'section'}
					<h2 class="fp-section">{f.title}</h2>
					<!-- Markdown réduit, échappé (reducedMarkdown). -->
					{#if f.description}<div class="fp-desc">{@html reducedMarkdown(f.description)}</div>{/if}
				{:else}
					{@const id = `fp-${cur}-${i}`}
					{@const values = listValues(f.list, columns)}
					<div class="fp-field">
						{#if f.widget !== 'checkbox'}
							<label for={id}>{label(f)}{#if f.required}<span class="fp-req"> *</span>{/if}</label>
						{/if}
						{#if f.help}<p class="fp-help">{f.help}</p>{/if}
						{#if f.kind === 'position'}
							<button type="button" class="fp-pos" {id}>⌖ {T.pos}</button>
						{:else if f.widget === 'textarea'}
							<textarea {id} rows="3" placeholder={f.placeholder} value={fixed(f)} readonly={f.locked}></textarea>
						{:else if f.widget === 'radio'}
							<div class="fp-radios" role="radiogroup" aria-labelledby={id}>
								{#each values as v (v)}
									<label class="fp-radio"><input type="radio" name={id} checked={fixed(f).toLowerCase() === v.toLowerCase()} />{v}</label>
								{/each}
							</div>
						{:else if f.widget === 'list'}
							<select {id} value={values.find((v) => v.toLowerCase() === fixed(f).toLowerCase()) ?? ''}>
								<option value="">{f.placeholder || T.choose}</option>
								{#each values as v (v)}<option>{v}</option>{/each}
							</select>
						{:else if f.widget === 'checkbox'}
							<label class="fp-check"><input type="checkbox" {id} checked={fixed(f) === 'true'} />{label(f)}{#if f.required}<span class="fp-req"> *</span>{/if}</label>
						{:else if f.widget === 'yesno'}
							<div class="fp-radios">
								<label class="fp-radio"><input type="radio" name={id} />{T.yes}</label>
								<label class="fp-radio"><input type="radio" name={id} />{T.no}</label>
							</div>
						{:else}
							<div class="fp-input">
								<input
									{id}
									type={f.widget === 'number' ? 'number' : f.widget === 'date' ? 'date' : f.widget === 'datetime' ? 'datetime-local' : f.format === 'email' ? 'email' : f.format === 'tel' ? 'tel' : f.format === 'url' ? 'url' : 'text'}
									placeholder={f.placeholder}
									value={fixed(f)}
									min={f.min ?? undefined}
									max={f.max ?? undefined}
									step={f.step ?? undefined}
									readonly={f.locked}
								/>
								{#if f.unit}<span class="fp-unit">{f.unit}</span>{/if}
							</div>
						{/if}
					</div>
				{/if}
			{/each}
			{#if cur === pages.length - 1 && def.appearance.privacy}
				<!-- Markdown réduit, échappé (reducedMarkdown). -->
				<div class="fp-privacy">{@html reducedMarkdown(def.appearance.privacy)}</div>
			{/if}
			<div class="fp-nav">
				{#if cur > 0}<button type="button" class="fp-btn ghost" onclick={() => (page = cur - 1)}>{T.prev}</button>{/if}
				{#if cur < pages.length - 1}
					<button type="button" class="fp-btn" onclick={() => (page = cur + 1)}>{T.next}</button>
				{:else}
					<button type="button" class="fp-btn" onclick={() => (thanks = true)}>{def.appearance.submit_label || 'Envoyer'}</button>
				{/if}
			</div>
		{/if}
		<footer class="fp-foot">{T.served} {serverName || 'le serveur'} · Lattis</footer>
	</div>
</div>

<style>
	.fp-frame {
		margin: 0 auto;
		width: 100%;
		max-width: 720px;
		border: 1px solid #d6d9dd;
		border-radius: 8px;
		background: #fff;
		overflow: hidden;
	}
	.fp-frame.phone {
		max-width: 360px;
		border-radius: 18px;
		border-width: 6px;
		border-color: #1f2328;
	}
	.fp-page {
		padding: 0 0 8px;
		font-size: 14px;
		color: #1f2328;
	}
	.fp-banner {
		display: block;
		width: 100%;
		max-height: 160px;
		object-fit: cover;
	}
	.fp-head {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 14px 16px 4px;
	}
	.fp-logo {
		height: 32px;
		width: auto;
	}
	h1 {
		margin: 0;
		font-size: 19px;
		line-height: 1.25;
	}
	.fp-intro,
	.fp-desc,
	.fp-privacy,
	.fp-thanks,
	.fp-note,
	.fp-field,
	.fp-section,
	.fp-progress,
	.fp-nav {
		margin-left: 16px;
		margin-right: 16px;
	}
	.fp-intro :global(p),
	.fp-desc :global(p),
	.fp-privacy :global(p),
	.fp-thanks :global(p) {
		margin: 6px 0;
	}
	.fp-intro :global(a),
	.fp-privacy :global(a),
	.fp-desc :global(a) {
		color: var(--fp-accent);
	}
	.fp-note {
		margin-top: 8px;
		margin-bottom: 0;
		font-size: 12px;
		color: #6b7280;
	}
	.fp-progress {
		height: 4px;
		margin-top: 8px;
		background: #e5e7eb;
		border-radius: 2px;
	}
	.fp-progress span {
		display: block;
		height: 100%;
		background: var(--fp-accent);
		border-radius: 2px;
	}
	.fp-section {
		margin-top: 18px;
		margin-bottom: 0;
		padding-bottom: 4px;
		font-size: 15px;
		border-bottom: 2px solid var(--fp-accent);
	}
	.fp-field {
		margin-top: 14px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.fp-field > label {
		font-weight: 600;
	}
	.fp-req {
		color: #b91c1c;
	}
	.fp-help {
		margin: 0;
		font-size: 12px;
		color: #6b7280;
	}
	.fp-input {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.fp-input input,
	textarea,
	select {
		flex: 1;
		min-width: 0;
		padding: 8px 10px;
		border: 1px solid #c4c8cc;
		border-radius: 6px;
		font: inherit;
		background: #fff;
	}
	.fp-input input:focus,
	textarea:focus,
	select:focus {
		outline: 2px solid var(--fp-accent);
		outline-offset: 0;
	}
	.fp-input input[readonly],
	textarea[readonly] {
		background: #f3f4f6;
	}
	.fp-unit {
		color: #4b5563;
	}
	.fp-radios {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.fp-radio,
	.fp-check {
		display: flex;
		align-items: center;
		gap: 8px;
		font-weight: 400;
	}
	.fp-radio input,
	.fp-check input {
		accent-color: var(--fp-accent);
	}
	.fp-pos {
		align-self: flex-start;
		padding: 7px 12px;
		border: 1px solid var(--fp-accent);
		border-radius: 6px;
		background: #fff;
		color: var(--fp-accent);
		font: inherit;
		cursor: pointer;
	}
	.fp-privacy {
		margin-top: 16px;
		font-size: 12px;
		color: #4b5563;
	}
	.fp-nav {
		display: flex;
		gap: 8px;
		margin-top: 16px;
	}
	.fp-btn {
		padding: 9px 18px;
		border: none;
		border-radius: 6px;
		background: var(--fp-accent);
		color: #fff;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.fp-btn.ghost {
		background: #fff;
		color: var(--fp-accent);
		border: 1px solid var(--fp-accent);
	}
	.fp-thanks {
		margin-top: 16px;
	}
	.fp-link {
		padding: 0;
		border: none;
		background: none;
		color: var(--fp-accent);
		text-decoration: underline;
		font: inherit;
		cursor: pointer;
	}
	.fp-foot {
		margin: 18px 16px 0;
		padding-top: 8px;
		border-top: 1px solid #e5e7eb;
		font-size: 11px;
		color: #6b7280;
	}
</style>
