// Builds the whole app — frontend into `web/build`, then the Go binary
// that `go:embed`s it — and runs it, for Playwright's webServer.
//
// **The suite runs against this binary, not against `npm run dev`.** That
// is deliberate and worth not undoing. Vite discovers dependencies
// lazily: the first page load reaching a new import triggers a
// re-optimize, and Vite then tells every connected client to reload
// ("optimized dependencies changed. reloading"). A test whose page is
// reloaded mid-interaction loses what it was doing, and the symptom is a
// click that lands and does nothing. On a cold `node_modules/.vite` that
// cost exactly one failure per worker — 14 workers, 14 failures, each a
// worker's *first* test — which read as flakiness for a long time
// because every run after the first passed.
//
// Serving the built SPA from the Go binary removes that whole class of
// failure rather than timing around it, and it tests the artifact a Host
// actually runs: the embedded frontend, the SPA fallback in
// `internal/api/routes.go`, and same-origin `/api` and `/ws` instead of
// vite's dev proxy. It costs one `npm run build` (~6s), and nothing at
// all in CI, which already builds the frontend before this runs.
//
// For iterating on a failing spec with HMR, run `npm run dev` and a
// backend yourself and point Playwright at :5173 — but don't make that
// the default again.
//
// Deliberately not a shell one-liner: cmd.exe treats a forward-slash
// relative path ("web/.e2e-data/foo.exe") as flags on the "web" command
// rather than a path, while a backslash version would break the
// equivalent Linux CI command. Spawning directly here sidesteps shell
// path-parsing differences entirely.
//
// Building to the same fixed path on every run (rather than `go run`,
// which executes a fresh temp binary each invocation) also means a
// Windows firewall/trust prompt only needs approving once, not on
// every test run.
import { spawn, spawnSync } from 'node:child_process';
import { mkdirSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..', '..');
const dataDir = path.join(repoRoot, 'web', '.e2e-data');
const binPath = path.join(
	dataDir,
	process.platform === 'win32' ? 'longtable-e2e.exe' : 'longtable-e2e'
);

mkdirSync(dataDir, { recursive: true });

// Every run starts from an empty database.
//
// It used to accumulate, and by the time anyone noticed it held a
// thousand rooms — all of which the home page lists. The create-room form
// sits under that list, so `getByRole('button', { name: 'Create room' })`
// was scrolling past a screen-height of links that other workers were
// still adding to, and occasionally clicking where the button had just
// been. That failure looked like "room creation is flaky" and was really
// "the page under test is a thousand rooms tall".
//
// Wiping at start rather than at the end deliberately: whatever the last
// run left is still there to inspect when something fails, which is most
// of why you'd want to look at this database at all.
//
// Set LONGTABLE_E2E_KEEP_DB=1 to append to the previous run instead —
// occasionally useful when reproducing something that needs the state a
// previous run built up.
if (!process.env.LONGTABLE_E2E_KEEP_DB) {
	for (const name of ['longtable.db', 'longtable.db-wal', 'longtable.db-shm']) {
		rmSync(path.join(dataDir, name), { force: true });
	}
	// The blobs are content-addressed, so keeping them past a wiped
	// database would be harmless — but then nothing ever removes them, and
	// "harmless" is how the room table got to a thousand rows.
	rmSync(path.join(dataDir, 'assets'), { recursive: true, force: true });
}

// The frontend first: `go:embed all:web/build` bakes it into the binary
// at compile time, so a stale `web/build` would silently test the last
// build's UI. Always rebuilt rather than checked for freshness — six
// seconds is cheaper than the afternoon spent on a test failing against
// code that isn't the code you just wrote.
// One command string rather than a command plus an args array: with
// `shell: true` node warns that args are concatenated rather than
// escaped, and the shell is needed at all because npm is a .cmd shim on
// Windows that spawn won't resolve on its own.
const web = spawnSync('npm run build', {
	cwd: path.join(repoRoot, 'web'),
	stdio: 'inherit',
	shell: true
});
if (web.error || web.status !== 0) {
	// Reported rather than swallowed: `status` is null when the command
	// couldn't be spawned at all, and `process.exit(null ?? 1)` alone
	// gives Playwright a bare "webServer was not able to start".
	console.error('e2e: frontend build failed:', web.error ?? `exit ${web.status}`);
	process.exit(web.status ?? 1);
}

const build = spawnSync('go', ['build', '-tags', 'nodynamic', '-o', binPath, './cmd/longtable'], {
	cwd: repoRoot,
	stdio: 'inherit'
});
if (build.status !== 0) {
	process.exit(build.status ?? 1);
}

const server = spawn(
	binPath,
	[
		'-addr',
		':8080',
		'-db',
		path.join(dataDir, 'longtable.db'),
		'-assets',
		path.join(dataDir, 'assets')
	],
	{ stdio: 'inherit' }
);
server.on('exit', (code) => process.exit(code ?? 0));
