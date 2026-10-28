import { expect, test } from './fixtures/table';
import { GRID, createToken, dragToken, settleAt, tokenInkAt } from './fixtures/map';
import { createScene, openScenesDialog } from './fixtures/room';

// The party end to end: the GM defines it in the Party dialog — one member
// started from a token on the map, one from nothing — makes a new scene,
// and puts the party on it from the button at the top. The Player stays
// on the old scene the whole time, so a copy landing on *their* map would
// show; that is the scene leak this feature would otherwise have made
// much worse.

const CHAT = 'Say something, or /roll 2d6+3';

test('a GM defines the party and adds it to a new scene', async ({ table }) => {
	const gm = table.gm.page;
	const player = (await table.join('Bob')).page;
	// The fixture leaves the Scenes dialog open over the map.
	await gm.keyboard.press('Escape');

	await expect(player.getByRole('button', { name: 'Party', exact: true })).toHaveCount(0);

	const spawn = await createToken(gm, 'Aria');
	// Off the spawn square, so the copy — which lands on the middle of the
	// GM's view — would show on the Player's map if it ever reached it.
	const aside = { x: spawn.x + GRID * 3, y: spawn.y };
	await dragToken(gm, spawn, aside);
	await settleAt(player, aside);

	// One member from the token on the map, given to Bob…
	await gm.getByRole('button', { name: 'Party', exact: true }).click();
	await expect(gm.getByText('No one in the party yet.')).toBeVisible();
	await gm.getByLabel('Add from this scene').selectOption({ label: 'Aria' });
	await expect(gm.getByLabel('Name')).toHaveValue('Aria');
	await gm.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.getByRole('button', { name: 'Add to party' }).click();

	// …and one from scratch.
	await gm.getByRole('button', { name: 'New member' }).click();
	await gm.getByLabel('Name').fill('Wolf');
	await gm.getByRole('button', { name: /^Large/ }).click();
	await gm.getByRole('button', { name: 'Add to party' }).click();

	await expect(gm.getByRole('button', { name: 'Edit Aria' })).toBeVisible();
	await expect(gm.getByRole('button', { name: 'Edit Wolf' })).toBeVisible();
	// The Wolf isn't coming tonight. Stored, so it's still unticked when the
	// dialog is opened again on the new scene.
	await gm.getByLabel('Wolf is attending').uncheck();
	await expect(gm.getByLabel('Wolf is attending')).not.toBeChecked();
	await gm.keyboard.press('Escape');

	// Making a scene takes the GM there and nobody else.
	await createScene(gm, 'Boss room');
	await gm.keyboard.press('Escape');
	await gm.getByRole('button', { name: 'Party', exact: true }).click();
	await expect(gm.getByLabel('Wolf is attending')).not.toBeChecked();
	await expect(gm.getByLabel('Aria is attending')).toBeChecked();
	await gm.getByRole('button', { name: 'Add to this scene' }).click();
	await expect.poll(() => tokenInkAt(gm, spawn)).toBeGreaterThan(0);

	// A chat line sent after the Add is a point both screens have passed:
	// once the Player has it, the token.created events that went out ahead
	// of it have arrived too, and been dropped rather than drawn.
	await gm.getByPlaceholder(CHAT).fill('party is in');
	await gm.getByPlaceholder(CHAT).press('Enter');
	await expect(player.getByText('party is in')).toBeVisible();
	expect(await tokenInkAt(player, spawn)).toBe(0);

	// The Wolf turns up after all. Ticking him is saved too; whether an
	// unticked member really is skipped is the hub's to prove, and
	// TestPartyAdd_SkipsMembersWhoAreNotAttending does.
	await gm.getByRole('button', { name: 'Party', exact: true }).click();
	await gm.getByLabel('Wolf is attending').check();
	await expect(gm.getByLabel('Wolf is attending')).toBeChecked();
	await gm.getByRole('button', { name: 'Add to this scene' }).click();

	// Now everyone is, and adding again says so — every time, not only the
	// first: the same refusal twice used to toast once.
	for (let press = 1; press <= 2; press++) {
		await gm.getByRole('button', { name: 'Party', exact: true }).click();
		await gm.getByRole('button', { name: 'Add to this scene' }).click();
		await expect(gm.getByText('The whole party is already here.')).toHaveCount(press);
	}

	// And the table, moved there, finds them waiting.
	await openScenesDialog(gm);
	await gm.getByRole('button', { name: 'Move everyone to Boss room' }).click();
	await gm.keyboard.press('Escape');
	await expect.poll(() => tokenInkAt(player, spawn)).toBeGreaterThan(0);
});
