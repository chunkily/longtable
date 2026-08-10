import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import {
	LAYER,
	createToken,
	detailsPanel,
	layerInk,
	openEditor,
	saveEditor,
	selectToken
} from './fixtures/map';

// Everything a token carries beyond its name and position — size,
// owner, visibility — set both when it's created and when it's edited.
//
// Editing is the first command whose broadcast depends on what the token
// *was*, not only on what it is now: crossing the hidden line tells a
// Player something different in each direction, and neither direction is
// visible from the DOM. So this drives two browsers and reads the canvas.

const TOKEN_LAYER = LAYER.tokens;

/**
 * What size the room believes this token is, read back through the edit
 * dialog's picker. The panel used to spell it out ("2×2 squares") and
 * doesn't any more — a token's footprint is drawn on the map at the size
 * it is — so the pressed option is where the stored value shows up in
 * the DOM now.
 */
async function expectSize(page: Page, label: string) {
	await openEditor(page);
	await expect(page.getByRole('button', { name: label })).toHaveAttribute('aria-pressed', 'true');
	await page.getByRole('button', { name: 'Close' }).click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeHidden();
}

/**
 * The "clicking away" gesture: a click on the dialog's overlay.
 *
 * The wait is not decoration. The dialog attaches its outside-click
 * listener a tick *after* the content paints, so a click sent the
 * instant `Save changes` becomes visible lands before anything is
 * listening and the dialog simply stays open — which reads exactly like
 * dismissal being broken. Only a test can hit that window: a person has
 * just released the mouse on the button that opened it.
 */
async function clickAway(page: Page) {
	await page.waitForTimeout(250);
	await page.mouse.click(5, 5);
}

test('a GM gives a token its size and owner as it is created', async ({ table }) => {
	const gm = table.gm;
	// Bob joins first: the picker offers whoever is connected, and someone
	// who hasn't arrived yet isn't.
	const player = await table.join();

	const spawn = await createToken(gm.page, "Bob's Fighter", {
		size: 'Large (2×2 squares)',
		owner: 'Bob'
	});
	await selectToken(gm.page, spawn, "Bob's Fighter");

	// Whose token it is has to be legible to the room, not just to the GM
	// who assigned it — that is the whole point of an owner, and the
	// Player's client resolves the id through the same roster.
	await expect(detailsPanel(gm.page)).toContainText("Bob's token");
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);
	await selectToken(player.page, spawn, "Bob's token");

	// Handing it back is a real edit, so an owner has to be removable —
	// the update carries the field every time rather than only when it
	// changed, which is what makes "nobody" expressible at all.
	await openEditor(gm.page);
	await expect(gm.page.getByLabel('Owner')).toHaveValue(/.+/);
	await gm.page.getByLabel('Owner').selectOption({ label: 'Nobody (monster or prop)' });
	await saveEditor(gm.page);

	await expect(detailsPanel(gm.page)).not.toContainText("Bob's token");
	await expect(detailsPanel(player.page)).not.toContainText("Bob's token");
	// Still selected and still 2x2 — clearing the owner changed one field.
	await expectSize(gm.page, 'Large (2×2 squares)');
});

