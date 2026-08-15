import { expect, test } from './fixtures/table';
import type { Page } from '@playwright/test';

// The chat log's two jobs beyond carrying what people typed: saying when
// something happened, and recording who came and went.
//
// Both need a real browser for the same reason — the joined line is
// written by the *server* when a second socket opens, so it only exists
// when there genuinely is a second person in the room.

function chatLog(page: Page) {
	return page.getByRole('list').first();
}

/** A `14:32` stamp, whatever the clock says while this runs. */
const TIME = /^\d{2}:\d{2}$/;

test('a message carries the time it landed, with the full date behind it', async ({ table }) => {
	const page = table.gm.page;

	await page.getByPlaceholder('Say something, or /roll 2d6+3').fill('what time is it');
	await page.getByRole('button', { name: 'Send' }).click();
	await expect(chatLog(page).getByText('what time is it')).toBeVisible();

	// The <time> beside that message, not any of the ones on the joined
	// lines above it.
	const stamp = chatLog(page).locator('li').filter({ hasText: 'what time is it' }).locator('time');
	await expect(stamp).toHaveText(TIME);

	// The tooltip answers the question the short form doesn't — which day
	// — so it has to say more than the four digits already on screen.
	const full = await stamp.getAttribute('title');
	expect(full).toBeTruthy();
	expect(full!.length).toBeGreaterThan(5);
	expect(await stamp.getAttribute('datetime')).toContain('T');
});

test('the room says so when somebody joins, for everyone already there', async ({ table }) => {
	const gm = table.gm;

	// Nobody else has arrived yet, so the log holds the GM's own line and
	// nothing about anyone else.
	await expect(chatLog(gm.page).getByText('Alice joined the room')).toBeVisible();
	await expect(chatLog(gm.page).getByText('Bob joined the room')).toHaveCount(0);

	const player = await table.join('Bob');

	// The GM sees it arrive live; the player reads it out of their own
	// history, which is the half that proves it was stored rather than
	// only broadcast.
	await expect(chatLog(gm.page).getByText('Bob joined the room')).toBeVisible();
	await expect(chatLog(player.page).getByText('Alice joined the room')).toBeVisible();
});

test('a join line survives a refresh, and is there for someone who arrives later', async ({
	table
}) => {
	const gm = table.gm;
	await table.join('Bob');
	await expect(chatLog(gm.page).getByText('Bob joined the room')).toBeVisible();

	await gm.page.reload();
	await expect(chatLog(gm.page).getByText('Bob joined the room')).toBeVisible();

	// A third person, arriving after all of it, reads the same history.
	const late = await table.join('Carol');
	await expect(chatLog(late.page).getByText('Bob joined the room')).toBeVisible();
	await expect(chatLog(late.page).getByText('Alice joined the room')).toBeVisible();
});

// A system line is the room talking, not a person: it carries no name in
// bold and offers no delete button, which is also what stops the log
// filling with bins nobody wants to press.
test('the room own lines are not dressed up as somebody speaking', async ({ table }) => {
	const page = table.gm.page;

	const joined = chatLog(page).locator('li').filter({ hasText: 'Alice joined the room' });
	await expect(joined).toBeVisible();
	await expect(joined.getByRole('button', { name: 'Delete message' })).toHaveCount(0);
	await expect(joined.locator('strong')).toHaveCount(0);
	await expect(joined.locator('time')).toHaveText(TIME);
});

// The delete button is invisible until the message is wanted, and — the
// half that matters — unclickable while invisible. A transparent button
// that still takes clicks means the first tap on a phone can delete a
// message that was never on screen.
test('the delete button stays out of the way until the message is hovered', async ({ table }) => {
	const page = table.gm.page;

	await page.getByPlaceholder('Say something, or /roll 2d6+3').fill('delete me later');
	await page.getByRole('button', { name: 'Send' }).click();
	const message = chatLog(page).locator('li').filter({ hasText: 'delete me later' });
	await expect(message).toBeVisible();

	const bin = message.getByRole('button', { name: 'Delete message' });
	await expect(bin).toHaveCSS('opacity', '0');
	await expect(bin).toHaveCSS('pointer-events', 'none');

	await message.hover();
	await expect(bin).toHaveCSS('opacity', '1');
	await expect(bin).toHaveCSS('pointer-events', 'auto');

	// And still deletes, which is the point of revealing it. The author
	// keeps seeing their own text struck through rather than the
	// bystander's placeholder, so what changes here is the button: one
	// more press purges it.
	await bin.click();
	await expect(message.getByRole('button', { name: 'Remove message permanently' })).toBeVisible();
});

