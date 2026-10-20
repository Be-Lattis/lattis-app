<script lang="ts">
	// Hôte des Serveurs Lattis dans la page : recharge l'état quand les
	// classeurs partagés sont activés ou coupés (Paramètres), et affiche la
	// page de connexion demandée par requestLogin (menu Compte, connexion
	// d'une requête, boîte de connexion).
	import { sharedWorkbooksEnabled } from '#lib/shell/settings.svelte.js';
	import LoginDialog from './LoginDialog.svelte';
	import { lattis, loadLattis } from './lattis.svelte.js';

	$effect(() => {
		const on = sharedWorkbooksEnabled();
		loadLattis();
		if (!on) lattis.login?.done(false);
	});
</script>

{#if lattis.login && lattis.state.enabled}
	{@const req = lattis.login}
	{#key req}
		<LoginDialog url={req.url} onlocal={req.local ? () => req.done('local') : undefined} ondone={(ok) => req.done(ok)} />
	{/key}
{/if}
