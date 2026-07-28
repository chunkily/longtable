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
import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '..', '..');
const dataDir = path.join(repoRoot, 'web', '.e2e-data');
const binPath = path.join(dataDir, process.platform === 'win32' ? 'longtable-e2e.exe' : 'longtable-e2e');

mkdirSync(dataDir, { recursive: true });

const build = spawnSync('go', ['build', '-o', binPath, './cmd/longtable'], {
	cwd: repoRoot,
	stdio: 'inherit'
});
if (build.status !== 0) {
	process.exit(build.status ?? 1);
}

const server = spawn(
	binPath,
	['-addr', ':8080', '-db', path.join(dataDir, 'longtable.db'), '-assets', path.join(dataDir, 'assets')],
	{ stdio: 'inherit' }
);
server.on('exit', (code) => process.exit(code ?? 0));
