<script lang="ts" module>
	/** Champ d'une boîte de dialogue ouverte par ui.dialog (internal/api). */
	export interface DialogField {
		name: string;
		label?: string;
		type?: 'text' | 'textarea' | 'password' | 'number' | 'integer' | 'checkbox' | 'select' | 'radio' | 'date' | 'color';
		default?: unknown;
		options?: string[] | null;
		placeholder?: string;
		required?: boolean;
		min?: number | null;
		max?: number | null;
	}

	export interface DialogSpec {
		id: number;
		title: string;
		message?: string;
		fields: DialogField[] | null;
		ok: string;
		cancel?: string;
		/** Code affiché sur demande (accord d'exécution d'un script reçu). */
		code?: string;
	}

	/** Libellé du bouton qui montre ou cache le code. */
	export function codeLabel(code: string, shown: boolean): string {
		if (shown) return 'Masquer le code';
		const n = code.replace(/\r\n/g, '\n').replace(/\n+$/, '').split('\n').length;
		return `Voir le code (${n} ligne${n > 1 ? 's' : ''})`;
	}

	/** Valeurs de départ des champs. */
	export function initialValues(fields: DialogField[]): Record<string, unknown> {
		const v: Record<string, unknown> = {};
		for (const f of fields) {
			const d = f.default;
			switch (f.type) {
				case 'checkbox':
					v[f.name] = d === true || d === 'true' || d === 1;
					break;
				case 'select':
				case 'radio':
					v[f.name] = d !== undefined && d !== null && f.options?.includes(String(d)) ? String(d) : f.type === 'select' ? (f.options?.[0] ?? '') : '';
					break;
				case 'color':
					v[f.name] = typeof d === 'string' && /^#[0-9a-f]{6}$/i.test(d) ? d : '#000000';
					break;
				default:
					v[f.name] = d === undefined || d === null ? '' : String(d);
			}
		}
		return v;
	}

	/** Vérifie les valeurs ; renvoie le message de la première erreur, ou ''. */
	export function validate(fields: DialogField[], v: Record<string, unknown>): string {
		for (const f of fields) {
			const label = f.label || f.name;
			const x = v[f.name];
			const empty = x === '' || x === undefined || x === null;
			if (f.required && empty && f.type !== 'checkbox') return `« ${label} » est obligatoire`;
			if ((f.type === 'number' || f.type === 'integer') && !empty) {
				const n = Number(String(x).replace(',', '.'));
				if (!Number.isFinite(n)) return `« ${label} » : nombre attendu`;
				if (f.type === 'integer' && !Number.isInteger(n)) return `« ${label} » : nombre entier attendu`;
				if (f.min !== undefined && f.min !== null && n < f.min) return `« ${label} » : au moins ${f.min}`;
				if (f.max !== undefined && f.max !== null && n > f.max) return `« ${label} » : au plus ${f.max}`;
			}
		}
		return '';
	}

	/** Valeurs envoyées au script : nombres convertis, champs vides omis. */
	export function answer(fields: DialogField[], v: Record<string, unknown>): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const f of fields) {
			const x = v[f.name];
			if (f.type === 'checkbox') out[f.name] = !!x;
			else if (f.type === 'number' || f.type === 'integer') {
				if (x !== '' && x !== undefined) out[f.name] = Number(String(x).replace(',', '.'));
			} else if (x !== '' && x !== undefined) out[f.name] = String(x);
		}
		return out;
	}
</script>

