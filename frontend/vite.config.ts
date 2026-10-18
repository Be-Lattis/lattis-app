import { sveltekit } from '@sveltejs/kit/vite';
import adapter from '@sveltejs/adapter-static';
import wails from '@wailsio/runtime/plugins/vite';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

// SvelteKit en SPA : pas de serveur dans Wails, la page est un shell statique
// (fallback index.html) servi par l'asset server Go depuis frontend/dist.
//
// Mode « web » (npm run build:web, dev:web) : la même application pour le
// navigateur, servie par le moteur web (cmd/lattis-engine, lattisd) depuis
// frontend/dist-web. « @wailsio/runtime » y est remplacé par
// src/lib/web/runtime.ts : appels JSON-RPC sur le WebSocket /ws.
export default defineConfig(({ mode }) => {
	const web = mode === 'web';
	const out = web ? 'dist-web' : 'dist';
	// Moteur local pour npm run dev:web (cmd/lattis-engine --addr …).
	const engine = process.env.LATTIS_ENGINE || 'http://127.0.0.1:8102';
	return {
		server: {
			host: '127.0.0.1',
			port: Number(process.env.WAILS_VITE_PORT) || 9245,
			strictPort: true,
			proxy: web
				? {
						'/ws': { target: engine, ws: true },
						'/upload': engine,
						'/download': engine
					}
				: undefined
		},
		resolve: web ? { alias: [{ find: /^@wailsio\/runtime$/, replacement: fileURLToPath(new URL('./src/lib/web/runtime.ts', import.meta.url)) }] } : undefined,
		plugins: [
			sveltekit({
				adapter: adapter({ pages: out, assets: out, fallback: 'index.html' })
			}),
			// Évènements typés des liaisons : propres au runtime Wails.
			...(web ? [] : [wails('./bindings')])
		],
		test: {
			include: ['src/**/*.test.ts']
		}
	};
});
