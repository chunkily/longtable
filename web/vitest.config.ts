import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';

// A dedicated config for unit tests, separate from vite.config.ts's
// full SvelteKit setup — these tests only need the Svelte compiler for
// .svelte.ts rune files, not routing/$app aliasing.
export default defineConfig({
	plugins: [svelte({ compilerOptions: { runes: true } })],
	test: {
		environment: 'jsdom',
		include: ['src/**/*.{test,spec}.{js,ts}']
	}
});