// The picker offers who's at the table rather than the room's whole
// roster — a participant row is created on every join, so the roster
// accumulates the same person from a second browser and everyone who
// ever dropped in once.
//
// Which leaves one way to lose data, and this is mostly about that: an
// owner who goes offline has to stay on the list, because the update
// sends the owner every time and a missing option would make the browser
// fall back to "Nobody" and quietly unassign them on the next save.
test('the owner picker offers who is connected, and keeps an owner who leaves', async ({
	table
}) => {
	const gm = table.gm;

	// Alone in the room, and a GM is never offered: owning a token would
	// grant them nothing they can't already do to every token. So there is
	// nobody to hand this one to, which the picker says rather than
	// showing a control with one dead option in it.
	await gm.page.getByRole('button', { name: 'New token' }).click();
	await expect(gm.page.getByText('No players are connected')).toBeVisible();
	await expect(gm.page.getByLabel('Owner')).not.toContainText('Alice');
	await gm.page.getByRole('button', { name: 'Close' }).click();

	const player = await table.join();

	const spawn = await createToken(gm.page, "Bob's Fighter", { owner: 'Bob' });
	await selectToken(gm.page, spawn, "Bob's token");

	// Bob shuts his laptop. He's still on the roster — that's a row in the
	// database — but he is no longer at the table. This close is the
	// *action*, not teardown: the fixture closes every context afterwards
	// anyway, and closing twice is harmless.
	await player.context.close();
	await expect(gm.page.getByRole('region', { name: "Who's connected" }).first()).not.toContainText(
		'Bob'
	);

	await openEditor(gm.page);
	// Still offered, and marked, because he still owns the token.
	await expect(gm.page.getByLabel('Owner')).toContainText('Bob — not connected');

	// The save that would have silently taken the token off him. Renaming
	// is the whole intent here; the owner has to come through untouched.
	await gm.page.getByLabel('Name').fill('Fighter');
	await saveEditor(gm.page);
	await expect(detailsPanel(gm.page)).toContainText('Fighter');
	await expect(detailsPanel(gm.page)).toContainText("Bob's token");

	// And it's the server's answer too, not just this page's state.
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await selectToken(gm.page, spawn, "Bob's token");
});

test('a GM renames and resizes a token, and the whole room sees it', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

	const spawn = await createToken(gm.page, 'Goblin');
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);
	const atOneSquare = await layerInk(player.page, TOKEN_LAYER);

	await selectToken(gm.page, spawn, 'Goblin');

	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Hobgoblin');
	await gm.page.getByRole('button', { name: 'Large (2×2 squares)' }).click();
	await saveEditor(gm.page);

	// The strip reads from room.tokens, so it re-renders from the
	// broadcast rather than from what was typed.
	await expect(detailsPanel(gm.page)).toContainText('Hobgoblin');

	// A 2x2 token covers four times the ground, on the map of someone who
	// only knows through the socket.
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(atOneSquare * 2);

	// And it is really on the server, not just on two canvases.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(atOneSquare * 2);
});

// The two halves of the hidden line, which are the only place this
// command needs to know what the token used to be.
test('hiding a token takes it off the players map, and revealing it puts it back', async ({
	table
}) => {
	const gm = table.gm;
	const player = await table.join();

	const spawn = await createToken(gm.page, 'Ambusher');
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);

	await selectToken(gm.page, spawn, 'Ambusher');

	await openEditor(gm.page);
	await gm.page.getByRole('button', { name: 'Hidden from players' }).click();
	await saveEditor(gm.page);

	// Gone for the player. The GM keeps it — dimmed, but still theirs to
	// see — which is what stops this passing for the wrong reason.
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBe(0);
	expect(await layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);
	await expect(detailsPanel(gm.page)).toContainText('hidden from players');

	// Back the other way. The player was never told the token existed, so
	// what arrives has to be the whole thing rather than a change to
	// something they are holding.
	await openEditor(gm.page);
	await gm.page.getByRole('button', { name: 'Visible', exact: true }).click();
	await saveEditor(gm.page);

	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);
});

test('a player is offered no way to edit a token they can select', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

	const spawn = await createToken(gm.page, 'Goblin');
	await expect.poll(() => layerInk(player.page, TOKEN_LAYER)).toBeGreaterThan(0);

	// Selecting works for anyone — asserted first, so a missed click can't
	// make the absent button look like a permission check.
	await selectToken(player.page, spawn, 'Goblin');
	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeHidden();
});

