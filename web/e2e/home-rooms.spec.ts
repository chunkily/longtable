import { expect, test, type Browser, type Page } from '@playwright/test';
import { createRoom, joinAsGM, joinAsNewPlayer } from './fixtures/room';

// The home page: what a browser that has never been anywhere is offered,
// and the rooms it has been to once it has. The half that needs a real
// browser is the privacy one — a second context has its own
// localStorage, so it stands in for somebody else's laptop on the same
// LAN, and it must not be able to see the first person's rooms by any
// route, including asking the server directly.

async function joinRoom(browser: Browser, slug: string, name: string) {
	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, name);
	// Waits on the roster rather than the canvas: these rooms have no
	// scene, so there is nothing for a canvas to draw.
	await expect(page.getByRole('region', { name: "Who's connected" })).toBeVisible();
	return { context, page };
}

const yourRooms = (page: Page) => page.getByRole('region', { name: 'Your rooms' });

// Nobody arrives here expecting a list. They arrive having been sent a
// code, or wanting to start a table — so those are the two things on
// screen, and the list of rooms you have none of isn't drawn at all.
test('a browser that has never been anywhere is offered the two things it can do', async ({
	page
}) => {
	await page.goto('/');

	await expect(page.getByRole('button', { name: 'Join a room' })).toBeVisible();
	await expect(page.getByRole('button', { name: 'Create a room' })).toBeVisible();
	await expect(yourRooms(page)).toHaveCount(0);

	// Each button opens its own step, and neither is on screen until it is
	// asked for: the old page put an empty list, a code box and a
	// three-field create form up at once.
	await page.getByRole('button', { name: 'Join a room' }).click();
	await expect(page.getByLabel('Room code')).toBeVisible();
	await expect(page.getByLabel('Room name')).toHaveCount(0);

	// A wrong turn costs a click, not a reload.
	await page.getByRole('button', { name: 'Back' }).click();
	await page.getByRole('button', { name: 'Create a room' }).click();
	await expect(page.getByLabel('Room name')).toBeVisible();
	await expect(page.getByLabel('Room code')).toHaveCount(0);
});

test('a room you create is on your home page, and nobody else can see it', async ({ browser }) => {
	const mine = await browser.newContext();
	const minePage = await mine.newPage();
	await createRoom(minePage, 'Mikes Surprise Party');

	// Back on the home page it's listed, with the role this browser holds.
	await minePage.goto('/');
	await expect(yourRooms(minePage).getByText('Mikes Surprise Party')).toBeVisible();
	await expect(yourRooms(minePage).getByText('GM')).toBeVisible();

	// A different browser is a different person on the same server. They
	// were never given the code, so they see nothing — and the list isn't
	// there at all rather than being there and empty.
	const theirs = await browser.newContext();
	const theirsPage = await theirs.newPage();
	await theirsPage.goto('/');
	await expect(theirsPage.getByRole('button', { name: 'Join a room' })).toBeVisible();
	await expect(yourRooms(theirsPage)).toHaveCount(0);
	await expect(theirsPage.getByText('Mikes Surprise Party')).toHaveCount(0);

	// And not by asking the server either — the endpoint that used to
	// answer this is gone, so there's no route around the UI.
	const body = await theirsPage.evaluate(async () => {
		const res = await fetch('/api/rooms');
		return { status: res.status, text: await res.text() };
	});
	expect(body.text).not.toContain('Mikes Surprise Party');

	await mine.close();
	await theirs.close();
});

test('a player who joins by code gets the room on their own home page', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Curse of Strahd');

	const player = await joinRoom(browser, slug, 'Bob');

	await player.page.goto('/');
	await expect(yourRooms(player.page).getByText('Curse of Strahd')).toBeVisible();
	// Their role in that room, not the GM's.
	await expect(yourRooms(player.page).getByText('Player')).toBeVisible();
	await expect(yourRooms(player.page).getByText('as Bob')).toBeVisible();

	await gm.close();
	await player.context.close();
});

// A room code is the only way into a room you haven't been in, so the box
// has to take the forms it actually arrives in — six characters read off
// someone's screen, or the whole link they pasted at you.
test('a room code can be pasted as a link or typed as six characters', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Tomb of Horrors');

	for (const pasted of [`http://localhost:5173/r/${slug}`, slug]) {
		const context = await browser.newContext();
		const page = await context.newPage();
		await page.goto('/');
		await page.getByRole('button', { name: 'Join a room' }).click();
		await page.getByLabel('Room code').fill(pasted);
		await page.getByRole('button', { name: 'Join', exact: true }).click();
		await expect(page).toHaveURL(new RegExp(`/r/${slug}$`));
		await context.close();
	}

	// Something that isn't a code says so and stays put, rather than
	// navigating to a room that was never going to exist.
	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto('/');
	await page.getByRole('button', { name: 'Join a room' }).click();
	await page.getByLabel('Room code').fill('where is the game');
	// `exact`, because the welcome step's `Join a room` button also
	// contains "Join" and Playwright matches accessible names by
	// substring unless told otherwise. It isn't on screen at this point,
	// but a locator that would go ambiguous the moment the layout changes
	// is one worth pinning down now.
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByText("doesn't look like a room code")).toBeVisible();
	await expect(page).toHaveURL(/\/$/);

	await context.close();
	await gm.close();
});

test('forgetting a room drops it from this list and leaves the room alone', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Forgettable Keep');

	await gmPage.goto('/');
	await gmPage.getByRole('button', { name: 'Forget Forgettable Keep' }).click();
	await expect(yourRooms(gmPage)).toHaveCount(0);

	// Survives a reload, so it really left storage rather than just the
	// rendered list.
	await gmPage.reload();
	await expect(yourRooms(gmPage)).toHaveCount(0);

	// The room itself is untouched — the code still works, and a GM who
	// forgot it can log back in with the password.
	await gmPage.goto(`/r/${slug}`);
	await joinAsGM(gmPage, 'Alice', 'hunter2');
	await expect(gmPage.getByRole('region', { name: "Who's connected" })).toBeVisible();

	await gm.close();
});
