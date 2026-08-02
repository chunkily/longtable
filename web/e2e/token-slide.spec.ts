import { expect, test, type Browser, type Page } from '@playwright/test';

// A token someone else moves used to jump. Proving it slides instead
// means catching it *between* the two squares, which only a real browser
// mid-animation can show — and the assertion has to be that it is in
// neither square rather than that it is in some particular place, since
// where it has got to depends on when the frame landed.

const TOKEN_LAYER = 4;
const GRID = 70;

async function inkAt(page: Page, point: { x: number; y: number }): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 20;
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
		{ layer: TOKEN_LAYER, x: point.x, y: point.y }
	);
}

/**
 * Watches one spot on the token layer on every animation frame until
 * stopped, and reports whether ink ever appeared there.
 *
 * Polling this from the test instead doesn't work: a slide is a one-shot
 * event lasting a fifth of a second, and `expect.poll` that misses the
 * window never gets another chance — which passed when the spec ran
 * alone and failed under a loaded four-worker run. Sampling per frame
 * inside the page sees all ~13 frames of it.
 */
async function watchInkAt(page: Page, point: { x: number; y: number }) {
	await page.evaluate(
		({ layer, x, y }) => {
			const w = window as unknown as { __seen: boolean; __watching: boolean };
			w.__seen = false;
			w.__watching = true;

			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 20;

			const sample = () => {
				if (!w.__watching) return;
				const data = context.getImageData(
					(x - radius) * dpr,
					(y - radius) * dpr,
					radius * 2 * dpr,
					radius * 2 * dpr
				).data;
				for (let i = 3; i < data.length; i += 4) {
					if (data[i] > 0) {
						w.__seen = true;
						break;
					}
				}
				requestAnimationFrame(sample);
			};
			requestAnimationFrame(sample);
		},
		{ layer: TOKEN_LAYER, x: point.x, y: point.y }
	);

	return {
		stop: () =>
			page.evaluate(() => {
				const w = window as unknown as { __seen: boolean; __watching: boolean };
				w.__watching = false;
				return w.__seen;
			})
	};
}

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

function spawnCentre(box: { width: number; height: number }) {
	const cell = {
		x: Math.round(box.width / 2 / GRID),
		y: Math.round(box.height / 2 / GRID)
	};
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

async function openRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	const slug = new URL(page.url()).pathname.split('/').pop()!;

	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

async function createToken(page: Page, name: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
}

test('a token someone else moves slides rather than jumping', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Slide');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const from = spawnCentre(box);
	// Far enough that the two probes can't overlap, and that the slide
	// lasts long enough to be caught in the middle of.
	const to = { x: from.x - 4 * GRID, y: from.y };
	const midpoint = { x: (from.x + to.x) / 2, y: from.y };

	await expect.poll(() => inkAt(player.page, from)).toBeGreaterThan(0);

	// Watching the halfway square before anything moves. Nothing has ever
	// been drawn there, so any ink at all means the token passed through
	// rather than teleporting over it.
	const transit = await watchInkAt(player.page, midpoint);

	// Drag it on the GM's map. The player only learns about it from the
	// broadcast, which is the case that used to teleport.
	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await gm.page.mouse.up();

	// It arrives in the right square...
	await expect.poll(() => inkAt(player.page, to)).toBeGreaterThan(0);
	expect(await inkAt(player.page, from)).toBe(0);
	expect(await inkAt(player.page, midpoint)).toBe(0);

	// ...having been seen in between on the way, which is the whole claim.
	expect(await transit.stop()).toBe(true);

	await gm.context.close();
	await player.context.close();
});

// Whoever did the dragging has already watched the token travel under
// their own pointer. Sliding it a second time when the echo arrives
// would be a visible rubber-band back to the square they left.
test('the person dragging does not see it slide again on the echo', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Slide Self');

	await createToken(gm.page, 'Goblin');
	const box = await canvasBox(gm.page);
	const from = spawnCentre(box);
	const to = { x: from.x - 4 * GRID, y: from.y };
	const midpoint = { x: (from.x + to.x) / 2, y: from.y };

	await expect.poll(() => inkAt(gm.page, from)).toBeGreaterThan(0);

	await gm.page.mouse.move(box.x + from.x, box.y + from.y);
	await gm.page.mouse.down();
	await gm.page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });

	// Started while the button is still down and the token is already at
	// its destination under the pointer, so the drag itself can't trip it.
	// A re-slide on the echo would put the token back at the square it
	// left before tweening forward again, and this would see that frame.
	// Settling and looking afterwards would miss it entirely.
	const rubberBand = await watchInkAt(gm.page, from);
	await gm.page.mouse.up();

	await gm.page.waitForTimeout(600);
	expect(await rubberBand.stop()).toBe(false);
	expect(await inkAt(gm.page, to)).toBeGreaterThan(0);
	expect(await inkAt(gm.page, midpoint)).toBe(0);

	await gm.context.close();
});
