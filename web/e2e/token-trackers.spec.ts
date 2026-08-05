import { expect, test, type Browser, type Page } from '@playwright/test';

// Hit points, armour class and conditions on a token — and the first
// thing on a token that someone other than the GM may change, which is
// the half of this that needs two browsers.
//
// The hover card has no DOM behind it (it's a Konva label), so it is
// read off its own canvas layer the way the selection ring is.

// One <canvas> per Konva layer, in the order game-canvas.svelte adds
// them: map, grid, fog, drawings, tokens, pings, measurements, preview,
// selection, hover. Index 9 is the hover card, and it is the only thing
// on that layer — so any ink there at all is the card being up.
const TOKEN_LAYER = 4;
const HOVER_LAYER = 9;
const GRID = 70;

async function layerInk(page: Page, layer: number): Promise<number> {
	return page.evaluate((index) => {
		const canvas = document.querySelectorAll('canvas')[index] as HTMLCanvasElement;
		const context = canvas.getContext('2d')!;
		const data = context.getImageData(0, 0, canvas.width, canvas.height).data;

		let opaque = 0;
		for (let i = 3; i < data.length; i += 4) if (data[i] > 0) opaque++;
		return opaque;
	}, layer);
}

async function canvasBox(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return box;
}

// Where a freshly created token lands — the cell at the centre of the
// creator's view, on a scene still at the identity transform.
function spawnCentre(box: { width: number; height: number }) {
	const cell = {
		x: Math.round(box.width / 2 / GRID),
		y: Math.round(box.height / 2 / GRID)
	};
	return { x: cell.x * GRID + GRID / 2, y: cell.y * GRID + GRID / 2 };
}

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

	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page, slug };
}

