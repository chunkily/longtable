import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import {
	GRID,
	LAYER,
	canvasBox,
	detailsPanel,
	layerInk,
	openEditor,
	saveEditor,
	selectToken,
	spawnCentre,
	trackerBox
} from './fixtures/map';

// Hit points, armour class and conditions on a token — and the first
// thing on a token that someone other than the GM may change, which is
// the half of this that needs two browsers.
//
// The hover card has no DOM behind it (it's a Konva label), so it is
// read off its own canvas layer the way the selection ring is. Index 9
// is the only thing on that layer, so any ink there at all is the card
// being up.
const HOVER_LAYER = LAYER.hover;
const TOKEN_LAYER = LAYER.tokens;

async function addCondition(page: Page, text: string) {
	await page.getByLabel('Conditions').fill(text);
	await page.getByRole('button', { name: 'Add condition' }).click();
}

test('a GM sets trackers and conditions, and the whole room can read them', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

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
	await saveEditor(gm.page);

	// The panel reads from room.tokens, so this is the broadcast coming
	// back rather than what was typed. The GM may edit this token, so
	// their slots are boxes; the third is empty and still present, which
	// is the point of showing all three.
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('7');
	await expect(trackerBox(gm.page, 'AC')).toHaveValue('15');
	await expect(trackerBox(gm.page, 'Tracker 3')).toHaveValue('');
	await expect(detailsPanel(gm.page)).toContainText('Prone');

	// The Player owns nothing here, so their boxes are readonly — same
	// element, same aria-label, just one that won't take a step from them.
	await expect(trackerBox(player.page, 'HP')).toHaveValue('7');
	await expect(trackerBox(player.page, 'AC')).toHaveValue('15');
	await expect(detailsPanel(player.page)).toContainText('Prone');

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
});

// Damage is what changes every round, so changing it doesn't cost a
// dialog: the values are edited in the panel itself. This is also the
// one path where a single number changes and the rest of the token has
// to be filled in from what the client holds — token.update clears what
// it isn't told, so a bug here renames the token to nothing.
test('a number typed into the panel reaches the room, and takes nothing else with it', async ({
	table
}) => {
	const gm = table.gm;
	const player = await table.join();

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
	await saveEditor(gm.page);

	// The value is not. Committed on blur rather than on every keystroke,
	// so typing "12" doesn't send a 1 on the way past.
	await trackerBox(gm.page, 'HP').fill('12');
	await trackerBox(gm.page, 'HP').blur();

	await expect(trackerBox(player.page, 'HP')).toHaveValue('12');

	// Everything the panel never asked about survived being sent back with
	// it: the name, and the size that came from a picker in another form.
	// The size is read back through the editor, since the panel no longer
	// spells it out.
	await expect(detailsPanel(gm.page)).toContainText('Goblin');
	await openEditor(gm.page);
	await expect(gm.page.getByRole('button', { name: 'Large (2×2 squares)' })).toHaveAttribute(
		'aria-pressed',
		'true'
	);
	await gm.page.getByRole('button', { name: 'Close' }).click();
	await expect(gm.page.getByRole('button', { name: 'Save changes' })).toBeHidden();

	// Clearing the box empties the slot rather than storing a zero — and
	// zero itself has to still be storable, which is the distinction the
	// whole nullable value exists for.
	await trackerBox(gm.page, 'HP').fill('0');
	await trackerBox(gm.page, 'HP').blur();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('0');

	await trackerBox(gm.page, 'HP').fill('');
	await trackerBox(gm.page, 'HP').blur();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('');

	// And it is really on the server, not just on two canvases.
	await gm.page.reload();
	await expect(gm.page.locator('canvas').first()).toBeVisible();
	await selectToken(gm.page, spawn, 'Goblin');
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('');
});

// The step buttons exist only while a box has focus, which is what lets
// the boxes themselves be big enough to read across a table: three
// permanently-visible pairs would cost the width the numbers just took.
// The subtle half is that a step must survive being clicked twice
// without re-focusing — "the ogre takes 7, then 3" is one interaction,
// and a panel that closed on the first click would make it two.
test('the step control appears on focus, adjusts by what it is told, and survives a second click', async ({
	table
}) => {
	const gm = table.gm;
	const player = await table.join();

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
	await saveEditor(gm.page);

	const decrease = gm.page.getByRole('button', { name: 'Decrease HP' });
	const increase = gm.page.getByRole('button', { name: 'Increase HP' });

	// Nothing is on screen until a box is being used.
	await expect(decrease).toBeHidden();

	await trackerBox(gm.page, 'HP').fill('30');
	await expect(decrease).toBeVisible();

	// An unset "by how much" means one, so the common case costs no
	// typing at all.
	await decrease.click();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('29');

	// Clicking again without touching the box first is the point: focus
	// never left it, so the control is still there.
	await decrease.click();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('28');

	// And the box beside them sets the size of the step.
	await gm.page.getByLabel('Adjust HP by').fill('7');
	await decrease.click();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('21');
	await increase.click();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('28');

	// Focus leaving the control takes it away again, and doesn't undo or
	// re-send anything the buttons already committed. It's the by-box that
	// holds focus by this point rather than the value box — which is
	// itself the proof that clicking the buttons never stole it.
	await gm.page.getByLabel('Adjust HP by').blur();
	await expect(decrease).toBeHidden();
	await expect(trackerBox(player.page, 'HP')).toHaveValue('28');

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
});

