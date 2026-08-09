import { expect, test, type Browser, type Locator, type Page } from '@playwright/test';
import { joinAsNewPlayer, openNewSceneDialog } from './room';

// The turn order, driven from two browsers: half of what it claims is
// about what the *Player* is shown — an order they can read, minus the
// combatants they're not supposed to know about.
//
// The panel is rendered twice, once in the rail and once in the mobile
// sheet, so every locator here takes `.first()`. Only the rail is
// visible at this viewport; the other copy is display:none, which
// Playwright still counts for strict mode.

async function openRoomAsGM(browser: Browser, roomName: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	const slug = new URL(page.url()).pathname.split('/').pop()!;

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await joinAsNewPlayer(page, 'Bob');
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

async function createToken(page: Page, name: string, hidden = false) {
	await page.getByRole('button', { name: 'New token' }).click();
	await page.getByLabel('Name').fill(name);
	if (hidden) await page.getByRole('button', { name: 'Hidden from players' }).click();
	await page.getByRole('button', { name: 'Create token' }).click();
	await expect(page.getByRole('button', { name: 'Create token' })).toBeHidden();
}

/** Switches the side panel to the tracker. */
async function openTracker(page: Page) {
	await page.getByRole('button', { name: 'Initiative', exact: true }).first().click();
	await expect(page.getByText(/^Round \d+$/).first()).toBeVisible();
}

/** The order as it reads on screen, top to bottom. */
function orderNames(page: Page): Locator {
	return page.locator('li').filter({ has: page.getByRole('button', { name: /Move .* up/ }) });
}

/** Adds the entry for a token already on the map. */
async function addEntry(page: Page, tokenName: string, rolled: string) {
	await page.getByLabel('Combatant').first().selectOption({ label: tokenName });
	await page.getByLabel('Rolled').first().fill(rolled);
	await page.getByRole('button', { name: 'Add to order' }).first().click();
}

/** Adds an entry for something that isn't on the map at all. */
async function addFreestanding(page: Page, name: string, rolled: string, hidden = false) {
	await page.getByLabel('Combatant').first().selectOption({ label: 'Something else…' });
	await page.getByLabel('Call it').first().fill(name);
	await page.getByLabel('Rolled').first().fill(rolled);
	if (hidden) {
		await page.getByTitle("Keep this off the players' tracker").first().click();
	}
	await page.getByRole('button', { name: 'Add to order' }).first().click();
}

test('the GM builds an order and the room sees it, in initiative order', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Initiative Order');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Goblin');
	await openTracker(gm.page);
	await openTracker(player.page);

	// A token-linked entry takes the token's name; a freestanding one is
	// for the things that aren't on the map at all.
	await addEntry(gm.page, 'Goblin', '12');
	await addFreestanding(gm.page, 'Lair action', '20');

	// Highest first, on both screens — the order *is* the values, so this
	// is the whole feature in one assertion.
	await expect(orderNames(gm.page).first()).toContainText('Lair action');
	await expect(orderNames(gm.page).nth(1)).toContainText('Goblin');
	await expect(player.page.getByText('Lair action').first()).toBeVisible();
	await expect(player.page.getByText('Goblin').first()).toBeVisible();

	await gm.context.close();
	await player.context.close();
});

test('the turn and the round advance for everyone, and wrap together', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Initiative Turns');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await openTracker(gm.page);
	await openTracker(player.page);
	await addFreestanding(gm.page, 'First', '20');
	await addFreestanding(gm.page, 'Second', '10');

	const nextTurn = gm.page.getByRole('button', { name: 'Next turn' }).first();
	await nextTurn.click();

	// The Player is who this is for: they can see whose turn it is
	// without asking.
	const playerRows = player.page.locator('li').filter({ hasText: 'First' });
	await expect(playerRows.first()).toContainText('now');
	await expect(player.page.getByText('Round 1').first()).toBeVisible();

	await nextTurn.click();
	await expect(player.page.locator('li').filter({ hasText: 'Second' }).first()).toContainText(
		'now'
	);
	await expect(player.page.getByText('Round 1').first()).toBeVisible();

	// Wrapping past the last combatant is what makes a round a round.
	await nextTurn.click();
	await expect(player.page.getByText('Round 2').first()).toBeVisible();
	await expect(playerRows.first()).toContainText('now');

	// And back over the wrap lands exactly where it started — the case an
	// off-by-one leaves a table arguing about whether a spell has expired.
	await gm.page.getByRole('button', { name: 'Previous turn' }).first().click();
	await expect(player.page.getByText('Round 1').first()).toBeVisible();
	await expect(player.page.locator('li').filter({ hasText: 'Second' }).first()).toContainText(
		'now'
	);

	await gm.context.close();
	await player.context.close();
});