async function joinRoomAsPlayer(browser: Browser, slug: string) {
	const context = await browser.newContext();
	const page = await context.newPage();

	await page.goto(`/r/${slug}`);
	await page.getByLabel('Your name').fill('Bob');
	await page.getByRole('button', { name: 'Join' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	return { context, page };
}

function detailsSection(page: Page) {
	return page.getByRole('region', { name: 'Selected token' }).first();
}

async function openEditor(page: Page) {
	await page.getByRole('button', { name: 'Edit token' }).first().click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeVisible();
}

async function save(page: Page) {
	await page.getByRole('button', { name: 'Save changes' }).click();
	await expect(page.getByRole('button', { name: 'Save changes' })).toBeHidden();
}

async function addCondition(page: Page, text: string) {
	await page.getByLabel('Conditions').fill(text);
	await page.getByRole('button', { name: 'Add condition' }).click();
}

// A tracker's box in the details panel, for a client that may edit the
// token. Scoped to the panel and named "… current value" so it can't
// collide with the dialog's own "Tracker N value" fields — the panel is
// rendered twice (desktop sidebar and mobile sheet) and the dialog may
// be open at the same time.
//
// `label` is whatever the slot is called, or "Tracker N" while it has no
// label yet, which is what the panel shows in that case too.
function trackerBox(page: Page, label: string) {
	return detailsSection(page).getByLabel(`${label} current value`);
}

// Selects the token, waiting for it to be drawn first.
//
// **The canvas box is re-read from the page being clicked, never shared
// between two of them.** A GM's toolbar carries a row a Player's doesn't
// (New scene, New token, the fog buttons), so the two canvases sit 44px
// apart vertically even though they're the same size — most of a grid
// square. Reusing one page's box on the other silently clicks the wrong
// cell, and only shows up on a 1×1 token: a 2×2 one is wide enough to
// absorb the error, which is why the specs that pass a box around have
// been getting away with it.
//
// The click is then made inside a poll rather than once, because a
// single one is unreliable for a separate reason: Konva fires `click`
// only when mousedown and mouseup land on the same node, and
// renderTokens destroys and rebuilds every token group on any change to
// room.tokens — so a rebuild landing between the two halves of a click
// swallows it (see references/canvas.md). The window is about a frame
// wide and the human answer is to click again, which is what this does.
//
// Every observing page also selects *before* the edit it is watching
// for, never after. That dodges the worst of that race, and makes the
// assertion stronger: the panel has to update from the event rather than
// merely read correctly when clicked again.
async function selectToken(page: Page, spawn: { x: number; y: number }, name: string) {
	await expect.poll(() => layerInk(page, TOKEN_LAYER)).toBeGreaterThan(0);
	const box = await canvasBox(page);
	await expect
		.poll(async () => {
			await page.mouse.click(box.x + spawn.x, box.y + spawn.y);
			return (await detailsSection(page).textContent()) ?? '';
		})
		.toContain(name);
}

test('a GM sets trackers and conditions, and the whole room can read them', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Goblin');
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, 'Goblin');
	// Selected on the Player's side before the edit, so what follows is
	// the criterion about changes being visible in real time rather than
	// on the next click.
	await selectToken(player.page, spawn, 'Goblin');

	await openEditor(gm.page);
	await gm.page.getByLabel('Tracker 1 label').fill('HP');
	await gm.page.getByLabel('Tracker 1 value').fill('7');
	await gm.page.getByLabel('Tracker 2 label').fill('AC');
	await gm.page.getByLabel('Tracker 2 value').fill('15');
	await addCondition(gm.page, 'Prone');
	await save(gm.page);

	// The panel reads from room.tokens, so this is the broadcast coming
	// back rather than what was typed. The GM may edit this token, so
	// their slots are boxes; the third is empty and still present, which
	// is the point of showing all three.
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('7');
	await expect(trackerBox(gm.page, 'AC')).toHaveValue('15');
	await expect(trackerBox(gm.page, 'Tracker 3')).toHaveValue('');
	await expect(detailsSection(gm.page)).toContainText('Prone');

	// The Player owns nothing here, so they get the read-only rendering —
	// and an unset slot reads as a dash rather than disappearing.
	await expect(detailsSection(player.page)).toContainText('HP 7');
	await expect(detailsSection(player.page)).toContainText('AC 15');
	await expect(detailsSection(player.page)).toContainText('Prone');

	// The other half of the acceptance criteria: the same numbers readable
	// on hover, without going near the details panel. The card has a layer
	// to itself and is the only thing on it, so any ink there is the card.
	// The Player's own canvas box, per selectToken's note. The pointer is
	// still resting on the token from that click, so this walks it off
	// first — which is also the assertion that the card goes away rather
	// than following the pointer around the map for the rest of the
	// session.
	const playerBox = await canvasBox(player.page);
	const away = { x: playerBox.x + spawn.x + GRID * 3, y: playerBox.y + spawn.y + GRID * 3 };
	await player.page.mouse.move(away.x, away.y);
	await expect.poll(() => layerInk(player.page, HOVER_LAYER)).toBe(0);

	await player.page.mouse.move(playerBox.x + spawn.x, playerBox.y + spawn.y);
	await expect.poll(() => layerInk(player.page, HOVER_LAYER)).toBeGreaterThan(0);

	await gm.context.close();
	await player.context.close();
});

// Damage is what changes every round, so changing it doesn't cost a
// dialog: the values are edited in the panel itself. This is also the
// one path where a single number changes and the rest of the token has
// to be filled in from what the client holds — token.update clears what
// it isn't told, so a bug here renames the token to nothing.
test('a number typed into the panel reaches the room, and takes nothing else with it', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Inline');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Goblin');
	await gm.page.getByRole('button', { name: 'Large (2×2 squares)' }).click();
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, 'Goblin');
	await selectToken(player.page, spawn, 'Goblin');

	// The label is still the dialog's job — it's set once when a creature
	// arrives, and a text box for it here would double the strip's width.
	await openEditor(gm.page);
	await gm.page.getByLabel('Tracker 1 label').fill('HP');
	await save(gm.page);

	// The value is not. Committed on blur rather than on every keystroke,
	// so typing "12" doesn't send a 1 on the way past.
	await trackerBox(gm.page, 'HP').fill('12');
	await trackerBox(gm.page, 'HP').blur();

	await expect(detailsSection(player.page)).toContainText('HP 12');

	// Everything the panel never asked about survived being sent back with
	// it: the name, and the size that came from a picker in another form.
	await expect(detailsSection(gm.page)).toContainText('Goblin');
	await expect(detailsSection(gm.page)).toContainText('2×2 squares');

	// Clearing the box empties the slot rather than storing a zero — and
	// zero itself has to still be storable, which is the distinction the
	// whole nullable value exists for.
	await trackerBox(gm.page, 'HP').fill('0');
	await trackerBox(gm.page, 'HP').blur();
	await expect(detailsSection(player.page)).toContainText('HP 0');

	await trackerBox(gm.page, 'HP').fill('');
	await trackerBox(gm.page, 'HP').blur();
	await expect(detailsSection(player.page)).toContainText('HP —');

	// And it is really on the server, not just on two canvases.
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await selectToken(gm.page, spawn, 'Goblin');
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('');

	await gm.context.close();
	await player.context.close();
});

