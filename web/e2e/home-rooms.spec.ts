import { expect, test, type Browser, type Page } from '@playwright/test';
import { joinAsGM, joinAsNewPlayer } from './room';

// The home page lists the rooms this browser has been in, and nothing
// else. The half that needs a real browser is the privacy one: a second
// context has its own localStorage, so it stands in for somebody else's
// laptop on the same LAN, and it must not be able to see the first
// person's rooms — by any route, including asking the server directly.

async function createRoom(page: Page, roomName: string, gmName = 'Alice') {
	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill(gmName);
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	return new URL(page.url()).pathname.split('/').pop()!;
}

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

test('a room you create is on your home page, and nobody else can see it', async ({ browser }) => {
	const mine = await browser.newContext();
	const minePage = await mine.newPage();
	await createRoom(minePage, 'Mikes Surprise Party');

	// Back on the home page it's listed, with the role this browser holds.
	await minePage.goto('/');
	await expect(yourRooms(minePage).getByText('Mikes Surprise Party')).toBeVisible();
	await expect(yourRooms(minePage).getByText('GM')).toBeVisible();

	// A different browser is a different person on the same server. They
	// were never told the link, so they see nothing.
	const theirs = await browser.newContext();
	const theirsPage = await theirs.newPage();
	await theirsPage.goto('/');
	await expect(theirsPage.getByText('Nothing here yet')).toBeVisible();
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

test('a player who joins by link gets the room on their own home page', async ({ browser }) => {
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

// Pasting an invite is the only way into a room you haven't been in, so
// it has to take the forms an invite actually arrives in.
test('an invite can be pasted as a link or as a bare code', async ({ browser }) => {
	const gm = await browser.newContext();
	const gmPage = await gm.newPage();
	const slug = await createRoom(gmPage, 'Tomb of Horrors');

	for (const invite of [`http://localhost:5173/r/${slug}`, slug]) {
		const context = await browser.newContext();
		const page = await context.newPage();
		await page.goto('/');
		await page.getByLabel('Have an invite?').fill(invite);
		await page.getByRole('button', { name: 'Join' }).click();
		await expect(page).toHaveURL(new RegExp(`/r/${slug}$`));
		await context.close();
	}

	// Something that isn't an invite says so and stays put, rather than
	// navigating to a room that was never going to exist.
	const context = await browser.newContext();
	const page = await context.newPage();
	await page.goto('/');
	await page.getByLabel('Have an invite?').fill('where is the game');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.getByText("doesn't look like an invite")).toBeVisible();
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
	await expect(gmPage.getByText('Nothing here yet')).toBeVisible();

	// Survives a reload, so it really left storage rather than just the
	// rendered list.
	await gmPage.reload();
	await expect(gmPage.getByText('Nothing here yet')).toBeVisible();

	// The room itself is untouched — the link still works, and a GM who
	// forgot it can log back in with the password.
	await gmPage.goto(`/r/${slug}`);
	await joinAsGM(gmPage, 'Alice', 'hunter2');
	await expect(gmPage.getByRole('region', { name: "Who's connected" })).toBeVisible();

	await gm.close();
});
