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
