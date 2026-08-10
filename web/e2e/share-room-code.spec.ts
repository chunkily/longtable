import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';

// Handing the room code to someone who isn't here yet.
//
// There is no copy button and that is the design, not an omission:
// `navigator.clipboard` exists only in a secure context and every Player
// is on `http://192.168.x.x:8080`, so a button would work for whoever is
// developing on localhost and fail for most of the people it's for. What
// the room offers instead is the code itself, readable and selectable,
// and a line pointing at the address bar — which copies on every device
// at the table.
//
// No scene in these rooms — nothing here needs a canvas.
test.use({ scene: false });

// Session info is rendered twice, once in the rail and once in the
// mobile sheet, so every locator here takes `.first()`. Only the rail is
// visible at this viewport; the other copy is display:none, which
// Playwright still counts for strict mode.
const sessionInfo = (page: Page) => page.getByRole('region', { name: 'Session info' }).first();

test('the room code is on screen, with how to pass it on', async ({ table }) => {
	const info = sessionInfo(table.gm.page);

	await expect(info).toContainText(table.slug);
	await expect(info).toContainText('To invite someone');

	// The code is what the room is actually reached by, so this checks the
	// thing on screen against the address rather than against itself — a
	// panel confidently showing the wrong six characters is the failure
	// worth catching, and it would pass any assertion made from the same
	// variable the component read.
	const fromURL = new URL(table.gm.page.url()).pathname.split('/').pop();
	await expect(info).toContainText(fromURL!);
});

// A Player is as likely to be the one messaging whoever is running late,
// and can already read the code out of their own address bar — so it is
// on their screen too rather than being a GM control.
test('a player has the code as well', async ({ table }) => {
	const player = await table.join('Bob');

	await expect(sessionInfo(player.page)).toContainText(table.slug);
	await expect(sessionInfo(player.page)).toContainText('To invite someone');
});
