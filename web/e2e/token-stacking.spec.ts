import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import {
	GRID,
	canvasBox,
	createToken,
	detailsPanel,
	selectToken,
	spawnCentre,
	trackerBox
} from './fixtures/map';
import type { Point } from './fixtures/map';

// Which token is on top when two overlap, and what moves one there.
//
// Konva has no DOM, so "on top" is asserted the way a person would find
// out: click where the two overlap and see which one answers. That is
// also the thing being fixed — a token buried under another can't be
// picked up at all.
//
// The overlap is a Large token with a Medium one on its top-left square:
// both cover that square, and the Large one still has three squares of
// its own to be clicked on.

/** The square both tokens stand on, and one only the big token covers. */
async function overlap(page: Page): Promise<{ shared: Point; bigOnly: Point; empty: Point }> {
	const shared = spawnCentre(await canvasBox(page));
	return {
		shared,
		bigOnly: { x: shared.x + GRID, y: shared.y + GRID },
		// Well clear of both tokens, and of the floating toolbar.
		empty: { x: shared.x - 2 * GRID, y: shared.y }
	};
}

/**
 * Puts the selection back to nothing, so the next click has to say which
 * token it landed on rather than agreeing with what was already there.
 */
async function clearSelection(page: Page, at: Point) {
	const box = await canvasBox(page);
	await page.mouse.click(box.x + at.x, box.y + at.y);
	await expect(detailsPanel(page)).not.toContainText('Ogre');
	await expect(detailsPanel(page)).not.toContainText('Rat');
}

/** The pair, big one first, so the small one starts on top of it. */
async function twoTokens(page: Page) {
	await createToken(page, 'Ogre', { size: 'Large' });
	await createToken(page, 'Rat');
	return overlap(page);
}

test('clicking a token brings it out from under the one on top of it', async ({ table }) => {
	const page = table.gm.page;
	const { shared, bigOnly, empty } = await twoTokens(page);

	// Creation order until anyone touches anything: the rat was made
	// last, so it is drawn last and answers for the shared square.
	await selectToken(page, shared, 'Rat');

	// Clicking the ogre where it isn't covered raises the whole token.
	await selectToken(page, bigOnly, 'Ogre');

	await clearSelection(page, empty);
	await selectToken(page, shared, 'Ogre');
});

test('a raised token stays raised when the tokens are rebuilt', async ({ table }) => {
	const page = table.gm.page;
	const { shared, bigOnly, empty } = await twoTokens(page);

	await selectToken(page, bigOnly, 'Ogre');

	// Any change to a token rebuilds every group on the layer, in
	// `room.tokens` order — which is where a `moveToTop` alone would be
	// lost. Editing one is the cheapest way to make that happen.
	await page.getByRole('button', { name: 'Edit token' }).first().click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeVisible();
	await page.getByLabel('Tracker 1 label').fill('HP');
	await page.getByLabel('Tracker 1 value').fill('12');
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeHidden();
	// The panel's trackers are boxes, so the new number is a value rather
	// than text — and waiting for it is what proves the edit came back off
	// the wire and rebuilt the layer.
	await expect(trackerBox(page, 'HP')).toHaveValue('12');

	await clearSelection(page, empty);
	await selectToken(page, shared, 'Ogre');
});

test('selecting a token from the initiative tracker raises it too', async ({ table }) => {
	const page = table.gm.page;
	const { shared, empty } = await twoTokens(page);

	// The tracker's click never reaches the canvas, so this is the path
	// that a raise wired into the stage's own handler would miss.
	await page.getByRole('button', { name: 'Initiative', exact: true }).first().click();
	await page.getByLabel('Combatant').first().selectOption({ label: 'Ogre' });
	await page.getByLabel('Rolled').first().fill('12');
	await page.getByRole('button', { name: 'Add to order' }).first().click();
	await page.getByRole('button', { name: 'Find Ogre on the map' }).first().click();
	await expect(detailsPanel(page)).toContainText('Ogre');

	await clearSelection(page, empty);
	await selectToken(page, shared, 'Ogre');
});

test('the order is this screen alone: not sent to the room, not kept over a reload', async ({
	table
}) => {
	const page = table.gm.page;
	const player = await table.join();
	const { shared, bigOnly, empty } = await twoTokens(page);

	await selectToken(page, bigOnly, 'Ogre');

	// Nobody else's map changed. The rat is still on top over there,
	// because raising is about what *this* person is handling.
	const theirs = spawnCentre(await canvasBox(player.page));
	await selectToken(player.page, theirs, 'Rat');

	// And a reload is a fresh screen with nothing touched on it yet,
	// which is creation order again.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await clearSelection(page, empty);
	await selectToken(page, shared, 'Rat');
});