// The step buttons exist only while a box has focus, which is what lets
// the boxes themselves be big enough to read across a table: three
// permanently-visible pairs would cost the width the numbers just took.
// The subtle half is that a step must survive being clicked twice
// without re-focusing — "the ogre takes 7, then 3" is one interaction,
// and a panel that closed on the first click would make it two.
test('the step control appears on focus, adjusts by what it is told, and survives a second click', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Step');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Ogre');
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, 'Ogre');
	await selectToken(player.page, spawn, 'Ogre');

	await openEditor(gm.page);
	await gm.page.getByLabel('Tracker 1 label').fill('HP');
	await save(gm.page);

	const decrease = gm.page.getByRole('button', { name: 'Decrease HP' });
	const increase = gm.page.getByRole('button', { name: 'Increase HP' });

	// Nothing is on screen until a box is being used.
	await expect(decrease).toBeHidden();

	await trackerBox(gm.page, 'HP').fill('30');
	await expect(decrease).toBeVisible();

	// An unset "by how much" means one, so the common case costs no
	// typing at all.
	await decrease.click();
	await expect(detailsSection(player.page)).toContainText('HP 29');

	// Clicking again without touching the box first is the point: focus
	// never left it, so the control is still there.
	await decrease.click();
	await expect(detailsSection(player.page)).toContainText('HP 28');

	// And the box beside them sets the size of the step.
	await gm.page.getByLabel('Adjust HP by').fill('7');
	await decrease.click();
	await expect(detailsSection(player.page)).toContainText('HP 21');
	await increase.click();
	await expect(detailsSection(player.page)).toContainText('HP 28');

	// Focus leaving the control takes it away again, and doesn't undo or
	// re-send anything the buttons already committed. It's the by-box that
	// holds focus by this point rather than the value box — which is
	// itself the proof that clicking the buttons never stole it.
	await gm.page.getByLabel('Adjust HP by').blur();
	await expect(decrease).toBeHidden();
	await expect(detailsSection(player.page)).toContainText('HP 28');

	// An empty slot steps from zero rather than refusing — a creature
	// that has taken damage before anyone wrote down its total is a
	// normal way for this to start.
	await trackerBox(gm.page, 'Tracker 2').click();
	await gm.page.getByRole('button', { name: 'Decrease Tracker 2' }).click();
	await expect(trackerBox(gm.page, 'Tracker 2')).toHaveValue('-1');

	// It really reached the server, not just the two canvases.
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await selectToken(gm.page, spawn, 'Ogre');
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('28');

	await gm.context.close();
	await player.context.close();
});

// A Player who owns the token gets the boxes too — that's the whole
// point of the per-field rule, and the panel is where they'll use it.
test('a player types their own damage into the panel without opening anything', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Inline Owner');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill("Bob's Fighter");
	await gm.page.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, "Bob's Fighter");
	await selectToken(player.page, spawn, "Bob's Fighter");

	// Unlabelled, because a Player can't set a label and shouldn't be
	// locked out of the numbers until a GM gets round to naming them.
	await trackerBox(player.page, 'Tracker 1').fill('5');
	await trackerBox(player.page, 'Tracker 1').blur();

	await expect(trackerBox(gm.page, 'Tracker 1')).toHaveValue('5');
	// The name a Player never had a box for is untouched by their edit.
	await expect(detailsSection(gm.page)).toContainText("Bob's Fighter");

	await gm.context.close();
	await player.context.close();
});