<script lang="ts">
	// Boîte de dialogue ouverte par un script (ui.dialog, ui.alert,
	// ui.confirm, ui.prompt) ou par un assistant via le serveur MCP. Le
	// script attend la réponse : Entrée valide, Échap annule.
	import { modal } from '#lib/ui/focus.js';
	import { tick } from 'svelte';

	interface Props {
		spec: DialogSpec;
		onanswer: (values: Record<string, unknown> | null) => void;
	}
	let { spec, onanswer }: Props = $props();

	const fields = $derived(spec.fields ?? []);
	let values = $state<Record<string, unknown>>({});
	let error = $state('');
	let showCode = $state(false);
	let box = $state<HTMLDivElement>();

	$effect.pre(() => {
		values = initialValues(spec.fields ?? []);
		error = '';
		showCode = false;
	});
	$effect(() => {
		void spec.id;
		tick().then(() => {
			const first = box?.querySelector<HTMLElement>('input, textarea, select, button.primary');
			first?.focus();
			if (first instanceof HTMLInputElement && first.type === 'text') first.select();
		});
	});

	function ok() {
		error = validate(fields, values);
		if (error) return;
		onanswer(answer(fields, values));
	}

	function onkey(e: KeyboardEvent) {
		e.stopPropagation();
		if (e.key === 'Escape') {
			e.preventDefault();
			onanswer(null);
		} else if (e.key === 'Enter' && !(e.target instanceof HTMLTextAreaElement)) {
			e.preventDefault();
			ok();
		}
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div class="dialog" role="dialog" aria-modal="true" aria-label={spec.title} tabindex="-1" bind:this={box} use:modal onkeydown={onkey}>
		<header>{spec.title}</header>
		{#if spec.message}<p class="message">{spec.message}</p>{/if}
		{#each fields as f (f.name)}
			{@const id = `dlg-${spec.id}-${f.name}`}
			<div class="field" class:inline={f.type === 'checkbox'}>
				{#if f.type === 'checkbox'}
					<input {id} type="checkbox" bind:checked={values[f.name] as boolean} />
					<label for={id}>{f.label || f.name}</label>
				{:else}
					<label for={id}>{f.label || f.name}{f.required ? ' *' : ''}</label>
					{#if f.type === 'textarea'}
						<textarea {id} rows="4" placeholder={f.placeholder} bind:value={values[f.name] as string}></textarea>
					{:else if f.type === 'select'}
						<select {id} bind:value={values[f.name] as string}>
							{#each f.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}
						</select>
					{:else if f.type === 'radio'}
						<div class="radios" role="radiogroup" aria-labelledby={id}>
							{#each f.options ?? [] as o (o)}
								<label class="radio"><input type="radio" name={id} value={o} bind:group={values[f.name] as string} />{o}</label>
							{/each}
						</div>
					{:else if f.type === 'number' || f.type === 'integer'}
						<input {id} type="text" inputmode="decimal" placeholder={f.placeholder} bind:value={values[f.name] as string} />
					{:else if f.type === 'date'}
						<input {id} type="date" bind:value={values[f.name] as string} />
					{:else if f.type === 'color'}
						<input {id} type="color" bind:value={values[f.name] as string} />
					{:else}
						<input {id} type={f.type === 'password' ? 'password' : 'text'} placeholder={f.placeholder} bind:value={values[f.name] as string} />
					{/if}
				{/if}
			</div>
		{/each}
		{#if spec.code && showCode}<pre class="code" aria-label="Code du script">{spec.code}</pre>{/if}
		{#if error}<p class="error">{error}</p>{/if}
		<div class="actions">
			{#if spec.code}
				<button class="see" aria-expanded={showCode} onclick={() => (showCode = !showCode)}>{codeLabel(spec.code, showCode)}</button>
			{/if}
			<button class="primary" onclick={ok}>{spec.ok || 'OK'}</button>
			{#if spec.cancel}<button onclick={() => onanswer(null)}>{spec.cancel}</button>{/if}
		</div>
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
		padding-top: 12vh;
	}
	.dialog {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		min-width: 340px;
		max-width: min(560px, calc(100vw - 32px));
		max-height: 76vh;
		overflow: auto;
		padding: 0 0 14px;
		outline: none;
	}
	header {
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
		margin-bottom: 8px;
	}
	.message {
		margin: 6px 16px 10px;
		white-space: pre-wrap;
		user-select: text;
	}
	.field {
		display: flex;
		flex-direction: column;
		gap: 3px;
		padding: 5px 16px;
	}
	.field.inline {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}
	label {
		color: #374151;
	}
	input[type='text'],
	input[type='password'],
	input[type='date'],
	textarea,
	select {
		font: inherit;
		padding: 4px 6px;
		border: 1px solid #c4c8cc;
		border-radius: 2px;
	}
	input:focus,
	textarea:focus,
	select:focus {
		outline: 2px solid #217346;
		outline-offset: -1px;
	}
	textarea {
		resize: vertical;
	}
	.radios {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 14px;
	}
	.radio {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.error {
		margin: 6px 16px 0;
		color: #c0392b;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding: 12px 16px 0;
	}
	.actions button {
		font: inherit;
		padding: 5px 16px;
		border: 1px solid #c4c8cc;
		background: #fff;
		cursor: pointer;
	}
	.actions button.see {
		margin-right: auto;
	}
	.code {
		margin: 6px 16px 0;
		padding: 6px 8px;
		max-height: 40vh;
		overflow: auto;
		background: #f6f8fa;
		border: 1px solid #e3e5e8;
		font: 12px Consolas, 'Cascadia Mono', monospace;
		white-space: pre;
		user-select: text;
	}
	.actions button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
