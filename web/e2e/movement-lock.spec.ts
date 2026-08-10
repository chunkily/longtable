import { expect, test, type Browser, type Page } from '@playwright/test';
import { joinAsNewPlayer, openNewSceneDialog, openRoomMenu } from './fixtures/room';

// The room's owner-only movement setting. Two browsers throughout: the
// claim is about what a *Player* can drag while a GM watches, and the
// setting has to reach them mid-session rather than at their next
// reload.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection, hover.
const TOKEN_LAYER = 4;

// The scene dialog's default, and what canvas-relative pixels are
// divided by to get cells.
const GRID = 70;

interface Point {
	x: number;
	y: number;
}

// Opaque pixels around a point of the token layer — "is the token
// *here*", which is what makes "it didn't move" checkable rather than
// just "something is on screen".
async function tokenInkAt(page: Page, point: Point): Promise<number> {
	return page.evaluate(
		({ layer, x, y }) => {
			const canvas = document.querySelectorAll('canvas')[layer] as HTMLCanvasElement;
			const context = canvas.getContext('2d')!;
			const dpr = window.devicePixelRatio || 1;
			const radius = 30;
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

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

function spawnCentre(box: { width: number; height: number }) {
	const cell = { x: Math.round(box.width / 2 / GRID), y: Math.round(box.height / 2 / GRID) };
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

async function dragToken(page: Page, box: { x: number; y: number }, from: Point, to: Point) {
	await page.mouse.move(box.x + from.x, box.y + from.y);
	await page.mouse.down();
	await page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await page.mouse.up();
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

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, 'Bob');
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

/** Creates a token from the GM's dialog, optionally owned by someone. */
async function createToken(page: Page, name: string, owner?: string) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	if (owner) await page.getByLabel('Owner').selectOption({ label: owner });
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
}

async function setMovement(page: Page, label: 'Anyone moves anything' | 'Only the owner') {
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Manage room' }).click();
	const choice = page.getByRole('button', { name: label });
	await choice.click();
	// The setting round-trips through the server, so the pressed state is
	// the signal it has actually landed rather than merely been clicked.
	await expect(choice).toHaveAttribute('aria-pressed', 'true');
	await page.getByRole('button', { name: 'Close' }).click();
}

test('a locked room holds a player to the tokens they own', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Movement Lock');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	// Two tokens on separate squares: one the player owns, one nobody
	// does. The GM's view is where both spawn, so the second is created
	// after the first has been dragged out of the centre.
	await createToken(gm.page, "Bob's Fighter", 'Bob');
	const gmBox = await canvasBox(gm.page);
	const playerBox = await canvasBox(player.page);
	const spawn = spawnCentre(gmBox);
	const fighterHome = { x: spawn.x - 3 * GRID, y: spawn.y };
	await expect.poll(() => tokenInkAt(gm.page, spawn)).toBeGreaterThan(0);
	await dragToken(gm.page, gmBox, spawn, fighterHome);
	await expect.poll(() => tokenInkAt(player.page, fighterHome)).toBeGreaterThan(0);

	await createToken(gm.page, 'Goblin');
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	// Open table first: the player can shove the goblin around, which is
	// the behaviour the lock is about to take away.
	const shoved = { x: spawn.x, y: spawn.y + 2 * GRID };
	await dragToken(player.page, playerBox, spawn, shoved);
	await expect.poll(() => tokenInkAt(gm.page, shoved)).toBeGreaterThan(0);

	await setMovement(gm.page, 'Only the owner');

	// Mid-session, with no reload on the player's side: the goblin stops
	// being something they can pick up at all.
	await dragToken(player.page, playerBox, shoved, spawn);
	await expect.poll(() => tokenInkAt(player.page, shoved)).toBeGreaterThan(0);
	expect(await tokenInkAt(gm.page, spawn)).toBe(0);

	// Their own fighter still moves, which is the point of the rule being
	// ownership rather than "players can't move anything".
	const advanced = { x: fighterHome.x, y: fighterHome.y + 2 * GRID };
	await dragToken(player.page, playerBox, fighterHome, advanced);
	await expect.poll(() => tokenInkAt(gm.page, advanced)).toBeGreaterThan(0);

	// And the GM is outside the lock they set — otherwise turning it on
	// takes the monsters away from the only person who moves them.
	await dragToken(gm.page, gmBox, shoved, spawn);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	// Unlocked again, the goblin is the player's to shove once more.
	await setMovement(gm.page, 'Anyone moves anything');
	await dragToken(player.page, playerBox, spawn, shoved);
	await expect.poll(() => tokenInkAt(gm.page, shoved)).toBeGreaterThan(0);

	await gm.context.close();
	await player.context.close();
});

// The setting is the room's, not the browser's — a Player arriving after
// it was set has to be held to it too.
test('the lock survives a reload and applies to someone who arrives later', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Movement Lock Persists');

	await createToken(gm.page, 'Goblin');
	const gmBox = await canvasBox(gm.page);
	const spawn = spawnCentre(gmBox);
	await expect.poll(() => tokenInkAt(gm.page, spawn)).toBeGreaterThan(0);

	await setMovement(gm.page, 'Only the owner');
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();

	// Still on after the reload, which is the difference between a setting
	// and a mood.
	await openRoomMenu(gm.page);
	await gm.page.getByRole('button', { name: 'Manage room' }).click();
	await expect(gm.page.getByRole('button', { name: 'Only the owner' })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
	await gm.page.getByRole('button', { name: 'Close' }).click();

	// A player who was never here when it was flipped gets it from
	// state.sync rather than from the event.
	const player = await joinRoomAsPlayer(browser, gm.slug);
	const playerBox = await canvasBox(player.page);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	const shoved = { x: spawn.x, y: spawn.y + 2 * GRID };
	await dragToken(player.page, playerBox, spawn, shoved);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);
	expect(await tokenInkAt(gm.page, shoved)).toBe(0);

	await gm.context.close();
	await player.context.close();
});