// The touch path: no pointer to rest anywhere, so a tap stands in for
// the hover. Driven by a plain click, which is what a tap produces.
test('tapping a message reveals its delete button, and tapping again puts it away', async ({
	table
}) => {
	const page = table.gm.page;

	await page.getByPlaceholder('Say something, or /roll 2d6+3').fill('tap me');
	await page.getByRole('button', { name: 'Send' }).click();
	const message = chatLog(page).locator('li').filter({ hasText: 'tap me' });
	const bin = message.getByRole('button', { name: 'Delete message' });

	// The pointer has to be parked off the message throughout, or hover
	// answers every assertion here and the tap proves nothing. Clicking
	// necessarily moves it there, so each check is made after moving it
	// away again — which is also the assertion that matters: the tap
	// *latches*, where hover only lasts as long as the pointer does.
	const elsewhere = page.getByPlaceholder('Say something, or /roll 2d6+3');

	await elsewhere.hover();
	await expect(bin).toHaveCSS('opacity', '0');

	await message.click({ position: { x: 5, y: 5 } });
	await elsewhere.hover();
	await expect(bin).toHaveCSS('opacity', '1');

	await message.click({ position: { x: 5, y: 5 } });
	await elsewhere.hover();
	await expect(bin).toHaveCSS('opacity', '0');
});

// A player sees no bin on someone else's message at all — hidden until
// hovered is not the same as absent, and only one of those is a
// permission.
test('a message you may not delete has no button to reveal', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join('Bob');

	await gm.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('the GM speaks');
	await gm.page.getByRole('button', { name: 'Send' }).click();

	const onPlayersScreen = chatLog(player.page).locator('li').filter({ hasText: 'the GM speaks' });
	await expect(onPlayersScreen).toBeVisible();
	await onPlayersScreen.hover();
	await expect(onPlayersScreen.getByRole('button', { name: 'Delete message' })).toHaveCount(0);
});

test('the log puts the date above the day it belongs to', async ({ table }) => {
	const page = table.gm.page;

	// One heading, above everything, since a fresh room's whole log is
	// today — including the joined line the fixture's own arrival wrote.
	await expect(chatLog(page).getByText('Today', { exact: true })).toHaveCount(1);

	await page.getByPlaceholder('Say something, or /roll 2d6+3').fill('still today');
	await page.getByRole('button', { name: 'Send' }).click();
	await expect(chatLog(page).getByText('still today')).toBeVisible();
	await expect(chatLog(page).getByText('Today', { exact: true })).toHaveCount(1);

	// It is the first thing in the list, not floating in the middle of it.
	await expect(chatLog(page).locator('li').first()).toHaveText('Today');
});

// The timestamp sits in a gutter of its own, so it lands in the same
// place on every line. It used to sit at the end of the row, sharing
// that edge with the delete button — which is only rendered for a
// message you may delete, so the time stepped left and right down the
// column depending on whose message it was.
test('the timestamp is in the same place whether or not you may delete the message', async ({
	table
}) => {
	const gm = table.gm;
	const player = await table.join('Bob');

	await gm.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('from the GM');
	await gm.page.getByRole('button', { name: 'Send' }).click();
	await player.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('from the player');
	await player.page.getByRole('button', { name: 'Send' }).click();

	// Read on the player's screen, where one of the two is theirs to
	// delete and the other isn't.
	const log = chatLog(player.page);
	const theirs = log.locator('li').filter({ hasText: 'from the player' }).locator('time');
	const notTheirs = log.locator('li').filter({ hasText: 'from the GM' }).locator('time');
	await expect(theirs).toBeVisible();
	await expect(notTheirs).toBeVisible();

	const [a, b] = [await theirs.boundingBox(), await notTheirs.boundingBox()];
	expect(a!.x).toBeCloseTo(b!.x, 0);
	// And the message text starts in the same place too, past the gutter.
	const bodies = log.locator('li').filter({ hasText: 'from the' }).locator('div');
	const [bodyA, bodyB] = [await bodies.first().boundingBox(), await bodies.last().boundingBox()];
	expect(bodyA!.x).toBeCloseTo(bodyB!.x, 0);
	expect(bodyA!.width).toBeCloseTo(bodyB!.width, 0);

	// The room's own lines lead with their time too, but centred as a
	// pair rather than sharing the gutter — being at a different x from
	// everything else is what makes them skippable on the way past.
	const systemLine = log.locator('li').filter({ hasText: 'joined the room' }).first();
	const systemStamp = systemLine.locator('time');
	const systemBox = await systemStamp.boundingBox();
	const sentenceBox = await systemLine.locator('span').first().boundingBox();

	expect(systemBox!.x).toBeGreaterThan(a!.x);
	expect(sentenceBox!.x).toBeGreaterThan(systemBox!.x);
});
