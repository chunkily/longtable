import { expect, test, type Browser, type Page } from '@playwright/test';
import { createRoom, openRoomMenu } from './fixtures/room';
import { LAYER } from './fixtures/map';

// Light and dark, and the three-way choice between them and the device.
//
// Almost all of this is invisible to a DOM assertion — the scheme is a
// class on <html> and a set of CSS custom properties — so the checks
// here are computed styles and canvas pixels rather than text. The map
// half matters more than it looks: the grid and the no-map slab are
// painted by Konva, which knows nothing about CSS, so they are the one
// part of the app that can silently stay light while everything around
// them goes dark.

const isDark = (page: Page) =>
	page.evaluate(() => document.documentElement.classList.contains('dark'));

/**
 * The colour of the grid lines, read off the grid layer's own canvas.
 *
 * Counting opaque pixels the way the other specs do would pass in both
 * schemes — the lines are there either way. What breaks in the dark is
 * that they are *black*, on a background that is now also black, so the
 * only assertion worth making is about the colour itself.
 */
async function gridLineColor(page: Page): Promise<{ r: number; g: number; b: number }> {
	return page.evaluate((layer) => {
		const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;
		for (let i = 0; i < data.length; i += 4) {
			if (data[i + 3] > 0) return { r: data[i], g: data[i + 1], b: data[i + 2] };
		}
		throw new Error('no grid lines drawn');
	}, LAYER.grid);
}

/** The middle of the map layer, which on a scene with no map is the placeholder slab. */
async function mapCentreColor(page: Page): Promise<{ r: number; g: number; b: number }> {
	return page.evaluate((layer) => {
		const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const d = context.getImageData(
			Math.floor(canvas.width / 2),
			Math.floor(canvas.height / 2),
			1,
			1
		).data;
		return { r: d[0], g: d[1], b: d[2] };
	}, LAYER.map);
}

async function roomWithAMap(browser: Browser, colorScheme: 'light' | 'dark', name: string) {
	const context = await browser.newContext({ colorScheme });
	const page = await context.newPage();
	await createRoom(page, name);
	// A scene, so there is a grid to look at. No map image on it, which
	// is also the case that shows the placeholder slab.
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Scenes', exact: true }).click();
	await page.getByRole('button', { name: 'New scene', exact: true }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene', exact: true }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
	return { context, page };
}

// The one test that would catch the boot script being removed.
//
// mode-watcher would still set the class once the app hydrates, so every
// other test here passes without it — and the bug it prevents is a
// white flash before that happens, which nothing else can see. Blocking
// the app's JavaScript is what makes the difference observable: what's
// left is the HTML the server sent, which is where the fix has to live
// because this is a client-rendered app with no SSR.
test('the scheme is applied before the app has loaded at all', async ({ browser }) => {
	const context = await browser.newContext({ colorScheme: 'dark' });
	const page = await context.newPage();
	await page.route('**/_app/immutable/entry/*.js', (route) => route.abort());

	await page.goto('/', { waitUntil: 'domcontentloaded' });

	expect(await isDark(page)).toBe(true);
	// Set for the browser's own widgets too — scrollbars, and the number
	// inputs on the tracker strip, which stay light against a dark panel
	// without it.
	await expect(page.locator('html')).toHaveAttribute('style', /color-scheme:\s*dark/);

	await context.close();
});

test('a browser with no preference of its own follows the device', async ({ browser }) => {
	for (const [colorScheme, expected] of [
		['dark', true],
		['light', false]
	] as const) {
		const context = await browser.newContext({ colorScheme });
		const page = await context.newPage();
		await page.goto('/');
		await expect(page.getByRole('button', { name: 'Join a room' })).toBeVisible();

		expect(await isDark(page)).toBe(expected);
		await context.close();
	}
});

// The reason there are three options rather than a switch. Following the
// device is a choice, and it has to survive the device changing its mind
// mid-session — which is what happens at sunset on every laptop with
// automatic themes turned on.
test('a browser following the device changes with it, without a reload', async ({ browser }) => {
	const context = await browser.newContext({ colorScheme: 'light' });
	const page = await context.newPage();
	await page.goto('/');
	await expect(page.getByRole('button', { name: 'Join a room' })).toBeVisible();
	expect(await isDark(page)).toBe(false);

	await page.emulateMedia({ colorScheme: 'dark' });
	await expect(page.locator('html')).toHaveClass(/dark/);

	await page.emulateMedia({ colorScheme: 'light' });
	await expect(page.locator('html')).not.toHaveClass(/dark/);

	await context.close();
});

test('a choice made on the home page overrides the device and outlives a reload', async ({
	browser
}) => {
	const context = await browser.newContext({ colorScheme: 'dark' });
	const page = await context.newPage();
	await page.goto('/');
	// The buttons carry an icon and no text, so these names come from
	// their aria-labels. That's the accessible name either way, which is
	// why swapping the words for a sun and a moon left every locator here
	// alone.
	await page.getByRole('button', { name: 'Light', exact: true }).click();
	await expect(page.locator('html')).not.toHaveClass(/dark/);

	await page.reload();
	await expect(page.getByRole('button', { name: 'Join a room' })).toBeVisible();
	await expect(page.locator('html')).not.toHaveClass(/dark/);

	// And the device stops being consulted, which is the whole point of
	// having overridden it.
	await page.emulateMedia({ colorScheme: 'dark' });
	await expect(page.locator('html')).not.toHaveClass(/dark/);

	// System hands it back. A two-state switch would have had nowhere to
	// put this, and the first tap of one would have cost the setting for
	// good.
	await page.getByRole('button', { name: 'System', exact: true }).click();
	await expect(page.locator('html')).toHaveClass(/dark/);

	await context.close();
});

// It floats in the corner rather than sitting in the flow, which is what
// lets it be on every step: the join and create steps each ask one
// question, and a control under one of them would have been a second.
// It used to be under the welcome step's two buttons and reachable from
// nowhere else.
test('the corner control is on every step of the home page', async ({ browser }) => {
	const context = await browser.newContext({ colorScheme: 'dark' });
	const page = await context.newPage();
	await page.goto('/');

	const dark = page.getByRole('button', { name: 'Dark', exact: true });
	await expect(dark).toBeVisible();

	await page.getByRole('button', { name: 'Join a room' }).click();
	await expect(page.getByLabel('Room code')).toBeVisible();
	await expect(dark).toBeVisible();

	// Clickable, not merely on screen — a fixed corner control is exactly
	// the shape that ends up under something else.
	await page.getByRole('button', { name: 'Light', exact: true }).click();
	await expect(page.locator('html')).not.toHaveClass(/dark/);

	await context.close();
});

test('the room menu carries the same control, and the map follows it', async ({ browser }) => {
	const { context, page } = await roomWithAMap(browser, 'dark', 'Dark Tower');

	const darkGrid = await gridLineColor(page);
	const darkSlab = await mapCentreColor(page);
	// Pale lines on a dark map, not the black ones that would be
	// invisible against it.
	expect(darkGrid.r).toBeGreaterThan(128);
	expect(darkSlab.r).toBeLessThan(128);

	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Light', exact: true }).click();

	// The canvas is repainted by an effect, so this is the wait as well
	// as the assertion.
	await expect(page.locator('html')).not.toHaveClass(/dark/);
	await expect
		.poll(async () => (await gridLineColor(page)).r, {
			message: 'grid lines should go back to black on a light map'
		})
		.toBeLessThan(128);
	expect((await mapCentreColor(page)).r).toBeGreaterThan(128);

	await context.close();
});
