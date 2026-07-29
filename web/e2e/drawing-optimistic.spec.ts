import { expect, test, type Page, type WebSocketRoute } from '@playwright/test';

// Drawing renders locally the moment you let go, without waiting for the
// server. Timing assertions would only prove that the round trip is
// fast, so these intercept the WebSocket and withhold the server's reply
// entirely: whatever is on the map with the echo dropped got there
// optimistically, and nothing else could have put it there.

const DRAWING_LAYER = 3;

async function inkAt(page: Page, point: { x: number; y: number }): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 6;
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
		{ layer: DRAWING_LAYER, x: point.x, y: point.y }
	);
}

async function canvasOrigin(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y };
}

async function selectTool(page: Page, name: string) {
	const button = page.getByRole('button', { name });
	const alreadyActive = await button.evaluate((el) => el.className.includes('bg-primary'));
	if (!alreadyActive) await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

async function createRoomWithScene(page: Page, name: string) {
	await page.goto('/');
	await page.getByLabel('Room name').fill(name);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);

	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
}

const LINE = { from: { x: 100, y: 100 }, to: { x: 300, y: 200 } };
const MIDPOINT = { x: 200, y: 150 };

async function drawLine(page: Page) {
	await selectTool(page, 'Line');
	const origin = await canvasOrigin(page);
	await page.mouse.move(origin.x + LINE.from.x, origin.y + LINE.from.y);
	await page.mouse.down();
	await page.mouse.move(origin.x + LINE.to.x, origin.y + LINE.to.y, { steps: 8 });
	await page.mouse.up();
}

// Passes every frame through except the ones the test wants withheld,
// handing each intercepted client command to onCommand so a test can
// answer in the server's place.
async function interceptRoomSocket(
	page: Page,
	options: {
		dropServerEvents: string[];
		onCommand?: (type: string, payload: Record<string, unknown>, route: WebSocketRoute) => void;
	}
) {
	await page.routeWebSocket(/\/ws\?/, (route) => {
		const server = route.connectToServer();

		route.onMessage((message) => {
			const text = String(message);
			const { type, payload } = JSON.parse(text);
			options.onCommand?.(type, payload ?? {}, route);
			server.send(text);
		});

		server.onMessage((message) => {
			const text = String(message);
			const { type } = JSON.parse(text);
			if (options.dropServerEvents.includes(type)) return;
			route.send(text);
		});
	});
}

test('a new drawing appears without waiting for the server', async ({ page }) => {
	await interceptRoomSocket(page, { dropServerEvents: ['drawing.created'] });
	await createRoomWithScene(page, 'Optimistic');

	await drawLine(page);

	// The server never confirmed this, and the preview shape is destroyed
	// on mouseup, so the only thing that can be drawing it is the local
	// copy the client added for itself.
	await expect.poll(() => inkAt(page, MIDPOINT)).toBeGreaterThan(0);
	await page.waitForTimeout(500);
	expect(await inkAt(page, MIDPOINT)).toBeGreaterThan(0);
});

test('a drawing the server rejects is taken back off the map', async ({ page }) => {
	// Refuse the stroke in the server's place, naming it the way the hub
	// does, and the client should undo what it drew for itself.
	await interceptRoomSocket(page, {
		dropServerEvents: ['drawing.created'],
		onCommand: (type, payload, route) => {
			if (type !== 'draw.create') return;
			route.send(
				JSON.stringify({
					type: 'error',
					payload: { message: 'nope', drawingId: payload.drawingId }
				})
			);
		}
	});
	await createRoomWithScene(page, 'Rejected');

	await drawLine(page);

	await expect.poll(() => inkAt(page, MIDPOINT)).toBe(0);
	await expect(page.getByText('nope')).toBeVisible();
});

test('an erase clears the stroke without waiting, and reverts if refused', async ({ page }) => {
	let refuse = false;
	await interceptRoomSocket(page, {
		dropServerEvents: ['drawing.deleted'],
		onCommand: (type, payload, route) => {
			if (type !== 'draw.delete' || !refuse) return;
			route.send(
				JSON.stringify({
					type: 'error',
					payload: { message: 'not yours', drawingId: payload.drawingId }
				})
			);
		}
	});
	await createRoomWithScene(page, 'Erase');

	// drawing.created still comes through, so the stroke is confirmed
	// before any of this.
	await drawLine(page);
	await expect.poll(() => inkAt(page, MIDPOINT)).toBeGreaterThan(0);

	// First erase: the confirmation is dropped, so the stroke can only be
	// gone because the client removed it itself.
	await selectTool(page, 'Erase');
	const origin = await canvasOrigin(page);
	await page.mouse.click(origin.x + MIDPOINT.x, origin.y + MIDPOINT.y);
	await expect.poll(() => inkAt(page, MIDPOINT)).toBe(0);

	// Second: draw another and have the server refuse the erase — the
	// stroke has to come back.
	refuse = true;
	await drawLine(page);
	await expect.poll(() => inkAt(page, MIDPOINT)).toBeGreaterThan(0);

	await selectTool(page, 'Erase');
	await page.mouse.click(origin.x + MIDPOINT.x, origin.y + MIDPOINT.y);
	await expect(page.getByText('not yours')).toBeVisible();
	await expect.poll(() => inkAt(page, MIDPOINT)).toBeGreaterThan(0);
});
