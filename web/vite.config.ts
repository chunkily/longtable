import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	server: {
		// In production the Go binary serves the frontend and API from the
		// same origin. In dev, `npm run dev` runs on its own port, so proxy
		// API/WS calls through to a `go run ./cmd/longtable` instance.
		proxy: {
			'/api': 'http://localhost:8080',
			'/ws': { target: 'ws://localhost:8080', ws: true }
		}
	}
});
