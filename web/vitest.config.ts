import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';

// A dedicated config for unit tests, separate from vite.config.ts's
// full SvelteKit setup — these tests only need the Svelte compiler for
// .svelte.ts rune files, not routing/$app aliasing.
export default defineConfig({
	plugins: [svelte({ compilerOptions: { runes: true } })],
	// Browser builds, since the tests run in jsdom. Without this `svelte`
	// resolves to its server entry, where `mount` throws — which only
	// mattered once something under test mounted a component
	// (token-icon-image.ts, reading an icon's SVG back for the canvas).
	resolve: { conditions: ['browser'] },
	test: {
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
