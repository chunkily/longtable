import { expect, test, type Page } from '@playwright/test';
import { PING_LIFETIME_MS, PING_PULSE_INTERVAL_MS } from '../src/lib/ping';

// A ping pulses several times over a few seconds rather than flashing
// once, so it still catches someone who glanced away at the wrong
// moment. Both assertions below fall out of that: there is still a ring
// on screen long after a single pulse would have finished, and the
// marker cleans itself up once the sequence ends.

// Layer order in game-canvas.svelte: map, grid, fog, drawings, tokens,
// pings, measurements, preview.
const PING_LAYER = 5;

// Every visible pixel on the ping layer. A pulse is an expanding ring,
// so its position moves through the sequence — counting the whole layer
// avoids chasing it.
async function pingInk(page: Page): Promise<number> {
	return page.evaluate((layer) => {
		const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;

		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, PING_LAYER);
}

test('a ping keeps pulsing after a single flash would have finished', async ({ page }) => {
	await page.goto('/');
	await page.getByLabel('Room name').fill('Ping');
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	const pingButton = page.getByRole('button', { name: 'Ping', exact: true });
	await pingButton.click();
	await expect(pingButton).toHaveClass(/bg-primary/);

	const box = (await page.locator('canvas').first().boundingBox())!;
	await page.mouse.click(box.x + 300, box.y + 200);

	await expect.poll(() => pingInk(page)).toBeGreaterThan(0);

	// Past the point where the first pulse has finished: anything still
	// drawn is a later one in the sequence.
	await page.waitForTimeout(PING_PULSE_INTERVAL_MS * 2 + 100);
	expect(await pingInk(page)).toBeGreaterThan(0);

	// And it tidies up after itself once the last pulse ends.
	await expect.poll(() => pingInk(page), { timeout: PING_LIFETIME_MS }).toBe(0);
});