test('a player is never shown a hidden combatant, and sees one when it is revealed', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Initiative Hidden');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await createToken(gm.page, 'Ambusher', true);
	await openTracker(gm.page);
	await openTracker(player.page);

	await addEntry(gm.page, 'Ambusher', '25');
	await addFreestanding(gm.page, 'Something in the dark', '18', true);
	await addFreestanding(gm.page, 'Bob', '11');

	// The GM sees all three, marked.
	await expect(orderNames(gm.page)).toHaveCount(3);
	await expect(gm.page.getByText('hidden').first()).toBeVisible();

	// The Player sees one. Not filtered out on their side — never sent,
	// so counting the rows can't tell them what's waiting.
	await expect(player.page.getByText('Bob').first()).toBeVisible();
	await expect(player.page.getByText('Ambusher')).toHaveCount(0);
	await expect(player.page.getByText('Something in the dark')).toHaveCount(0);

	// Revealing the token puts its entry on their tracker, which nothing
	// about the tracker itself has any reason to notice.
	await gm.page.getByRole('button', { name: 'Show Something in the dark' }).first().click();
	await expect(player.page.getByText('Something in the dark').first()).toBeVisible();

	await gm.context.close();
	await player.context.close();
});

test('an entry can be re-valued, reordered among ties, and removed', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Initiative Editing');

	await openTracker(gm.page);
	await addFreestanding(gm.page, 'Alice', '15');
	await addFreestanding(gm.page, 'Bob', '15');
	await addFreestanding(gm.page, 'Slow', '5');

	await expect(orderNames(gm.page).first()).toContainText('Alice');

	// Ties are what the manual order is for — the numbers still agree
	// with the list afterwards, which is why it refuses to cross values.
	await gm.page.getByRole('button', { name: 'Move Bob up' }).first().click();
	await expect(orderNames(gm.page).first()).toContainText('Bob');

	// Changing the value re-sorts, which is the point of editing it in
	// place.
	await gm.page.getByLabel('Slow initiative').first().fill('30');
	await gm.page.getByLabel('Slow initiative').first().blur();
	await expect(orderNames(gm.page).first()).toContainText('Slow');

	await gm.page.getByRole('button', { name: 'Remove Slow' }).first().click();
	await expect(orderNames(gm.page)).toHaveCount(2);
	await expect(gm.page.getByText('Slow')).toHaveCount(0);

	await gm.context.close();
});

test('clearing empties the order and leaves the tokens on the map', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Initiative Clear');

	await createToken(gm.page, 'Goblin');
	await openTracker(gm.page);
	await addEntry(gm.page, 'Goblin', '12');
	await gm.page.getByRole('button', { name: 'Next turn' }).first().click();
	await expect(orderNames(gm.page)).toHaveCount(1);

	// Two clicks, like removing a seat: clearing is the one action here
	// the other button can't undo.
	await gm.page.getByRole('button', { name: 'Clear the tracker' }).first().click();
	await gm.page.getByRole('button', { name: 'Confirm clearing the tracker' }).first().click();

	await expect(orderNames(gm.page)).toHaveCount(0);
	await expect(gm.page.getByText('Round 1').first()).toBeVisible();

	// The token is still there: leaving the order is not leaving the map.
	await gm.page.getByRole('button', { name: 'Chat' }).first().click();
	await gm.page.getByRole('button', { name: 'Initiative', exact: true }).first().click();
	await expect(gm.page.getByLabel('Combatant').first()).toContainText('Goblin');

	await gm.context.close();
});

// The last checkbox of token-selection-highlight, which was waiting for
// entries to exist.
test('clicking an entry selects the token it stands for', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Initiative Selects');

	await createToken(gm.page, 'Goblin');
	await openTracker(gm.page);
	await addEntry(gm.page, 'Goblin', '12');

	const details = gm.page.getByRole('region', { name: 'Selected token' }).first();
	// The empty strip is a plain shaded block with no text in it, so
	// "nothing selected" is the absence of the token's name.
	await expect(details).not.toContainText('Goblin');

	await gm.page.getByRole('button', { name: 'Find Goblin on the map' }).first().click();
	await expect(details).toContainText('Goblin');

	await gm.context.close();
});
