import { expect, test } from './fixtures/table';
import { GRID, createToken, dragToken, settleAt, tokenInkAt } from './fixtures/map';
import { createScene, openScenesDialog } from './fixtures/room';

// The party end to end: a GM marks a character, makes a new scene and adds
// the party to it in one click. The Player is the half that matters most —
// they stay on the old scene the whole time the GM is prepping, and a copy
// landing on *their* map is the bug this feature would otherwise have made
// much worse (token.created used to be folded in whatever scene it was for).

const CHAT = 'Say something, or /roll 2d6+3';

test('a GM adds the party to a new scene without it reaching anyone else', async ({ table }) => {
	const gm = table.gm.page;
	const player = (await table.join('Bob')).page;
	// The fixture leaves the Scenes dialog open over the map.
	await gm.keyboard.press('Escape');

	await expect(player.getByRole('button', { name: 'Add party' })).toHaveCount(0);

	const spawn = await createToken(gm, 'Aria', { inParty: true });
	// Off the spawn square, so the copy — which lands on the middle of the
	// GM's view — would show on the Player's map if it ever reached it.
	const aside = { x: spawn.x + GRID * 3, y: spawn.y };
	await dragToken(gm, spawn, aside);
	await settleAt(player, aside);
	await expect.poll(() => tokenInkAt(player, spawn)).toBe(0);

	// Making a scene takes the GM there and nobody else.
	await createScene(gm, 'Boss room');
	await gm.keyboard.press('Escape');
	await gm.getByRole('button', { name: 'Add party' }).click();
	await expect.poll(() => tokenInkAt(gm, spawn)).toBeGreaterThan(0);

	// A chat line sent after the Add party is a point both screens have
	// passed: once the Player has it, the token.created that went out ahead
	// of it has arrived too, and has been dropped rather than drawn.
	await gm.getByPlaceholder(CHAT).fill('party is in');
	await gm.getByPlaceholder(CHAT).press('Enter');
	await expect(player.getByText('party is in')).toBeVisible();
	expect(await tokenInkAt(player, spawn)).toBe(0);

	// Everyone is already here, and pressing it again says so.
	await gm.getByRole('button', { name: 'Add party' }).click();
	await expect(gm.getByText('The whole party is already here.')).toBeVisible();

	// And the table, moved there, finds Aria waiting.
	await openScenesDialog(gm);
	await gm.getByRole('button', { name: 'Move everyone to Boss room' }).click();
	await gm.keyboard.press('Escape');
	await expect.poll(() => tokenInkAt(player, spawn)).toBeGreaterThan(0);
});
