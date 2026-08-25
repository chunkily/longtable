import { expect, type Page } from '@playwright/test';

/**
 * Reading and driving the map.
 *
 * Konva has no DOM, so everything about the map is either a pixel probe
 * or a raw mouse gesture — and both are easy to write in a way that
 * passes for the wrong reason. Every spec used to carry its own copy of
 * these, which meant every spec was free to get the waits subtly wrong
 * on its own: `layerInk() > 0` where it meant "*this* token arrived", a
 * click where it meant "click until it lands". The flaky ones were the
 * copies that guessed wrong.
 *
 * Import these rather than re-declaring them. If a wait here turns out
 * to be wrong, it is wrong in one place.
 */

/**
 * One <canvas> per Konva layer, in the order game-canvas.svelte adds
 * them. Several specs index these by number, so a layer inserted
 * anywhere but the end renumbers them — see references/canvas.md.
 */
export const LAYER = {
	map: 0,
	grid: 1,
	fog: 2,
	drawings: 3,
	tokens: 4,
	pings: 5,
	measurements: 6,
	preview: 7,
	selection: 8,
	hover: 9
} as const;

/** The scene dialog's default, and what canvas pixels divide by to give cells. */
export const GRID = 70;

export interface Point {
	x: number;
	y: number;
}

/** Opaque pixels across a whole layer: "is there anything here at all". */
export async function layerInk(page: Page, layer: number): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;

		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, layer);
}

/**
 * Opaque pixels in a box around a point — "is it *here*", which is what
 * makes "it went back to the square it came from" checkable rather than
 * merely "something is on screen".
 *
 * The radius is generous enough to take in a whole 1×1 token wherever a
 * dashed selection ring's gaps happen to fall.
 */
export async function inkAt(page: Page, layer: number, point: Point, radius = 30): Promise<number> {
	return page.evaluate(
		({ layer, x, y, radius }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const data = context.getImageData(
				(x - radius) * dpr,
				(y - radius) * dpr,
				radius * 2 * dpr,
				radius * 2 * dpr
			).data;

			let opaque = 0;
			for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
			return opaque;
		},
		{ layer, x: point.x, y: point.y, radius }
	);
}

/** Ink on the token layer, at a point. The commonest probe by far. */
export function tokenInkAt(page: Page, point: Point): Promise<number> {
	return inkAt(page, LAYER.tokens, point);
}

/**
 * The canvas's position and size, **re-read from the page being
 * clicked**. Never share one page's box with another: the property that
 * makes it safe today — both roles seeing the same layout — is exactly
 * what the next layout change breaks silently, and the symptom is a
 * click that selects nothing with no error anywhere.
 */
export async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

/**
 * Where a freshly created token lands: the cell at the centre of the
 * creator's own view. A fresh scene sits at the identity transform, so
 * canvas pixels are world coordinates — don't pan or zoom in a spec that
 * relies on this.
 */
