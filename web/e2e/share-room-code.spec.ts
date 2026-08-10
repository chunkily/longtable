import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import { openRoomMenu } from './fixtures/room';

// Handing the room code to someone who isn't here yet.
//
// There is no copy button and that is the design, not an omission:
// `navigator.clipboard` exists only in a secure context and every Player
// is on `http://192.168.x.x:8080`, so a button would work for whoever is
// developing on localhost and fail for most of the people it's for. What
// the room offers instead is the code readable in the menu, and a dialog
// holding it and the address as readonly fields — one click selects
// either, and Ctrl-C works on every device at the table.
//
// No scene in these rooms — nothing here needs a canvas.
test.use({ scene: false });

const menuCode = (page: Page) => page.getByRole('button', { name: 'Room code' }).first();

test('the code is readable in the menu without opening anything', async ({ table }) => {
	await openRoomMenu(table.gm.page);

	// The entry carries the code itself, so the answer to "what's the
	// code?" costs one tap rather than a dialog.
	await expect(menuCode(table.gm.page)).toContainText(table.slug);
});

test('the menu entry opens both ways of sharing, as selectable fields', async ({ table }) => {
	const page = table.gm.page;
	await openRoomMenu(page);
	await menuCode(page).click();

	// Readonly rather than static text: an <input> is one click plus
	// Ctrl-A to select, where a run of text in a dialog is a drag people
	// miss. Asserting `readonly` is asserting nobody can edit their way
	// into a code that doesn't exist.
	const code = page.getByLabel('Code', { exact: true });
	await expect(code).toHaveValue(table.slug);
	await expect(code).toHaveAttribute('readonly', '');

	// The link is this browser's own address, so it is checked against the
	// page rather than rebuilt from parts — a link that doesn't lead back
	// here is the failure worth catching.
	await expect(page.getByLabel('Link', { exact: true })).toHaveValue(page.url());
});

// A Player is as likely to be the one messaging whoever is running late,
// and can already read the code out of their own address bar — so this
// is not a GM control.
test('a player can share the room too', async ({ table }) => {
	const player = await table.join('Bob');
	await openRoomMenu(player.page);

	await expect(menuCode(player.page)).toContainText(table.slug);
	await menuCode(player.page).click();
	await expect(player.page.getByLabel('Code', { exact: true })).toHaveValue(table.slug);
});
