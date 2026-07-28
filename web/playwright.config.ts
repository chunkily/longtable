import { defineConfig } from '@playwright/test';

// E2E data lives outside both web/ and the repo's default
// longtable.db/longtable-assets paths so a test run never touches a
// developer's local room data. See web/.gitignore.
export default defineConfig({
	testDir: 'e2e',
	fullyParallel: false,
	retries: 0,
	use: {
		baseURL: 'http://localhost:5173'
	},
	webServer: [
		{
			// See e2e/run-backend.mjs for why this isn't an inline shell
			// command: cmd.exe and sh disagree on relative-path syntax for
			// the binary being invoked.
			command: 'node e2e/run-backend.mjs',
			url: 'http://localhost:8080/api/healthz',
			reuseExistingServer: false,
			stdout: 'pipe',
			stderr: 'pipe'
		},
		{
			command: 'npm run dev -- --port 5173',
			url: 'http://localhost:5173',
			reuseExistingServer: false,
			stdout: 'pipe',
			stderr: 'pipe'
		}
	]
});
