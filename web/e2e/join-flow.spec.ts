import { expect, test, type Browser } from '@playwright/test';
import { createRoom, joinAsGM, joinAsNewPlayer, openSeatPicker } from './fixtures/room';

// The pre-join screen, which asks one question at a time. The thing
// worth testing here isn't that joining works — every other spec joins —
// it's that each step shows only its own question: the old screen put
// the seat list, the role toggle, a name box and a password field on
// screen at once, and two thirds of that was addressed to somebody else.
//
// No scene is created in these rooms: nothing here needs a canvas.

/** A device with no stored session, arriving at the pre-join screen. */
async function openFreshDevice(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${slug}`);
	return { context, page };
}

test('the join screen asks for a role before it asks for anything else', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Role First');

	const visitor = await openFreshDevice(browser, slug);
	const page = visitor.page;

	// Nothing to fill in yet — the first screen is two answers.
	await expect(page.getByRole('button', { name: 'Player', exact: true })).toBeVisible();
	await expect(page.getByRole('button', { name: "I'm the GM" })).toBeVisible();
	await expect(page.getByLabel('Your name')).toHaveCount(0);
	await expect(page.getByLabel('GM password')).toHaveCount(0);

	// The Player side is the seats, and never the room password: a Player
	// who is shown a password field reads it as one they're expected to
	// know.
	await openSeatPicker(page);
	await expect(page.getByRole('button', { name: "I'm new here" })).toBeVisible();
	await expect(page.getByLabel('GM password')).toHaveCount(0);
	await expect(page.getByLabel('Your name')).toHaveCount(0);

	// A wrong turn costs a click, not a reload.
	await page.getByRole('button', { name: 'Back' }).click();
	await page.getByRole('button', { name: "I'm the GM" }).click();
	await expect(page.getByLabel('GM password')).toBeVisible();
	// And the GM never sees the chairs — the GM seat isn't one of them.
	await expect(page.getByText('Take your seat')).toHaveCount(0);
	await expect(page.getByRole('button', { name: "I'm new here" })).toHaveCount(0);

	await gm.close();
	await visitor.context.close();
});

test('a new player names themselves only after saying they are new', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'New Here');

	const visitor = await openFreshDevice(browser, slug);
	const page = visitor.page;

	// A room nobody has joined still offers the slot — it's the only one
	// on the list, rather than the list being replaced by a name box. And
	// the step says the table is empty rather than leaving that to be
	// inferred from a list with nothing in it, which is also what "we
	// haven't asked yet" looks like.
	await openSeatPicker(page);
	await expect(page.getByRole('button', { name: /Take .*'s seat/ })).toHaveCount(0);
	await expect(page.getByText('Nobody has taken a seat')).toBeVisible();
	await page.getByRole('button', { name: "I'm new here" }).click();

	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByText('playing as')).toContainText('Bob');

	// And now Bob's chair is on the list for the next device that arrives.
	const second = await openFreshDevice(browser, slug);
	await openSeatPicker(second.page);
	await expect(second.page.getByRole('button', { name: "Take Bob's seat" })).toBeVisible();

	await gm.close();
	await visitor.context.close();
	await second.context.close();
});

// The GM path from a device that doesn't remember the room — the case a
// GM hits after clearing a browser, or on a borrowed laptop.
test('the GM signs back in with the room password from a fresh device', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'GM Returns');

	const fresh = await openFreshDevice(browser, slug);
	await joinAsGM(fresh.page, 'Alice', 'hunter2');
	await expect(fresh.page.getByText('gm', { exact: true })).toBeVisible();

	await gm.close();
	await fresh.context.close();
});

// Joining fresh when your own chair is sitting there is the mistake
// seats were built to stop, so the slot has to stay a slot: it must not
// be reachable before the list it belongs to has arrived.
test('the seat list is waited for rather than assumed empty', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Slow Seats');

	const bob = await openFreshDevice(browser, slug);
	await joinAsNewPlayer(bob.page, 'Bob');
	await expect(bob.page.getByText('playing as')).toContainText('Bob');

	// Hold the seat list open until the test lets it go. Until then the
	// step says it's looking, and there is nothing to click — before this,
	// an unanswered list rendered as a table with no chairs at it.
	const phone = await browser.newContext();
	const phonePage = await phone.newPage();
	let release = () => {};
	const held = new Promise<void>((resolve) => (release = resolve));
	await phonePage.route('**/seats', async (route) => {
		await held;
		await route.continue();
	});

	await phonePage.goto(`/r/${slug}`);
	await phonePage.getByRole('button', { name: 'Player', exact: true }).click();
	await expect(phonePage.getByText('Looking for the seats')).toBeVisible();
	await expect(phonePage.getByRole('button', { name: "I'm new here" })).toHaveCount(0);

	release();
	await expect(phonePage.getByRole('button', { name: "Take Bob's seat" })).toBeVisible();

	await gm.close();
	await bob.context.close();
	await phone.close();
});
