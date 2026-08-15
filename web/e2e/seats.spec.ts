import { expect, test, type Browser, type Page } from '@playwright/test';
import {
	createRoom,
	joinAsGM,
	joinAsNewPlayer,
	openNewSceneDialog,
	openRoomMenu,
	openSeatPicker,
	takeSeat
} from './fixtures/room';

// Taking a seat, from a device that doesn't remember you.
//
// Every test here uses a **second browser context** rather than a second
// tab, and that is the whole point: contexts have their own
// localStorage, so the second one genuinely has no session and has to go
// through the seat picker. Two tabs would share the first one's token
// and prove nothing — see ADR-0008 and the note on the story.

const GRID = 70;
const TOKEN_LAYER = 4;

async function layerInk(page: Page, layer: number): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;
		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, layer);
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

const detailsSection = (page: Page) => page.getByRole('region', { name: 'Selected token' }).first();

async function openRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	const slug = await createRoom(page, roomName);

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

/** A device arriving as someone the room has never seen. */
async function newPlayerDevice(browser: Browser, slug: string, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, name);
	await expect(page.getByText('player', { exact: true })).toBeVisible();

	return { context, page };
}

/** A device with no stored session, arriving at the pre-join screen. */
async function openFreshDevice(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${slug}`);
	return { context, page };
}

// The headline: a cleared browser or a borrowed laptop costs you a
// session, not the tokens you own.
test('a new device takes a seat and gets back the tokens it owns', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Seat Claim');
	const bob = await newPlayerDevice(browser, gm.slug, 'Bob');

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill("Bob's Fighter");
	await gm.page.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);

	// Bob's laptop is gone; this is his phone, which has never been here.
	const phone = await openFreshDevice(browser, gm.slug);
	await takeSeat(phone.page, 'Bob');

	// He is Bob, not a second person called Bob.
	await expect(phone.page.getByText('playing as')).toContainText('Bob');
	await expect(phone.page.locator('canvas').first()).toBeVisible();

	// And the token still belongs to him — nothing about it moved, which
	// is why claiming needs no migration of token rows.
	const box = await canvasBox(phone.page);
	const spawn = spawnCentre(box);
	await expect.poll(() => layerInk(phone.page, TOKEN_LAYER)).toBeGreaterThan(0);
	await phone.page.mouse.click(box.x + spawn.x, box.y + spawn.y);
	await expect(detailsSection(phone.page)).toContainText("Bob's token");

	// The room still has one Bob. This is the assertion that separates a
	// seat from a renamed session: before seats, the phone would be a
	// second participant and the fighter would still belong to the laptop.
	const connected = gm.page.getByRole('region', { name: "Who's connected" }).first();
	await expect(connected.getByText('Bob', { exact: true })).toHaveCount(1);

	await gm.context.close();
	await bob.context.close();
	await phone.context.close();
});

// The criterion the story flags as most likely to be quietly skipped,
// because it only shows up with two real devices.
test('two devices on one seat are one person to everyone else', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Seat Two Devices');
	const laptop = await newPlayerDevice(browser, gm.slug, 'Bob');

	const phone = await openFreshDevice(browser, gm.slug);
	await takeSeat(phone.page, 'Bob');
	await expect(phone.page.getByText('playing as')).toContainText('Bob');

	// Both of Bob's devices are connected at once. The roster shows Alice
	// and one Bob, and the connected list agrees — the hub keys presence
	// on the seat, so a phone and a laptop collapse to one entry exactly
	// as two tabs always did.
	const connected = gm.page.getByRole('region', { name: "Who's connected" }).first();
	await expect(connected.getByText('Bob', { exact: true })).toHaveCount(1);
	await expect(connected.getByText('Alice (GM)')).toBeVisible();

	// Closing one device leaves the other one there: presence drops when
	// the *last* connection on a seat goes, not the first.
	await laptop.context.close();
	await expect(connected.getByText('Bob', { exact: true })).toHaveCount(1);

	await phone.context.close();
	await expect(connected.getByText('Bob', { exact: true })).toHaveCount(0);

	await gm.context.close();
});

// The GM seat is a role boundary rather than an identity one, so it is
// never on the list of chairs anyone can sit in.
test('the GM seat is not offered on the seat picker', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Seat GM Excluded');
	const bob = await newPlayerDevice(browser, gm.slug, 'Bob');

	const fresh = await openFreshDevice(browser, gm.slug);
	await openSeatPicker(fresh.page);
	await expect(fresh.page.getByRole('button', { name: "Take Bob's seat" })).toBeVisible();
	await expect(fresh.page.getByRole('button', { name: "Take Alice's seat" })).toHaveCount(0);

	// The GM side of the first question is how you get that seat, and it
	// still works.
	await fresh.page.getByRole('button', { name: 'Back' }).click();
	await joinAsGM(fresh.page, 'Alice', 'hunter2');
	await expect(fresh.page.getByText('gm', { exact: true })).toBeVisible();

	// And logging in again took the same seat rather than adding one:
	// the picker still offers only Bob's.
	const another = await openFreshDevice(browser, gm.slug);
	await openSeatPicker(another.page);
	await expect(another.page.getByRole('button', { name: "Take Bob's seat" })).toBeVisible();
	await expect(another.page.getByRole('button', { name: /Take .*'s seat/ })).toHaveCount(1);

	await gm.context.close();
	await bob.context.close();
	await fresh.context.close();
	await another.context.close();
});

// A GM sets the table before anyone arrives, and clears a chair away
// when the campaign moves on.
test('a GM adds a seat for someone who has not arrived, and removes one', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Seat Management');

	await openRoomMenu(gm.page);
	await gm.page.getByRole('button', { name: 'Manage room' }).click();
	await gm.page.getByLabel('Add a seat').fill('Carol');
	await gm.page.getByRole('button', { name: 'Add', exact: true }).click();
	await expect(gm.page.getByRole('button', { name: 'Remove Carol' })).toBeVisible();
	await gm.page.getByRole('button', { name: 'Close' }).click();

	// Carol's chair is waiting for her, and taking it makes her Carol
	// rather than asking her to type a name.
	const carol = await openFreshDevice(browser, gm.slug);
	await takeSeat(carol.page, 'Carol');
	await expect(carol.page.getByText('playing as')).toContainText('Carol');
	await carol.context.close();

	// Removing it takes two clicks, like deleting a scene: it signs out
	// every device on the seat and un-owns anything it owned.
	await openRoomMenu(gm.page);
	await gm.page.getByRole('button', { name: 'Manage room' }).click();
	await gm.page.getByRole('button', { name: 'Remove Carol' }).click();
	await gm.page.getByRole('button', { name: 'Confirm removing Carol' }).click();
	await expect(gm.page.getByRole('button', { name: 'Remove Carol' })).toHaveCount(0);

	// The GM's own seat is never removable — the room password signs you
	// into it, so losing it would strand the only role that could help.
	await expect(gm.page.getByRole('button', { name: 'Remove Alice' })).toHaveCount(0);

	await gm.context.close();
});

// Leaving spends a session, not an identity: the seat stays on the
// picker with everything still attached to it.
test('leaving a room leaves the seat behind to come back to', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Seat Leave');
	const bob = await newPlayerDevice(browser, gm.slug, 'Bob');

	await openRoomMenu(bob.page);
	await bob.page.getByRole('button', { name: 'Leave room' }).click();
	await bob.page.getByRole('button', { name: 'Confirm leaving the room' }).click();
	await expect(bob.page).toHaveURL(/\/$/);

	// Same browser, back at the room: its session is gone, so it gets the
	// picker — and Bob's chair is still on it.
	await bob.page.goto(`/r/${gm.slug}`);
	await openSeatPicker(bob.page);
	await expect(bob.page.getByRole('button', { name: "Take Bob's seat" })).toBeVisible();

	await gm.context.close();
	await bob.context.close();
});

// Leave room arms before it fires, so a menu that reopens still armed
// would take a session on the next stray tap. That reset used to hang
// off the menu's own close(); it hangs off the popover's open state now,
// which Escape and a click outside both go through.
test('a half-confirmed Leave room is disarmed by the time the menu comes back', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Seat Disarm');

	await openRoomMenu(gm.page);
	await gm.page.getByRole('button', { name: 'Leave room' }).click();
	await expect(gm.page.getByRole('button', { name: 'Confirm leaving the room' })).toBeVisible();

	await gm.page.keyboard.press('Escape');
	await expect(gm.page.getByRole('button', { name: 'Leave room' })).toBeHidden();
	// Closing hands focus back to the button the menu was opened from,
	// which is the whole reason this menu is on the popover primitive.
	await expect(gm.page.getByRole('button', { name: 'Menu', exact: true })).toBeFocused();

	await openRoomMenu(gm.page);
	await expect(gm.page.getByRole('button', { name: 'Confirm leaving the room' })).toHaveCount(0);

	await gm.context.close();
});
