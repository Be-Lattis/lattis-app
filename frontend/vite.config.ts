import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-static';
import wails from '@wailsio/runtime/plugins/vite';
import { defineConfig } from 'vitest/config';

// SvelteKit en SPA : pas de serveur dans Wails, la page est un shell statique
// (fallback index.html) servi par l'asset server Go depuis frontend/dist.
export default defineConfig({
	server: {
		host: '127.0.0.1',
		port: Number(process.env.WAILS_VITE_PORT) || 9245,
		strictPort: true
	},
	plugins: [
		sveltekit({
			adapter: adapter({ pages: 'dist', assets: 'dist', fallback: 'index.html' })
		}),
		wails('./bindings')
	],
	test: {
		include: ['src/**/*.test.ts']
	}
});
