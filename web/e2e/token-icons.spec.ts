import { readFileSync } from 'node:fs';
import { expect, test } from './fixtures/table';
import type { Page } from '@playwright/test';
import { canvasBox, openEditor, selectToken, spawnCentre, tokenInkAt } from './fixtures/map';

// A token without art can wear an icon on a coloured disc. Konva has no
// DOM, so what's checked here is the round trip — chosen in one dialog,
// kept by the server, shown back in another — plus that the token reaches
// the map for someone else. The disc's own colours and ink are unit-tested
// in token-look.test.ts.

/** New token with an icon, found by searching for what it's for. */
async function makeIconToken(
	page: Page,
	name: string,
	search: string,
	icon: string,
	colour: string
) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Icon', exact: true }).click();
	await page.getByLabel('Search icons').fill(search);
	await page.getByRole('button', { name: icon, exact: true }).click();
	await page.getByLabel(colour, { exact: true }).check();
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
	const spawn = spawnCentre(await canvasBox(page));
	await expect.poll(() => tokenInkAt(page, spawn)).toBeGreaterThan(0);
	return spawn;
}

test('an icon and a colour chosen for a token are what its edit dialog shows', async ({
	table
}) => {
	const gm = table.gm.page;
	const player = (await table.join('Bob')).page;
	await gm.keyboard.press('Escape');

	const spawn = await makeIconToken(gm, 'Lich', 'undead', 'Skull', 'Lantern Yellow');
	await expect.poll(() => tokenInkAt(player, spawn)).toBeGreaterThan(0);

	await selectToken(gm, spawn, 'Lich');
	await openEditor(gm);
	await expect(gm.getByRole('button', { name: 'Icon', exact: true })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
	await expect(gm.getByRole('button', { name: 'Skull', exact: true })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
	await expect(gm.getByLabel('Lantern Yellow', { exact: true })).toBeChecked();
});

// Choosing what a token looks like is part of making one, for a Player as
// much as a GM — the same as choosing its art.
test('a Player can give a token they make an icon', async ({ table }) => {
	const gm = table.gm.page;
	const player = (await table.join('Bob')).page;
	await gm.keyboard.press('Escape');

	const spawn = await makeIconToken(player, 'Owl', 'familiar', 'Bird', 'Frost Cyan');
	await expect.poll(() => tokenInkAt(gm, spawn)).toBeGreaterThan(0);
});

// The second pool, vendored from game-icons.net: found and kept the same
// way, and credited under the grid while one of its icons is showing,
// which their licence asks for. Whichever icon is picked first, so
// trimming the picks doesn't break the test.
const firstGameIcon: { label: string } | undefined = JSON.parse(
	readFileSync(new URL('../src/lib/game-icons.picks.json', import.meta.url), 'utf8')
)[0];

test('a game-icons.net icon is credited and kept like any other', async ({ table }) => {
	test.skip(!firstGameIcon, 'no game-icons picked');
	const gm = table.gm.page;
	await gm.keyboard.press('Escape');
	const label = firstGameIcon!.label;

	await gm.getByRole('button', { name: 'New token' }).click();
	await gm.getByRole('button', { name: 'Icon', exact: true }).click();
	await gm.getByLabel('Search icons').fill(label);
	await expect(gm.getByRole('link', { name: 'game-icons.net' })).toBeVisible();
	await gm.keyboard.press('Escape');

	const spawn = await makeIconToken(gm, 'Monster', label, label, 'Lantern Yellow');
	await selectToken(gm, spawn, 'Monster');
	await openEditor(gm);
	await expect(gm.getByRole('button', { name: label, exact: true })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
});