// The three ways out that keep the token as it was, and the one that
// asks. The failure this is built around: an edit typed, the dialog
// dismissed, and the work gone without anything having said so.
test('cancelling an edit leaves the token alone, whichever way you cancel', async ({ table }) => {
	const gm = table.gm;

	const spawn = await createToken(gm.page, 'Goblin');
	await selectToken(gm.page, spawn, 'Goblin');

	// The Cancel button.
	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Cancelled by button');
	await gm.page.getByRole('button', { name: 'Cancel' }).click();
	await expect(gm.page.getByRole('button', { name: 'Save changes' })).toBeHidden();
	await expect(detailsPanel(gm.page)).toContainText('Goblin');

	// Escape.
	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Cancelled by escape');
	await gm.page.keyboard.press('Escape');
	await expect(gm.page.getByRole('button', { name: 'Save changes' })).toBeHidden();
	await expect(detailsPanel(gm.page)).toContainText('Goblin');

	// The X in the corner.
	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Cancelled by the X');
	await gm.page.getByRole('button', { name: 'Close' }).click();
	await expect(gm.page.getByRole('button', { name: 'Save changes' })).toBeHidden();
	await expect(detailsPanel(gm.page)).toContainText('Goblin');

	// Reopening shows the token, not the last thing typed into it — the
	// form is filled from the token every time it opens.
	await openEditor(gm.page);
	await expect(gm.page.getByLabel('Name')).toHaveValue('Goblin');
	await gm.page.getByRole('button', { name: 'Cancel' }).click();
});

// Clicking away is the ambiguous one — as often a misclick as a
// decision — so it asks instead of guessing, and asks in place of the
// form rather than on top of it.
test('clicking away from an edited form asks, with one dialog on screen', async ({ table }) => {
	const gm = table.gm;

	const spawn = await createToken(gm.page, 'Goblin');
	await selectToken(gm.page, spawn, 'Goblin');

	// Nothing typed: clicking away is just closing, with nothing to ask.
	await openEditor(gm.page);
	await clickAway(gm.page);
	await expect(gm.page.getByRole('button', { name: 'Save changes' })).toBeHidden();
	await expect(gm.page.getByText('Keep your changes')).toHaveCount(0);

	// Something typed: the editor gives way to the question. One dialog at
	// a time is the part worth asserting — the form is gone, not stacked
	// underneath.
	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Hobgoblin');
	await clickAway(gm.page);
	await expect(gm.page.getByText('Keep your changes')).toBeVisible();
	await expect(gm.page.getByLabel('Name')).toHaveCount(0);

	// Back returns to the form with what was typed still in it — the
	// reply to a misclick should be "put it back how it was".
	await gm.page.getByRole('button', { name: 'Back' }).click();
	await expect(gm.page.getByLabel('Name')).toHaveValue('Hobgoblin');

	// Discarding from the question throws the edit away, like every other
	// way out of the editor.
	await clickAway(gm.page);
	await gm.page.getByRole('button', { name: 'Discard changes' }).click();
	await expect(detailsPanel(gm.page)).toContainText('Goblin');

	// And saving from it keeps what was typed, which is the whole reason
	// the values have to survive the swap.
	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Hobgoblin');
	await clickAway(gm.page);
	await gm.page.getByRole('button', { name: 'Save changes' }).click();
	await expect(detailsPanel(gm.page)).toContainText('Hobgoblin');
});

// Token edits are undoable now, which is what makes every decision
// above recoverable rather than final.
test('an edit can be undone and redone, and the room sees both', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

	const spawn = await createToken(gm.page, 'Goblin');
	await selectToken(gm.page, spawn, 'Goblin');
	await selectToken(player.page, spawn, 'Goblin');

	await openEditor(gm.page);
	await gm.page.getByLabel('Name').fill('Hobgoblin');
	await saveEditor(gm.page);
	await expect(detailsPanel(player.page)).toContainText('Hobgoblin');

	// Ctrl+Z puts the old name back for the whole room, not just for the
	// person who pressed it.
	await gm.page.keyboard.press('Control+z');
	await expect(detailsPanel(gm.page)).toContainText('Goblin');
	await expect(detailsPanel(player.page)).toContainText('Goblin');

	await gm.page.keyboard.press('Control+Shift+z');
	await expect(detailsPanel(gm.page)).toContainText('Hobgoblin');
	await expect(detailsPanel(player.page)).toContainText('Hobgoblin');
});
