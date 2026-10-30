import { defineConfig } from '@playwright/test';

// E2E data lives outside both web/ and the repo's default
// longtable.db/longtable-assets paths so a test run never touches a
// developer's local room data. See web/.gitignore.

// Every run keeps its own failures. Playwright empties `outputDir` at the
// start of a run, so with one fixed folder the trace of last night's flake
// was gone the moment anyone ran a single spec to look into it — and the
// flakes here are the kind that don't come back on demand. A folder per
// run, named for when it started, so they sort; only failures leave
// anything in it (see `trace` and `screenshot` below), and both folders
// are gitignored. Delete old ones by hand.
//
// Computed once per process: Playwright loads this config in the runner
// and again in each worker, and a worker computing its own timestamp
// would write to a folder the report doesn't point at.
const runStamp = (process.env.LONGTABLE_E2E_RUN ??= localStamp(new Date()));

// Local time, not `toISOString`'s UTC: these folders get matched against
// the clock on the wall and the machine's own event log, and a name eight
// hours off sent one investigation looking at the wrong half of the day.
function localStamp(d: Date): string {
	const two = (n: number) => String(n).padStart(2, '0');
	return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())}-${two(d.getHours())}-${two(d.getMinutes())}-${two(d.getSeconds())}`;
}

export default defineConfig({
	testDir: 'e2e',
	outputDir: `test-results/${runStamp}`,
	// The HTML report is the way into a failure: each failed test with its
	// screenshot and a link that opens its trace — a filmstrip of what the
	// page showed at every step, canvas included, beside the DOM, console
	// and network. `npx playwright show-report playwright-report/<run>`.
	reporter: [['list'], ['html', { outputFolder: `playwright-report/${runStamp}`, open: 'never' }]],
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
		baseURL: 'http://localhost:8080',
		// Kept only for the runs that fail, which is the whole point: the
		// failures worth debugging here are the ones that happen under a
		// loaded four-worker suite and refuse to happen again on their own.
		// A stack trace says which assertion gave up; a trace says what the
		// page looked like at the time, which for a canvas is the only
		// thing that answers "was it painted yet?".
		//
		// `retain-on-failure` rather than `on-first-retry`, because retries
		// stay at 0 above: a retried flake reads as a pass, and this suite
		// would rather show its flakes than hide them. The cost is that
		// every test records and most recordings are thrown away.
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
		// No `video`, and not by oversight: it is a context-level option
		// applied when Playwright creates the context, and the `table`
		// fixture calls `browser.newContext()` itself for every device at
		// the table. Setting it here records nothing and says it will.
		// Tracing survives that because it hooks the browser fixture, which
		// is why the trace above is the one worth having anyway — it plays
		// back per-action screenshots with the DOM, network and console
		// beside them.
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
