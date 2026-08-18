import { expect, test, type Page } from '@playwright/test';
import { createRoom, createScene } from './fixtures/room';
import { LAYER } from './fixtures/map';

// The bold grid: a viewing preference, so everything here is about one
// browser's own screen. Konva paints the grid, which means none of it is
// visible to a DOM assertion — the checks are canvas pixels.

/**
 * How much of the grid layer is painted, waiting until *something* is.
 *
 * A `<canvas>` is in the DOM with a box a frame or more before Konva has
 * drawn into it, and toggling the grid repaints — so reading the moment
 * this is called can catch an empty canvas, which counts as zero and
 * satisfies "the faint grid is fainter" for the wrong reason. See the
 * longer note in theme.spec.ts, which is where that bug was found.
 */
async function gridInk(page: Page): Promise<number> {
	const handle = await page.waitForFunction(
		(layer) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement | undefined;
			const context = canvas?.getContext('2d');
			if (!context) return null;
			const data = context.getImageData(0, 0, canvas!.width, canvas!.height).data;
			let opaque = 0;
			for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
			return opaque > 0 ? opaque : null;
		},
		LAYER.grid,
		{ timeout: 15_000 }
	);
	return handle.jsonValue();
}

const boldButton = (page: Page) => page.getByRole('button', { name: 'Bold grid' });

test('the bold grid is thicker, stays on across a reload, and is this browser alone', async ({
	browser
}) => {
	const context = await browser.newContext();
	const page = await context.newPage();
	const slug = await createRoom(page, 'Grid Contrast');
	await createScene(page, 'Map');

	const faint = await gridInk(page);
	await expect(boldButton(page)).toHaveAttribute('aria-pressed', 'false');

	await boldButton(page).click();
	await expect(boldButton(page)).toHaveAttribute('aria-pressed', 'true');
	// The casing under every line is what makes this measurable: the same
	// lines in the same places, painting more of the canvas.
	await expect.poll(() => gridInk(page)).toBeGreaterThan(faint);

	// Persisted per browser, like the theme control — the point of the
	// setting is not having to set it again every session.
	await page.reload();
	await expect(boldButton(page)).toHaveAttribute('aria-pressed', 'true');
	await expect.poll(() => gridInk(page)).toBeGreaterThan(faint);

	// And it never left this browser: a second device in the same room is
	// still on the faint grid, which is what "not synced to the room"
	// means in the one place it is observable.
	const other = await browser.newContext();
	const otherPage = await other.newPage();
	await otherPage.goto(`/r/${slug}`);
	await otherPage.getByRole('button', { name: "I'm the GM" }).click();
	await otherPage.getByLabel('Your name').fill('Alice');
	await otherPage.getByLabel('GM password').fill('hunter2');
	await otherPage.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(otherPage.getByRole('button', { name: 'Bold grid' })).toHaveAttribute(
		'aria-pressed',
		'false'
	);
	expect(await gridInk(otherPage)).toBe(faint);

	// Off again puts the faint grid back rather than landing somewhere new.
	await boldButton(page).click();
	await expect.poll(() => gridInk(page)).toBe(faint);

	await context.close();
	await other.close();
});

// The bold grid is one fixed pair of colours, unlike the faint one,
// which is picked per scheme. What it has to stand out against is the
// map art rather than the page, so a dark UI must not change it.
test('the bold grid looks the same in both schemes', async ({ browser }) => {
	const inkInScheme = async (colorScheme: 'light' | 'dark') => {
		const context = await browser.newContext({ colorScheme });
		const page = await context.newPage();
		await createRoom(page, `Grid Scheme ${colorScheme}`);
		await createScene(page, 'Map');
		await boldButton(page).click();
		await expect(boldButton(page)).toHaveAttribute('aria-pressed', 'true');
		const ink = await gridInk(page);
		await context.close();
		return ink;
	};

	expect(await inkInScheme('dark')).toBe(await inkInScheme('light'));
});