// A Player who owns the token gets the boxes too — that's the whole
// point of the per-field rule, and the panel is where they'll use it.
test('a player types their own damage into the panel without opening anything', async ({
	table
}) => {
	const gm = table.gm;
	const player = await table.join();

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
	await expect(detailsPanel(gm.page)).toContainText("Bob's Fighter");
});

// A token with nothing set on it gets no card, because a card on every
// token the pointer crossed would make the map unusable mid-fight.
test('a token with no trackers or conditions shows nothing on hover', async ({ table }) => {
	const gm = table.gm;

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
	await expect(detailsPanel(gm.page)).toContainText('Rock');
	expect(await layerInk(gm.page, HOVER_LAYER)).toBe(0);
});

// The wheel is a step too, so a mouse without a keyboard nearby still
// changes a tracker without opening the by-box — but only once the value
// box has focus, and the by-box itself refuses to scroll below 1, since a
// step of zero would make the buttons (and the wheel) do nothing.
test('the wheel steps a focused tracker, and never drops the step size below 1', async ({
	table
}) => {
	const gm = table.gm;

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill('Troll');
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(gm.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, 'Troll');

	await openEditor(gm.page);
	// Filled and then checked rather than filled and trusted. Under a full
	// run this fill has been seen not to stick — the same shape as the
	// hydration race in planning/backlog/e2e-flakes.md, where Svelte
	// reconciles an input back to its bound value if the fill lands early
	// enough. The symptom surfaces four lines below as a tracker box that
	// never appears, which reads as the wheel being broken.
	const label = gm.page.getByLabel('Tracker 1 label');
	await expect
		.poll(async () => {
			await label.fill('HP');
			return label.inputValue();
		})
		.toBe('HP');
	await saveEditor(gm.page);

	// The relabelled box arriving is the proof the save landed; without
	// this the next line's failure is a timeout with nothing to read.
	const hp = trackerBox(gm.page, 'HP');
	await expect(hp).toBeVisible();
	await hp.fill('30');
	await hp.blur();

	// Hovering an unfocused box is not enough — the browser dispatches the
	// wheel to whatever is under the pointer regardless of focus, so this
	// is the case that actually needs the check rather than proving itself
	// by construction.
	await hp.hover();
	await gm.page.mouse.wheel(0, -100);
	await expect(hp).toHaveValue('30');

	await hp.click();
	await gm.page.mouse.wheel(0, -100);
	await expect(hp).toHaveValue('31');
	await gm.page.mouse.wheel(0, 100);
	await gm.page.mouse.wheel(0, 100);
	await expect(hp).toHaveValue('29');

	// Committing out of the by-box by clicking the value box instead of
	// blurring off the panel entirely — a full blur takes the whole panel
	// away, per the step-control test above, which would take "by" with it
	// before its value could be asserted.
	const by = gm.page.getByLabel('Adjust HP by');
	await by.fill('0');
	await hp.click();
	await expect(by).toHaveValue('1');

	await by.fill('-5');
	await hp.click();
	await expect(by).toHaveValue('1');

	// Scrolling down on the by-box past 1 holds at 1 rather than going
	// negative.
	await by.hover();
	await gm.page.mouse.wheel(0, 100);
	await expect(by).toHaveValue('1');
});

// The per-field permission split. An owner tracks their own damage; the
// name, art, size, owner and visibility stay the GM's, and the form they
// get says so by not being there.
test('a player edits the trackers on their own token and nothing else', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

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
	await saveEditor(player.page);

	// Both sides may edit this token — the GM anything, Bob its numbers —
	// so both render the value as a box rather than as text.
	await expect(trackerBox(player.page, 'HP')).toHaveValue('4');

	// It reached the GM, which is the point of a Player being able to do
	// it at all — the table sees the damage without being told.
	await expect(trackerBox(gm.page, 'HP')).toHaveValue('4');
	await expect(detailsPanel(gm.page)).toContainText('Poisoned');
	// And the name the Player never had a box for came through untouched.
	await expect(detailsPanel(gm.page)).toContainText("Bob's Fighter");
});

// The other side of the same rule: a token nobody owns is nobody's to
// edit or delete but the GM's.
test('a player gets no editor for a token they do not own', async ({ table }) => {
	const gm = table.gm;
	const player = await table.join();

	await gm.page.getByRole('button', { name: 'New token' }).click();
	await gm.page.getByLabel('Name').fill("Bob's Fighter");
	await gm.page.getByLabel('Owner').selectOption({ label: 'Bob' });
	await gm.page.getByRole('button', { name: 'Create token' }).click();
	await expect(gm.page.getByRole('button', { name: 'Create token' })).toBeHidden();

	const box = await canvasBox(player.page);
	const spawn = spawnCentre(box);
	await selectToken(gm.page, spawn, "Bob's Fighter");
	await selectToken(player.page, spawn, "Bob's Fighter");

	// Owning it earns both the editor and the delete — the same rule, since
	// a Player who can conjure their own tokens has to be able to clear
	// them away again.
	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeVisible();
	await expect(player.page.getByRole('button', { name: 'Delete token' })).toBeVisible();

	// Handed back to nobody, both go with it: an unowned token is a piece
	// of the GM's scene that a Player is merely allowed to move.
	await openEditor(gm.page);
	await gm.page.getByLabel('Owner').selectOption({ label: 'Nobody (monster or prop)' });
	await saveEditor(gm.page);

	await expect(player.page.getByRole('button', { name: 'Edit token' })).toBeHidden();
	await expect(player.page.getByRole('button', { name: 'Delete token' })).toBeHidden();
	await expect(detailsPanel(player.page)).toContainText("Bob's Fighter");
});
