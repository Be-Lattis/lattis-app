<script lang="ts">
	// Connexion PostgreSQL : enregistrée dans le classeur sans son mot de
	// passe, qui reste dans le trousseau du système (« Mémoriser ») ou en
	// mémoire le temps de la session.
	import Select from '#lib/ui/Select.svelte';
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import type { Connection } from '#bindings/internal/query/models.js';
	import { clone } from './steps';

	interface Props {
		connection?: Connection | null;
		/** Enregistrée : renvoie son nom. */
		onsaved: (name: string) => void;
		oncancel: () => void;
	}
	let { connection = null, onsaved, oncancel }: Props = $props();

	const initial = (): Connection =>
		connection ? clone(connection) : { id: 0, name: '', kind: 'postgres', postgres: { host: 'localhost', port: 5432, database: '', user: 'postgres', sslmode: '' }, remembered: false };
	let c = $state<Connection>(initial());
	let password = $state('');
	let remember = $state(true);
	let error = $state('');
	let tested = $state('');
	let busy = $state(false);
	const err = (e: unknown) => String(e instanceof Error ? e.message : e);

	$effect(() => {
		if (!c.id && !c.name && c.postgres.database) c.name = c.postgres.database;
	});

	async function test() {
		error = tested = '';
		busy = true;
		try {
			const msg = await DocumentService.TestConnection(c, password, remember);
			if (msg) error = msg;
			else tested = 'Connexion réussie.';
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}

	async function save() {
		error = '';
		if (!c.name.trim()) return void (error = 'Donnez un nom à la connexion.');
		busy = true;
		try {
			await DocumentService.SaveConnection(c, password, remember);
			onsaved(c.name.trim());
		} catch (e) {
			error = err(e);
		} finally {
			busy = false;
		}
	}
</script>

<div class="back">
	<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
	<div
		class="dialog"
		role="dialog"
		aria-modal="true"
		aria-label="Connexion PostgreSQL"
		tabindex="-1"
		onkeydown={(e) => {
			e.stopPropagation();
			if (e.key === 'Escape') oncancel();
		}}
	>
		<header>Base de données PostgreSQL</header>
		<div class="body">
			<div class="grid">
				<label>Serveur <input bind:value={c.postgres.host} placeholder="localhost" /></label>
				<label>Port <input type="number" bind:value={c.postgres.port} /></label>
				<label>Base de données <input bind:value={c.postgres.database} /></label>
				<label>Utilisateur <input bind:value={c.postgres.user} /></label>
				<label
					>Chiffrement (SSL) <Select
						value={c.postgres.sslmode ?? ''}
						options={[
							{ value: '', label: 'Si le serveur le propose' },
							{ value: 'disable', label: 'Désactivé' },
							{ value: 'require', label: 'Obligatoire' },
							{ value: 'verify-full', label: 'Obligatoire, certificat vérifié' }
						]}
						width="100%"
						onchange={(v) => (c.postgres.sslmode = v || undefined)}
					/></label
				>
				<label>Nom de la connexion <input bind:value={c.name} /></label>
				<label>Mot de passe <input type="password" bind:value={password} autocomplete="off" placeholder={c.remembered ? '(mémorisé)' : ''} /></label>
				<label class="check"><input type="checkbox" bind:checked={remember} /> Mémoriser dans le trousseau du système</label>
			</div>
			<p class="muted">Le mot de passe n'est jamais enregistré dans le classeur. La connexion ne sert qu'à lire (transaction en lecture seule).</p>
		</div>
		{#if error}<p class="error">{error}</p>{/if}
		{#if tested}<p class="ok">{tested}</p>{/if}
		<footer>
			<button onclick={test} disabled={busy || !c.postgres.host || !c.postgres.database}>Tester</button>
			<span class="grow"></span>
			<button class="primary" onclick={save} disabled={busy || !c.postgres.host || !c.postgres.database}>OK</button>
			<button onclick={oncancel}>Annuler</button>
		</footer>
	</div>
</div>

<style>
	.back {
		position: fixed;
		inset: 0;
		z-index: 70;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.dialog {
		background: #fff;
		border: 1px solid #c9cdd2;
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: 520px;
		max-width: calc(100vw - 32px);
		font-size: 12px;
	}
	header {
		padding: 10px 14px 4px;
		font-size: 14px;
		font-weight: 600;
	}
	.body {
		padding: 4px 14px;
	}
	.grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 8px 12px;
	}
	label {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	label.check {
		flex-direction: row;
		align-items: center;
		align-self: end;
	}
	input:not([type='checkbox']) {
		font: inherit;
		border: 1px solid #c9cdd2;
		border-radius: 3px;
		padding: 3px 6px;
	}
	input:focus {
		outline: 1px solid #217346;
		border-color: #217346;
	}
	.muted {
		color: #6b7280;
	}
	.error {
		color: #b42318;
		margin: 0 14px;
	}
	.ok {
		color: #217346;
		margin: 0 14px;
	}
	footer {
		display: flex;
		gap: 6px;
		padding: 10px 14px;
	}
	.grow {
		flex: 1;
	}
	button {
		font: inherit;
		border: 1px solid #c9cdd2;
		background: #fff;
		border-radius: 3px;
		padding: 3px 10px;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
	}
	button.primary {
		background: #217346;
		border-color: #217346;
		color: #fff;
		padding-left: 15px;
		padding-right: 15px;
	}
</style>
