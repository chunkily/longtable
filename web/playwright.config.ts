import { defineConfig } from '@playwright/test';

// E2E data lives outside both web/ and the repo's default
// longtable.db/longtable-assets paths so a test run never touches a
// developer's local room data. See web/.gitignore.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: false,
	retries: 0,
	// Headroom rather than a fix: these are round trips against a real
	// server, not unit tests, so 30s is tight. Capping workers was tried
	// as a determinism measure and **made things worse** — 3 failing runs
	// in 6 at `workers: 4` against 1 in 6 at the default — so the default
	// stays. See planning/backlog/e2e-hang-after-token-edit.md.
	timeout: 60_000,
	use: {
		// The Go binary, serving the built SPA it embeds — not `npm run
		// dev`. See e2e/run-app.mjs for why; the short version is that
		// vite's dependency optimizer reloads every connected client the
		// first time it meets a new import, which cost one failure per
		// worker on a cold cache and looked like flakiness for months.
		baseURL: 'http://localhost:8080'
	},
	webServer: {
		// See e2e/run-app.mjs for why this isn't an inline shell command:
		// cmd.exe and sh disagree on relative-path syntax for the binary
		// being invoked.
		command: 'node e2e/run-app.mjs',
		url: 'http://localhost:8080/api/healthz',
		reuseExistingServer: false,
		// The frontend build runs inside that script before the Go build, so
		// the default 60s isn't enough on a cold start.
		timeout: 180_000,
		stdout: 'pipe',
		stderr: 'pipe'
	}
});