// A token with nothing set on it gets no card, because a card on every
// token the pointer crossed would make the map unusable mid-fight.
test('a token with no trackers or conditions shows nothing on hover', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Empty');

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Rock');
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();
	await expect.poll(() => layerInk(gm.page, TOKEN_LAYER)).toBeGreaterThan(0);

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await gm.page.mouse.move(box.x + spawn.x, box.y + spawn.y);

	// Selecting it proves the pointer really is over the token, so an
	// empty hover layer can't pass for the wrong reason.
	await gm.page.mouse.click(box.x + spawn.x, box.y + spawn.y);
	await expect(detailsSection(gm.page)).toContainText('Rock');
	expect(await layerInk(gm.page, HOVER_LAYER)).toBe(0);

	await gm.context.close();
});

// The per-field permission split. An owner tracks their own damage; the
// name, art, size, owner and visibility stay the GM's, and the form they
// get says so by not being there.
test('a player edits the trackers on their own token and nothing else', async ({ browser }) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Owner');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill("Bob's Fighter");
	await gm.page.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(player.page);
	const spawn = spawnCentre(box);
	await selectToken(player.page, spawn, "Bob's Fighter");
	// The GM watches from a selection made before the Player's edit, for
	// the same reason as above.
	await selectToken(gm.page, spawn, "Bob's Fighter");

	await openEditor(player.page);
	// The GM-only fields aren't on a Player's form at all. The server
	// ignores them for a non-GM regardless — this is the half that keeps
	// them from being offered and then quietly discarded.
	await expect(player.page.getByLabel('Name')).toBeHidden();
	await expect(player.page.getByLabel('Owner')).toBeHidden();
	await expect(player.page.getByRole('button', { name: 'Hidden from players' })).toBeHidden();

	await player.page.getByLabel('Tracker 1 label').fill('HP');
	await player.page.getByLabel('Tracker 1 value').fill('4');
	await addCondition(player.page, 'Poisoned');
	await save(player.page);

	// Both sides may edit this token — the GM anything, Bob its numbers —
	// so both render the value as a box rather than as text.
	await expect(trackerBox(player.page, 'HP')).toHaveValue('4');

	// It reached the GM, which is the point of a Player being able to do
	// it at all — the table sees the damage without being told.
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('4');
	await expect(detailsSection(gm.page)).toContainText('Poisoned');
	// And the name the Player never had a box for came through untouched.
	await expect(detailsSection(gm.page)).toContainText("Bob's Fighter");

	await gm.context.close();
	await player.context.close();
});

// The other side of the same rule: a token nobody owns is nobody's to
// edit but the GM's, and deleting stays GM-only even for one they do own.
test('a player gets no editor for a token they do not own, and never a delete', async ({
	browser
}) => {
	const gm = await openRoomAsGM(browser, 'Token Trackers Not Yours');
	const player = await joinRoomAsPlayer(browser, gm.slug);

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill("Bob's Fighter");
	await gm.page.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(player.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, "Bob's Fighter");
	await selectToken(player.page, spawn, "Bob's Fighter");

	// Owning it earns an editor but never a delete: a token is a piece of
	// the GM's scene that a Player is merely allowed to move and to take
	// damage on.
	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeVisible();
	await expect(player.page.getByRole('button', { name: 'Delete token' })).toBeHidden();

	// Handed back to nobody, the editor goes with it.
	await openEditor(gm.page);
	await gm.page.getByLabel('Owner').selectOption({ label: 'Nobody (monster or prop)' });
	await save(gm.page);

	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeHidden();
	await expect(detailsSection(player.page)).toContainText("Bob's Fighter");

	await gm.context.close();
	await player.context.close();
});
