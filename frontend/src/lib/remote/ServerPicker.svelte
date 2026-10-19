<script lang="ts">
	// Choix d'un Serveur Lattis (Publier, Ouvrir depuis un serveur, Data
	// Lake, source Power Query…) : la liste et, à côté, « Gérer les
	// serveurs… » (ajouter, se connecter, retirer). Au retour de la boîte, la
	// liste est relue : le serveur ajouté est choisi s'il est proposé, sinon
	// le choix est gardé, ou passe au premier s'il a été retiré.
	import Icon from '#lib/icons/Icon.svelte';
	import type { Snippet } from 'svelte';
	import type { ServerState } from '#bindings/internal/remote/auth/models.js';
	import { pickServer, serverOption } from './lattis';
	import { loadLattis } from './lattis.svelte.js';
	import ServerManageDialog from './ServerManageDialog.svelte';

	interface Props {
		/** Adresse du serveur choisi ('' : aucun). */
		value: string;
		/** Serveurs proposés (lus dans lattis.state, relus au retour de la boîte). */
		servers: readonly ServerState[];
		disabled?: boolean;
		id?: string;
		/** Texte quand aucun serveur n'est proposé. */
		empty?: string;
		/** Contenu à la place du texte quand aucun serveur n'est proposé (bouton « Se connecter… »). */
		none?: Snippet;
		/** Choix changé (par la liste ou au retour de « Gérer les serveurs »). */
		onchange?: (url: string) => void;
	}
	let { value = $bindable(), servers, disabled = false, id, empty = 'Aucun serveur connecté', none, onchange }: Props = $props();

	let managing = $state(false);

	function set(url: string) {
		if (url === value) return;
		value = url;
		onchange?.(url);
	}

	async function closed(added: string) {
		managing = false;
		await loadLattis();
		set(
			pickServer(
				servers.map((s) => s.url),
				value,
				added
			)
		);
	}
</script>

<span class="picker">
	{#if servers.length}
		<select {id} {value} {disabled} aria-label={id ? undefined : 'Serveur'} onchange={(e) => set((e.currentTarget as HTMLSelectElement).value)}>
			{#if !servers.some((s) => s.url === value)}<option value={value} disabled hidden></option>{/if}
			{#each servers as s (s.url)}<option value={s.url}>{serverOption(s)}</option>{/each}
		</select>
	{:else}
		{#if none}{@render none()}{:else}<span class="none">{empty}</span>{/if}
	{/if}
	<button type="button" class="manage" {disabled} title="Ajouter, se connecter à un serveur ou en retirer un de la liste" onclick={() => (managing = true)}
		><Icon name="database" size={13} />Gérer les serveurs…</button
	>
</span>
{#if managing}<ServerManageDialog onclose={closed} />{/if}

<style>
	.picker {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	select {
		flex: 1;
		min-width: 0;
		font: inherit;
		height: 26px;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 2px 6px;
		background: #fff;
		color: inherit;
		cursor: pointer;
	}
	select:hover:not(:disabled) {
		border-color: #217346;
	}
	select:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	select:disabled {
		background: #f6f7f8;
		cursor: default;
	}
	.none {
		flex: 1;
		color: #6b7280;
		font-size: 12px;
	}
	.manage {
		flex-shrink: 0;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font: inherit;
		height: 26px;
		padding: 0 10px;
		border: 1px solid #c4c8cc;
		border-radius: 3px;
		background: #fff;
		color: inherit;
		cursor: pointer;
		white-space: nowrap;
		--icon-accent: #217346;
	}
	.manage:hover:not(:disabled) {
		border-color: #217346;
		background: #eef6f0;
	}
	.manage:disabled {
		opacity: 0.55;
		cursor: default;
	}
</style>
