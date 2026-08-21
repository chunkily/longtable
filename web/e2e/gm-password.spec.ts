import { expect, test } from './fixtures/table';
import { openRoomMenu } from './fixtures/room';

// Rotating the room's own GM password from inside the room, which is
// what a GM used to have to ask their Host to do at the command line.
//
// The point of the feature is what happens on the *next* GM login, so
// the check has to leave the room and come back rather than trusting the
// toast.

const OLD = 'hunter2'; // what the fixture creates every room with
const NEW = 'a whole new password';

test('a GM changes the room password, and the next GM login needs the new one', async ({
	table
}) => {
	const page = table.gm.page;

	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Manage room' }).click();

	// Scoped to its own section: the room's separate join password lives
	// in the same dialog and its field carries the same "New password"
	// label — the heading above each says whose, same as the two Save
	// buttons, so a query has to say which section it means.
	const section = page.locator('section').filter({ hasText: 'GM password' });

	// Mismatched halves can't be saved, which is the whole reason there
	// are two boxes: a typo here isn't recoverable from inside the room.
	await section.getByLabel('New password').fill(NEW);
	await section.getByLabel('Confirm password').fill('a whole new passwrod');
	await expect(page.getByText('Both boxes have to match.')).toBeVisible();
	await expect(section.getByRole('button', { name: 'Save' })).toBeDisabled();

	await section.getByLabel('Confirm password').fill(NEW);
	await expect(page.getByText('Both boxes have to match.')).toHaveCount(0);
	await section.getByRole('button', { name: 'Save' }).click();
	await expect(page.getByText('Password changed')).toBeVisible();
	// Emptied on the way out, so what is on screen is never mistaken for
	// the password the room is now holding.
	await expect(section.getByLabel('New password')).toHaveValue('');
	await page.getByRole('button', { name: 'Close' }).click();

	// Nobody is signed out by their own hygiene: this device's session is
	// untouched, which a reload is the honest way to prove.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect(page.getByText('playing as')).toContainText('Alice');

	// Leaving spends the session, so coming back is a real GM login.
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Leave room' }).click();
	await page.getByRole('button', { name: 'Confirm leaving the room' }).click();
	await expect(page).toHaveURL(/\/$/);

	await page.goto(`/r/${table.slug}`);
	// Spelled out rather than through `joinAsGM`, which waits for the
	// form to go — and the whole point of this attempt is that it stays.
	await page.getByRole('button', { name: "I'm the GM" }).click();
	await page.getByLabel('Your name').fill('Alice');
	await page.getByLabel('GM password').fill(OLD);
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByText('incorrect password')).toBeVisible();

	// And the new one lets the same person back into the same seat.
	await page.getByLabel('GM password').fill(NEW);
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect(page.getByText('playing as')).toContainText('Alice');
});