export function spawnCentre(box: { width: number; height: number }): Point {
	const cell = { x: Math.round(box.width / 2 / GRID), y: Math.round(box.height / 2 / GRID) };
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

/** Drags from one canvas point to another, in page coordinates. */
export async function dragToken(page: Page, from: Point, to: Point) {
	const box = await canvasBox(page);
	await page.mouse.move(box.x + from.x, box.y + from.y);
	await page.mouse.down();
	await page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await page.mouse.up();
}

/**
 * Waits for a move made elsewhere to have finished arriving — not just
 * for the token to be visible at its destination, but for the slide that
 * carried it there to be over.
 *
 * Ink shows up under the probe partway through the tween, so polling for
 * the token at its destination returns *before* it has settled, and a
 * drag started in that window fights the tween and leaves the token
 * where it was. The slide is a fixed 0.22s (TOKEN_MOVE_SECONDS in
 * game-canvas.svelte); this is that with room to spare.
 */
export async function settleAt(page: Page, point: Point) {
	await expect.poll(() => tokenInkAt(page, point)).toBeGreaterThan(0);
	await page.waitForTimeout(400);
}

/**
 * Watch the space *between* two squares for any ink at all, for as long
 * as something might cross it.
 *
 * Use this rather than `watchInkAt` on a midpoint when the claim is "it
 * travelled" rather than "it was exactly here". A slide is a fixed 220ms
 * however far it goes, so there are only about thirteen painted frames
 * in the whole move and easing spends most of them near the ends.
 * Measured on an idle machine, exactly **one** of those frames landed
 * within 20px of the midpoint — so a midpoint probe is a margin-of-one
 * assertion, and it failed roughly once a suite. Watching the whole band
 * between the squares tests the same claim against every frame that
 * crosses it instead of the one that happens to land in the middle.
 *
 * The band stops a square short of each end, so a token sitting still at
 * either end never registers — that clearance is what the midpoint probe
 * was buying with its narrowness, and it is kept here by geometry rather
 * than by being small.
 */
export async function watchInkBetween(page: Page, from: Point, to: Point, layer = LAYER.tokens) {
	const horizontal = Math.abs(to.x - from.x) >= Math.abs(to.y - from.y);
	// Inset along the axis being travelled; on the other axis there is no
	// travel to inset, so cover a square's worth of token instead.
	const inset = { x: horizontal ? GRID : GRID / 2, y: horizontal ? GRID / 2 : GRID };
	const left = Math.min(from.x, to.x) + (horizontal ? inset.x : -inset.x);
	const right = Math.max(from.x, to.x) - (horizontal ? inset.x : -inset.x);
	const top = Math.min(from.y, to.y) + (horizontal ? -inset.y : inset.y);
	const bottom = Math.max(from.y, to.y) - (horizontal ? -inset.y : inset.y);

	return watchInkInRect(page, { left, top, width: right - left, height: bottom - top }, layer);
}

/**
 * Watches one spot on a layer on every animation frame until stopped,
 * and reports whether ink ever appeared there.
 *
 * Polling from the test instead doesn't work for anything transient: a
 * slide is a one-shot event lasting a fifth of a second, and an
 * `expect.poll` that misses the window never gets another chance —
 * which passed when the spec ran alone and failed under a loaded
 * four-worker run. Sampling per frame inside the page sees all ~13
 * frames of it.
 *
 * For "did it travel", use `watchInkBetween`: a point this small on a
 * moving thing's path is a margin-of-one assertion, and the measurement
 * is in that function's comment.
 */
export async function watchInkAt(page: Page, point: Point, layer = LAYER.tokens, radius = 20) {
	return watchInkInRect(
		page,
		{ left: point.x - radius, top: point.y - radius, width: radius * 2, height: radius * 2 },
		layer
	);
}

interface Rect {
	left: number;
	top: number;
	width: number;
	height: number;
}

/**
 * Sample a rectangle every frame until stopped, reporting whether
 * anything was ever painted in it.
 *
 * Sampling on `requestAnimationFrame` is not a detail: it is once per
 * painted frame, which is the most often the pixels can change. It also
 * means a watch only sees what the page actually rendered — an animation
 * starved of frames genuinely never paints the intermediate positions,
 * and no faster sampler would find them.
 *
 * `stop()` reports the frame count as well as the answer, because a
 * watch that never ran reports "never seen" — which is a *pass* for
 * every assertion of the form "this must not appear". Those are the
 * assertions that quietly stop testing anything, so they should check
 * `frames` too.
 */
async function watchInkInRect(page: Page, rect: Rect, layer: number) {
	// Each watch gets its own slot on the page. They used to share one
	// pair of globals, which is fine for one watch and quietly wrong for
	// two: the second reset the first's result, and the first `stop()`
	// switched off both sample loops. The failure mode is the dangerous
	// direction — a spec asserting "this never appeared" passes when its
	// watcher was turned off early.
	const key = `__inkWatch${watchers++}`;

	await page.evaluate(
		({ key, layer, rect }) => {
			const slots = window as unknown as Record<
				string,
				{ seen: boolean; frames: number; watching: boolean }
			>;
			const slot = { seen: false, frames: 0, watching: true };
			slots[key] = slot;

			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;

			const sample = () => {
				if (!slot.watching) return;
				slot.frames++;
				const data = context.getImageData(
					rect.left * dpr,
					rect.top * dpr,
					rect.width * dpr,
					rect.height * dpr
				).data;
				for (let i = 3; i < data.length; i += 4) {
					if (data[i] > 0) {
						slot.seen = true;
						break;
					}
				}
				requestAnimationFrame(sample);
			};
			requestAnimationFrame(sample);
		},
		{ key, layer, rect }
	);

	return {
		stop: () =>
			page.evaluate((key) => {
				const slots = window as unknown as Record<
					string,
					{ seen: boolean; frames: number; watching: boolean }
				>;
				slots[key].watching = false;
				return { seen: slots[key].seen, frames: slots[key].frames };
			}, key)
	};
}

// Only has to be unique within a run; pages are fresh per test anyway.
let watchers = 0;

/** The selected-token panel. Rendered twice; only the rail is visible here. */
export function detailsPanel(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

/**
 * Makes a token and waits for it to actually be on the map, returning
 * the point it landed on.
 *
 * Waits for ink **at the spawn square** rather than anywhere on the
 * layer: a spec that already has tokens would otherwise be told "yes, a
 * token exists" by one that was already there and carry on before the
 * new one arrived. It also waits for the dialog to be gone rather than
 * merely closed — raw mouse coordinates make no actionability checks, so
 * a dialog still running its exit animation swallows the click meant for
 * the canvas.
 */
export async function createToken(
	page: Page,
	name: string,
	options: { count?: number; owner?: string; hidden?: boolean; size?: string } = {}
): Promise<Point> {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	if (options.count && options.count > 1) {
		await page.getByLabel('How many').fill(String(options.count));
	}
	if (options.size) await page.getByRole('button', { name: options.size }).click();
	if (options.owner) await page.getByLabel('Owner').selectOption({ label: options.owner });
	if (options.hidden) await page.getByRole('button', { name: 'Hidden from players' }).click();

	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const spawn = spawnCentre(await canvasBox(page));
	await expect.poll(() => tokenInkAt(page, spawn)).toBeGreaterThan(0);
	return spawn;
}

/**
 * Selects a token by clicking it, **repeatedly until the panel says so**.
 *
 * One click is unreliable and the reason is structural: Konva fires
 * `click` only when mousedown and mouseup land on the same node, and
 * `renderTokens` destroys and rebuilds every token group whenever
 * `room.tokens` changes — so a rebuild landing between the two halves of
 * a click swallows it. The window is about a frame wide, the human
 * answer is to click again, and this is that.
 */
export async function selectToken(page: Page, point: Point, name: string) {
	await expect.poll(() => layerInk(page, LAYER.tokens)).toBeGreaterThan(0);
	const box = await canvasBox(page);
	await expect
		.poll(async () => {
			await page.mouse.click(box.x + point.x, box.y + point.y);
			return (await detailsPanel(page).textContent()) ?? '';
		})
		.toContain(name);
}

/** Opens the edit dialog for whatever is selected. */
export async function openEditor(page: Page) {
	await page.getByRole('button', { name: 'Edit token' }).first().click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeVisible();
}

/** Submits the edit dialog and waits for it to be gone. */
export async function saveEditor(page: Page) {
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeHidden();
}

/**
 * A tracker's box in the details panel. Named "… current value" so it
 * can't collide with the dialog's own "Tracker N value" fields — the
 * panel is rendered twice and the dialog may be open at the same time.
 */
export function trackerBox(page: Page, label: string) {
	return detailsPanel(page).getByLabel(`${label} current value`);
}
