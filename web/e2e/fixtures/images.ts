import { fileURLToPath } from 'node:url';

/**
 * Absolute path to an image in this folder, for
 * `locator.setInputFiles(...)`.
 *
 * Resolved against this module rather than the working directory, so a
 * run started from somewhere other than `web/` still finds them —
 * `setInputFiles` resolves a relative path against `process.cwd()`, and
 * the failure it gives when it can't is not obviously about paths.
 *
 * Uploading by path also sends the file's real basename, which is what
 * keeps the filename a spec asserts on tied to the bytes that produce
 * it. See `README.md` beside this file for why that matters more than it looks.
 */
export function fixture(name: string): string {
	return fileURLToPath(new URL(`./${name}`, import.meta.url));
}
