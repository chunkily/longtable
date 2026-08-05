// Builds the Go backend to a fixed path and runs it, for Playwright's
// webServer. Deliberately not a shell one-liner: cmd.exe treats a
// forward-slash relative path ("web/.e2e-data/foo.exe") as flags on
// the "web" command rather than a path, while a backslash version
// would break the equivalent Linux CI command. Spawning directly here
// sidesteps shell path-parsing differences entirely.
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
