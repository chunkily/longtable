import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import { openRoomMenu } from './fixtures/room';

// A GM's own join password, separate from the GM password: it gates
// joining as a Player rather than the GM seat, and is optional — a link
// alone is enough until one is set. Both ways of becoming a Player (a
// fresh seat, and claiming one someone already sat in) go through it,
// and a device that already has a session is never asked.
//
// The pre-join screen asks one question at a time, so a required
// password is its own step before the seat list — checked against the
// server there and then, so a wrong guess is refused before a seat or a
// name is ever asked for, not after.

test.use({ scene: false });

/**
 * Sets the room's join password from Manage room, via the "Password
 * protected?" toggle. Scoped to its own section — the GM password's
 * field carries a label that's also a substring match for this one.
 */
async function setJoinPassword(gmPage: Page, value: string) {
	await openRoomMenu(gmPage);
	await gmPage.getByRole('button', { name: 'Manage room' }).click();
	const section = gmPage.locator('section').filter({ hasText: 'Room join password' });
	await section.getByRole('button', { name: 'Yes', exact: true }).click();
	await section.getByLabel('Password', { exact: true }).fill(value);
	await section.getByRole('button', { name: 'Save' }).click();
	await expect(gmPage.getByText('Password set')).toBeVisible();
	await gmPage.getByRole('button', { name: 'Close' }).click();
}

test('a wrong password is refused immediately, before any seat or name is asked for', async ({
	table,
	browser
}) => {
	await setJoinPassword(table.gm.page, 'correct horse');

	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${table.slug}`);
	await page.getByRole('button', { name: 'Player', exact: true }).click();

	await expect(page.getByLabel('Room password')).toBeVisible();
	await page.getByLabel('Room password').fill('wrong password');
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByText('incorrect room password')).toBeVisible();
	// Refused right here — the seat list and the name box haven't appeared.
	await expect(page.getByRole('button', { name: "I'm new here" })).toHaveCount(0);
	await expect(page.getByLabel('Your name')).toHaveCount(0);

	await page.getByLabel('Room password').fill('correct horse');
	await page.getByRole('button', { name: 'Continue' }).click();
	await expect(page.getByRole('button', { name: "I'm new here" })).toBeVisible();

	await page.getByRole('button', { name: "I'm new here" }).click();
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByText('playing as')).toContainText('Bob');

	await context.close();
});

test('the same password gates claiming a seat someone already sat in', async ({
	table,
	browser
}) => {
	await table.join('Bob');
	await setJoinPassword(table.gm.page, 'correct horse');

	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${table.slug}`);
	await page.getByRole('button', { name: 'Player', exact: true }).click();

	await page.getByLabel('Room password').fill('correct horse');
	await page.getByRole('button', { name: 'Continue' }).click();

	await page.getByRole('button', { name: "Take Bob's seat" }).click();
	await expect(page.getByText('playing as')).toContainText('Bob');

	await context.close();
});

test('a device that already has a session is never asked for the password', async ({ table }) => {
	const bob = await table.join('Bob');
	await setJoinPassword(table.gm.page, 'correct horse');

	await bob.page.reload();
	await expect(bob.page.getByText('playing as')).toContainText('Bob');
	await expect(bob.page.getByLabel('Room password')).toHaveCount(0);
});

test('turning protection off lets anyone back in without a password', async ({
	table,
	browser
}) => {
	const gmPage = table.gm.page;
	await setJoinPassword(gmPage, 'correct horse');

	await openRoomMenu(gmPage);
	await gmPage.getByRole('button', { name: 'Manage room' }).click();
	const section = gmPage.locator('section').filter({ hasText: 'Room join password' });
	await section.getByRole('button', { name: 'No', exact: true }).click();
	await expect(gmPage.getByText('Password removed')).toBeVisible();
	await gmPage.getByRole('button', { name: 'Close' }).click();

	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${table.slug}`);
	await page.getByRole('button', { name: 'Player', exact: true }).click();
	// No password step at all now — straight to the seat list.
	await expect(page.getByLabel('Room password')).toHaveCount(0);
	await expect(page.getByRole('button', { name: "I'm new here" })).toBeVisible();

	await page.getByRole('button', { name: "I'm new here" }).click();
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByText('playing as')).toContainText('Bob');

	await context.close();
});

test('a join password set at room creation gates joining from the start', async ({
	page,
	browser
}) => {
	await page.goto('/');
	await page.waitForLoadState('networkidle');
	await page.getByRole('button', { name: 'Create a room' }).click();
	await expect(page.getByLabel('Room name')).toBeVisible();

	await page.getByLabel('Room name').fill('Protected Room');
	await page.getByLabel('Your name').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Yes', exact: true }).click();
	await page.getByLabel('Join password').fill('tablesecret');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	const slug = new URL(page.url()).pathname.split('/').pop()!;

	const context = await browser.newContext();
	const playerPage = await context.newPage();
	await playerPage.goto(`/r/${slug}`);
	await playerPage.getByRole('button', { name: 'Player', exact: true }).click();

	await expect(playerPage.getByLabel('Room password')).toBeVisible();
	await playerPage.getByLabel('Room password').fill('tablesecret');
	await playerPage.getByRole('button', { name: 'Continue' }).click();
	await playerPage.getByRole('button', { name: "I'm new here" }).click();
	await playerPage.getByLabel('Your name').fill('Bob');
	await playerPage.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(playerPage.getByText('playing as')).toContainText('Bob');

	await context.close();
});

test('leaving the create-room toggle on "No" creates a room nobody needs a password for', async ({
	page
}) => {
	await page.goto('/');
	await page.waitForLoadState('networkidle');
	await page.getByRole('button', { name: 'Create a room' }).click();
	await expect(page.getByLabel('Room name')).toBeVisible();

	await page.getByLabel('Room name').fill('Open Room');
	await page.getByLabel('Your name').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await expect(page.getByLabel('Join password')).toHaveCount(0);
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
});
