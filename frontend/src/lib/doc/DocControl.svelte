<script lang="ts">
	// Contrôle d'un Onglet Document (champ texte, zone de texte, case, liste,
	// fichier) : c'est un contrôle du classeur (scripts : controls.*,
	// évènements), lié à une cellule d'une feuille (« Feuille1!B3 ») ; sa
	// valeur est relue à chaque modification du classeur (live.stamp).
	import { untrack } from 'svelte';
	import type { ButtonConfig } from '#lib/grid/buttons.js';
	import { baseName } from '#lib/grid/buttons.js';

	export interface ControlState {
		config: ButtonConfig;
		value: string;
		error: string;
	}

	interface Props {
		id: number;
		live: { stamp: number };
		load: (id: number) => Promise<ControlState>;
		/** Écrit la valeur dans la cellule liée (et déclenche change). */
		setValue: (id: number, raw: string) => Promise<void>;
		/** Bouton : déclenche son action (click). */
		click: (id: number) => Promise<void>;
		pickFile: (title: string) => Promise<string>;
		locked?: boolean;
		/** Largeur et hauteur imposées (poignée). */
		width?: number;
		height?: number;
	}

	let { id, live, load, setValue, click, pickFile, locked = false, width, height }: Props = $props();

	let st = $state.raw<ControlState | null>(null);
	let missing = $state(false);
	let draft = $state('');

	$effect(() => {
		void live.stamp;
		const i = id;
		untrack(() =>
			load(i).then(
				(r) => {
					st = r;
					missing = false;
					if (document.activeElement !== input) draft = r.value;
				},
				() => (missing = true)
			)
		);
	});

	let input = $state<HTMLInputElement | HTMLTextAreaElement>();
	const kind = $derived(st?.config.kind ?? '');
	const isTrue = (v: string) => /^(vrai|true|1)$/i.test(v.trim());

	async function commit(v: string) {
		if (!st || locked || v === st.value) return;
		await setValue(id, v);
	}

	// Les touches restent dans le contrôle : elles ne vont pas à l'éditeur de texte.
	function keys(ev: KeyboardEvent) {
		ev.stopPropagation();
		if (ev.key === 'Escape') {
			draft = st?.value ?? '';
			(ev.currentTarget as HTMLElement).blur();
		} else if (ev.key === 'Enter' && (kind === 'input' || ev.ctrlKey)) {
			ev.preventDefault();
			commit(draft);
		}
	}

	async function chooseFile() {
		if (locked) return;
		const p = await pickFile(st?.config.label || 'Choisir un fichier');
		if (p) await commit(p);
	}

	const style = $derived(
		[st?.config.color ? `--c-bg:${st.config.color}` : '', st?.config.textColor ? `--c-fg:${st.config.textColor}` : '', width ? `width:${width}px` : '', height ? `height:${height}px` : ''].filter(Boolean).join(';')
	);
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<span class="dctl" {style} title={st ? `${st.config.name ?? ''} · cellule liée : ${st.config.link ?? ''}${st.error ? ' · ' + st.error : ''} · clic droit : modifier` : ''} onmousedown={(e) => e.stopPropagation()}>
	{#if missing}
		<span class="gone">Contrôle supprimé</span>
	{:else if !st}
		<span class="gone">…</span>
	{:else if kind === 'input'}
		<input bind:this={input} type="text" bind:value={draft} placeholder={st.config.label} disabled={locked} onkeydown={keys} onblur={() => commit(draft)} />
	{:else if kind === 'textarea'}
		<textarea bind:this={input} bind:value={draft} placeholder={st.config.label} disabled={locked} onkeydown={keys} onblur={() => commit(draft)}></textarea>
	{:else if kind === 'checkbox'}
		<label class="chk"><input type="checkbox" checked={isTrue(st.value)} disabled={locked} onchange={(e) => commit((e.currentTarget as HTMLInputElement).checked ? 'VRAI' : 'FAUX')} />{st.config.label}</label>
	{:else if kind === 'select'}
		<select value={st.value} disabled={locked} onchange={(e) => commit((e.currentTarget as HTMLSelectElement).value)}>
			{#if !(st.config.options ?? []).includes(st.value)}<option value={st.value}>{st.value}</option>{/if}
			{#each st.config.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}
		</select>
	{:else if kind === 'file'}
		<button class="file" disabled={locked} onclick={chooseFile}>{st.value ? baseName(st.value) : st.config.label || 'Choisir un fichier…'}</button>
	{:else}
		<button class="btn" disabled={locked} onclick={() => click(id)}>{st.config.label || 'Bouton'}</button>
	{/if}
	{#if st?.error}<span class="err" title={st.error}>!</span>{/if}
</span>

<style>
	.dctl {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		vertical-align: middle;
		margin: 0 2px;
		max-width: 100%;
		box-sizing: border-box;
		font: 13px 'Segoe UI', system-ui, sans-serif;
		user-select: none;
	}
	input[type='text'],
	textarea,
	select,
	.file {
		font: inherit;
		padding: 3px 6px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		background: var(--c-bg, #fff);
		color: var(--c-fg, #1f2328);
		box-sizing: border-box;
		width: 100%;
		height: 100%;
		min-width: 120px;
		user-select: text;
	}
	input[type='text'] {
		min-height: 26px;
	}
	textarea {
		min-height: 56px;
		resize: none;
	}
	.chk {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		color: var(--c-fg, inherit);
		cursor: pointer;
	}
	.chk input {
		accent-color: #217346;
	}
	.file {
		cursor: pointer;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.btn {
		font: inherit;
		padding: 3px 12px;
		border: none;
		border-radius: var(--float-radius, 6px);
		background: var(--c-bg, #217346);
		color: var(--c-fg, #fff);
		cursor: pointer;
		width: 100%;
		height: 100%;
	}
	.gone {
		color: #9ca3af;
		font-style: italic;
		font-size: 12px;
	}
	.err {
		color: #b42318;
		font-weight: 700;
	}
</style>
