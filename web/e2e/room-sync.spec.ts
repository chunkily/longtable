import { expect, test } from '@playwright/test';
import { createRoom, joinAsNewPlayer } from './fixtures/room';

// Exercises the full stack for real: Go store + WS hub + frontend
// reducer, via two separate browser contexts (so each gets its own
// localStorage, mirroring two different players' browsers).
test('GM creates a room and a player joins and syncs chat live', async ({ browser }) => {
	const gmContext = await browser.newContext();
	const gmPage = await gmContext.newPage();

	const slug = await createRoom(gmPage, 'Curse of Strahd');

	// By role rather than by text: since every page got a real <title>,
	// SvelteKit's live-region announcer holds "Curse of Strahd —
	// Longtable" too, and a bare text match resolves to two elements.
	await expect(gmPage.getByRole('heading', { name: 'Curse of Strahd' })).toBeVisible();
	await expect(gmPage.getByText('gm', { exact: true })).toBeVisible();

	// A second browser context has no session for this room, so it
	// lands on the join form rather than being auto-recognized as GM.
	const playerContext = await browser.newContext();
	const playerPage = await playerContext.newPage();

	await playerPage.goto(`/r/${slug}`);
	await expect(playerPage.getByRole('button', { name: 'Player' })).toBeVisible();
	await joinAsNewPlayer(playerPage, 'Bob');

	await expect(playerPage.getByText('player', { exact: true })).toBeVisible();

	// Chat sent by the GM must reach the player in real time.
	await gmPage.getByPlaceholder('Say something, or /roll 2d6+3').fill('welcome to the party');
	await gmPage.getByRole('button', { name: 'Send' }).click();
	await expect(playerPage.getByText('welcome to the party')).toBeVisible();

	// A /roll command from the player must produce a roll result visible to both.
	await playerPage.getByPlaceholder('Say something, or /roll 2d6+3').fill('/roll 2d6+3');
	await playerPage.getByRole('button', { name: 'Send' }).click();
	await expect(gmPage.getByText(/\/roll 2d6\+3/)).toBeVisible();

	await gmContext.close();
	await playerContext.close();
});
