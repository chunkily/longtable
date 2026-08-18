import { expect, test } from './fixtures/table';
import { openRoomMenu } from './fixtures/room';

// Ending a room, which is the one thing in this app with no undo behind
// it. Two browsers, because the half that can't be checked from the
// GM's own screen is what happens to everybody else's.

test('a GM deletes the room, and everyone in it is told rather than left hanging', async ({
	table
}) => {
	const gm = table.gm.page;
	const player = (await table.join()).page;

	await openRoomMenu(gm);
	await gm.getByRole('button', { name: 'Manage room' }).click();

	// Armed before it fires, like every other destructive control here —
	// and the only one guarding something that can't be undone.
	await gm.getByRole('button', { name: 'Delete room' }).click();
	await expect(gm.getByRole('button', { name: 'Really delete this room?' })).toBeVisible();
	await gm.getByRole('button', { name: 'Really delete this room?' }).click();

	// Both browsers end up somewhere that still exists: the GM off the
	// call they made, the player off the socket telling them.
	await expect(gm).toHaveURL(/\/$/);
	await expect(player).toHaveURL(/\/$/);
	await expect(player.getByText('That room has been deleted.')).toBeVisible();

	// The home page lists rooms out of localStorage, so a room left in
	// there would sit at the top of the list one click from a dead end.
	await expect(gm.getByRole('region', { name: 'Your rooms' })).toHaveCount(0);
	await expect(player.getByRole('region', { name: 'Your rooms' })).toHaveCount(0);

	// And the code itself is answered honestly rather than by a loading
	// screen that never resolves.
	await player.goto(`/r/${table.slug}`);
	await expect(player.getByText('Room not found!')).toBeVisible();
});

// A Player is offered nothing to delete the room with. The server
// refuses them too (`requireGM`), which is where it counts — this is
// about the button not being there to press in the first place.
test('a Player has no way to delete the room', async ({ table }) => {
	const player = (await table.join()).page;

	await openRoomMenu(player);
	await expect(player.getByRole('button', { name: 'Manage room' })).toHaveCount(0);
});
