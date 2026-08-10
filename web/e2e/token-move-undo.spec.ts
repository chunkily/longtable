import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import { GRID, createToken, dragToken, settleAt, tokenInkAt, type Point } from './fixtures/map';

// Undoing a token move over the real stack. Both halves need a browser:
// the move is a drag on a Konva canvas with no DOM to assert on, and the
// claim being made — "back in the square it came from, for everyone" —
// is about where pixels are on a second client's screen.

function undoButton(page: Page) {
	return page.getByRole('button', { name: 'Undo', exact: true });
}

// Anyone at the table can move a token, so anyone has to be able to take
// the move back — this drives it from the Player's browser rather than
// the GM's for that reason.
test('a player undoes their own token move and the whole room sees it go back', async ({
	table
}) => {
	const player = await table.join();

	const spawn = await createToken(player.page, 'Goblin');
	// Far enough that the two probes can't overlap.
	const moved: Point = { x: spawn.x - 3 * GRID, y: spawn.y - 2 * GRID };

	await dragToken(player.page, spawn, moved);
	await expect.poll(() => tokenInkAt(table.gm.page, moved)).toBeGreaterThan(0);

	await undoButton(player.page).click();

	// Back where it started on both screens. The GM never touched it and
	// learns about both the move and its undo the same way — from the
	// broadcast — which is the half that would silently not happen if undo
	// only put the token back locally.
	await expect.poll(() => tokenInkAt(table.gm.page, spawn)).toBeGreaterThan(0);
	expect(await tokenInkAt(table.gm.page, moved)).toBe(0);
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	// And it's really back on the server, not just on two canvases.
	await table.gm.page.reload();
	await expect(table.gm.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => tokenInkAt(table.gm.page, spawn)).toBeGreaterThan(0);
});

// The history can't tell who dragged last, so the position stands in for
// it: a token that isn't where your move left it has been moved since,
// and the entry is passed over rather than dragging it back out from
// under whoever moved it.
test('undo passes over a move once someone else has moved the same token', async ({ table }) => {
	const player = await table.join();

	// The *player* makes the token, and that matters: undo walks back
	// through the stack until something applies, so if the GM had created
	// it their stack would be [create, move] and a declined move-undo
	// would fall through and delete the token — leaving no ink at either
	// square, and this test failing about one run in five depending on
	// whether the deletion's broadcast beat the probe below. Created by
	// the player, the GM's stack holds exactly the one move this test is
	// about.
	const spawn = await createToken(player.page, 'Goblin');
	const byGM: Point = { x: spawn.x - 3 * GRID, y: spawn.y };
	const byPlayer: Point = { x: spawn.x - 3 * GRID, y: spawn.y - 3 * GRID };

	await settleAt(table.gm.page, spawn);
	await dragToken(table.gm.page, spawn, byGM);
	await settleAt(player.page, byGM);

	await dragToken(player.page, byGM, byPlayer);
	await settleAt(table.gm.page, byPlayer);

	// The GM still has their own move on the stack, so the button is live.
	await expect(undoButton(table.gm.page)).toBeEnabled();
	await undoButton(table.gm.page).click();

	// Going disabled is how the test knows the click was handled: the entry
	// was taken off the stack and declined, leaving nothing behind it.
	// Without that signal, "the token didn't move" is a race with an undo
	// that simply hadn't happened yet.
	await expect(undoButton(table.gm.page)).toBeDisabled();
	expect(await tokenInkAt(table.gm.page, byPlayer)).toBeGreaterThan(0);
	expect(await tokenInkAt(table.gm.page, byGM)).toBe(0);
});
