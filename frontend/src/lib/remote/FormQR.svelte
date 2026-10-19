<script lang="ts">
	// Lien d'un formulaire et son QR code, généré sur le poste : copier le
	// lien, enregistrer le QR code en SVG ou en PNG (affiche, un QR code par
	// site avec un lien pré-rempli).
	import { DocumentService } from '#bindings/internal/bridge/index.js';
	import { fileName, qrPNG, qrSVG } from './forms';

	interface Props {
		url: string;
		title: string;
		/** Message de la barre d'état (lien copié, QR code enregistré). */
		onnotice?: (msg: string) => void;
		/** QR code plus petit (liste de liens pré-remplis). */
		small?: boolean;
	}
	let { url, title, onnotice, small = false }: Props = $props();
	let error = $state('');
	const svg = $derived(qrSVG(url));

	async function copy() {
		try {
			await navigator.clipboard.writeText(url);
			onnotice?.('Lien du formulaire copié');
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}

	async function save(ext: 'svg' | 'png') {
		error = '';
		try {
			const data = ext === 'svg' ? qrSVG(url, 8) : await qrPNG(url);
			const path = await DocumentService.FormSaveQR(fileName(`QR ${title}`), ext, data);
			if (path) onnotice?.(`QR code enregistré : ${path}`);
		} catch (e) {
			error = String(e instanceof Error ? e.message : e);
		}
	}
</script>

<div class="qr" class:small>
	<!-- QR code dessiné par qrcode-generator (SVG sans texte saisi). -->
	<div class="qr-img" role="img" aria-label="QR code du lien">{@html svg}</div>
	<div class="qr-side">
		<code class="qr-url">{url}</code>
		<div class="qr-btns">
			<button type="button" class="sf-btn" onclick={copy}>Copier le lien</button>
			<button type="button" class="sf-btn" onclick={() => save('png')} title="Image PNG, pour un document ou une affiche">QR code en PNG…</button>
			<button type="button" class="sf-btn" onclick={() => save('svg')} title="Image vectorielle, nette à toute taille">QR code en SVG…</button>
		</div>
		{#if error}<p class="sf-error" role="alert">{error}</p>{/if}
	</div>
</div>

<style>
	.qr {
		display: flex;
		gap: 14px;
		align-items: flex-start;
	}
	.qr-img {
		flex: none;
		width: 132px;
		height: 132px;
		border: 1px solid #e5e7eb;
		border-radius: 4px;
		background: #fff;
	}
	.qr.small .qr-img {
		width: 84px;
		height: 84px;
	}
	.qr-img :global(svg) {
		display: block;
		width: 100%;
		height: 100%;
	}
	.qr-side {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.qr-url {
		padding: 4px 6px;
		border-radius: 3px;
		background: #f3f4f6;
		font-size: 12px;
		word-break: break-all;
		user-select: text;
	}
	.qr-btns {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
</style>
