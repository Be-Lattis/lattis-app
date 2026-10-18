<script lang="ts">
	// Boîte « nom invalide » (refuseName, nameAlert.ts) : bloquante, un seul
	// bouton OK ; Entrée, Échap ou OK la ferment et la saisie reprend dans
	// le champ du nom, le texte tapé gardé et sélectionné.
	import { modal } from './focus.js';

	interface Props {
		title: string;
		message: string;
		onclose: () => void;
	}
	let { title, message, onclose }: Props = $props();

	function onkey(e: KeyboardEvent) {
		// Rien ne passe à la fenêtre derrière (grille, raccourcis).
		e.stopPropagation();
		if (e.key === 'Enter' || e.key === 'Escape') {
			e.preventDefault();
			// Entrée maintenue depuis le champ : la boîte reste lisible.
			if (!e.repeat) onclose();
		}
	}
</script>

<div class="name-alert-back" role="presentation" onmousedown={(e) => e.preventDefault()}>
	<div class="name-alert" role="alertdialog" aria-modal="true" aria-labelledby="name-alert-title" aria-describedby="name-alert-msg" tabindex="-1" use:modal onkeydown={onkey}>
		<header id="name-alert-title">
			<svg width="20" height="20" viewBox="0 0 20 20" aria-hidden="true"><path d="M10 1.8 19 18H1z" fill="#d97706" /><path d="M9.1 7h1.8l-.3 6H9.4zM10 14.4a1.1 1.1 0 1 1 0 2.2 1.1 1.1 0 0 1 0-2.2z" fill="#fff" /></svg>
			{title}
		</header>
		<p id="name-alert-msg">{message}</p>
		<div class="actions">
			<button class="primary" data-autofocus onclick={onclose}>OK</button>
		</div>
	</div>
</div>

<style>
	.name-alert-back {
		position: fixed;
		inset: 0;
		z-index: 1000;
		background: rgba(0, 0, 0, 0.25);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 14vh;
		font: 13px 'Segoe UI', system-ui, sans-serif;
		color: #1f2328;
	}
	.name-alert {
		background: #fff;
		border: 1px solid #d4d7db;
		border-radius: var(--float-radius, 6px);
		box-shadow: 0 8px 28px rgba(0, 0, 0, 0.25);
		width: min(440px, calc(100vw - 32px));
		padding: 0 0 14px;
		outline: none;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		font-weight: 600;
		border-bottom: 1px solid #e3e5e8;
	}
	p {
		margin: 12px 16px 4px;
		white-space: pre-wrap;
		line-height: 1.45;
		user-select: text;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		padding: 12px 16px 0;
	}
	button {
		font: inherit;
		min-width: 80px;
		padding: 5px 16px;
		border: 1px solid #217346;
		background: #217346;
		color: #fff;
		cursor: pointer;
	}
	button:focus-visible {
		outline: 2px solid #1b5e3a;
		outline-offset: 2px;
	}
</style>
