import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import {
	GRID,
	LAYER,
	createToken,
	detailsPanel,
	dragToken,
	layerInk,
	selectToken,
	tokenInkAt,
	type Point
} from './fixtures/map';

// Deleting a token has to reach the whole room and be recoverable, and
// neither half shows up in the DOM: the token is Konva, and "it came
// back in the same square" is a claim about pixels. So this drives two
// browsers and reads the canvas.

function deleteButton(page: Page) {
	return page.getByRole('button', { name: 'Delete token' }).first();
}

test('a GM deletes the selected token for the whole room, and undo puts it back', async ({
	table
}) => {
	const player = await table.join();
	const gm = table.gm;

	const spawn = await createToken(gm.page, 'Goblin');
	// Two cells up and left of where it spawned, so what's asserted below
	// is that the token came back *where it was*, not merely that a token
	// with that name exists again.
	const moved: Point = { x: spawn.x - 2 * GRID, y: spawn.y - 2 * GRID };

	// Selected *before* the drag rather than after it, and not for
	// convenience: renderTokens destroys and rebuilds every token group
	// when the token.moved echo arrives, and Konva only fires `click` if
	// mousedown and mouseup landed on the same node. A click sent straight
	// after a drag races that rebuild and is silently swallowed — it fails
	// about three runs in four. The selection survives a drag either way
	// (token-selection.spec.ts proves that), so this covers the same
	// ground without the race.
	await selectToken(gm.page, spawn, 'Goblin');

	await dragToken(gm.page, spawn, moved);
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);

	await deleteButton(gm.page).click();

	// Gone for the player, who only knows through the broadcast — and the
	// GM's strip falls back to its empty state because the token behind
	// the selection is no longer there.
	await expect.poll(() => layerInk(player.page, LAYER.tokens)).toBe(0);
	await expect.poll(() => layerInk(gm.page, LAYER.tokens)).toBe(0);
	await expect(detailsPanel(gm.page)).not.toContainText('Goblin');

	await gm.page.getByRole('button', { name: 'Undo', exact: true }).click();

	// Back in the square it was standing in, for everyone.
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);
	expect(await tokenInkAt(player.page, spawn)).toBe(0);
	// And selected again on the GM's screen: the id was never cleared, so
	// a token returning under it returns selected. Deliberate — see the
	// backlog note — and the reason nothing has to clear it by hand.
	await expect(detailsPanel(gm.page)).toContainText('Goblin');

	// It's really on the server, not just back on two canvases.
	await player.page.reload();
	await expect(player.page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => tokenInkAt(player.page, moved)).toBeGreaterThan(0);
});

test('a player can select a token but is offered no way to delete it', async ({ table }) => {
	const player = await table.join();

	const spawn = await createToken(table.gm.page, 'Goblin');
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	// Selecting works for anyone; the delete button is the GM's alone. The
	// selection succeeding is what stops this from passing for the wrong
	// reason — a missed click would show no button either.
	await selectToken(player.page, spawn, 'Goblin');
	await expect(deleteButton(player.page)).toBeHidden();
	await expect(deleteButton(table.gm.page)).toBeHidden(); // nothing selected there yet
});
