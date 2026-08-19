import { expect, test } from './fixtures/table';
import { openSeatPicker, openSeatsDialog } from './fixtures/room';

// An identity colour is only worth anything on somebody else's screen,
// which is why these run two browsers: the point is that Bob's name is
// Bob's colour where *Alice* is looking.

/** The hex for a palette key, from web/src/lib/identity-color.ts. */
const HEX = { teal: 'rgb(45, 212, 191)', pink: 'rgb(236, 72, 153)' };

/** GM_IDENTITY_COLOR.light, which is what these run under. */
const GM_BLACK = 'rgb(0, 0, 0)';

test('a name in chat wears the colour its owner picked', async ({ table }) => {
	const gm = table.gm;
	const bob = await table.join('Bob', 'Lagoon Teal');

	await bob.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('over here');
	await bob.page.getByRole('button', { name: 'Send' }).click();

	// Read on the GM's screen, off the roster that arrived over the
	// socket rather than out of the form Bob filled in.
	const name = gm.page
		.getByRole('list')
		.first()
		.locator('li')
		.filter({ hasText: 'over here' })
		.locator('strong');
	await expect(name).toHaveText('Bob:');
	await expect(name).toHaveCSS('color', HEX.teal);
});

// The "before choosing, I can see what everyone else has" criterion. The
// seat picker is the only screen that exists at that moment, so it is
// the only place this can be answered.
test('the seat picker shows the colours already at the table', async ({ table, browser }) => {
	await table.join('Bob', 'Lagoon Teal');

	// A third device, no session, looking at the room cold — its own
	// context rather than a tab, since a tab shares localStorage and
	// would arrive already seated.
	const context = await browser.newContext();
	const visitor = await context.newPage();
	await visitor.goto(`/r/${table.slug}`);
	await openSeatPicker(visitor);

	const bobsSeat = visitor.getByRole('button', { name: "Take Bob's seat" });
	await expect(bobsSeat).toBeVisible();
	await expect(bobsSeat.locator('span[style*="background-color"]')).toHaveCSS(
		'background-color',
		HEX.teal
	);

	await context.close();
});

// Two people may be the same colour: the swatches say which are taken so
// a clash can be a choice, and nothing refuses one.
test('two people may take the same colour', async ({ table }) => {
	await table.join('Bob', 'Rose Pink');
	const carol = await table.join('Carol', 'Rose Pink');

	await carol.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('same as Bob');
	await carol.page.getByRole('button', { name: 'Send' }).click();

	const name = carol.page
		.getByRole('list')
		.first()
		.locator('li')
		.filter({ hasText: 'same as Bob' })
		.locator('strong');
	await expect(name).toHaveCSS('color', HEX.pink);
});

// The colour belongs to the seat, so a different device taking that seat
// arrives wearing it — the criterion that was unsatisfiable before seats
// and sessions were separated.
test('a colour comes back with the seat on a new device', async ({ table, browser }) => {
	const bob = await table.join('Bob', 'Lagoon Teal');
	await bob.context.close();

	const context = await browser.newContext();
	const newDevice = await context.newPage();
	await newDevice.goto(`/r/${table.slug}`);
	await openSeatPicker(newDevice);
	await newDevice.getByRole('button', { name: "Take Bob's seat" }).click();
	await expect(newDevice.getByRole('button', { name: "Take Bob's seat" })).toBeHidden();

	await newDevice.getByPlaceholder('Say something, or /roll 2d6+3').fill('back again');
	await newDevice.getByRole('button', { name: 'Send' }).click();

	const name = newDevice
		.getByRole('list')
		.first()
		.locator('li')
		.filter({ hasText: 'back again' })
		.locator('strong');
	await expect(name).toHaveCSS('color', HEX.teal);

	await context.close();
});

// Changing a colour after the fact. The swatch in the rail is the
// control; the assertion is on the *other* browser, since a colour only
// means anything on somebody else's screen.
test('changing your colour recolours your name for everyone', async ({ table }) => {
	const gm = table.gm;
	const bob = await table.join('Bob', 'Lagoon Teal');

	await bob.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('watch this');
	await bob.page.getByRole('button', { name: 'Send' }).click();

	const nameOnGMsScreen = gm.page
		.getByRole('list')
		.first()
		.locator('li')
		.filter({ hasText: 'watch this' })
		.locator('strong');
	await expect(nameOnGMsScreen).toHaveCSS('color', HEX.teal);

	// The swatch in the rail is still the way in, but it opens the Seats
	// dialog now rather than the palette: the colours are picked beside
	// everyone else's, which is the list you want on screen while
	// choosing one. The palette itself sits open at the foot of that
	// dialog — see the note in seats-dialog.svelte for why it is not on a
	// popover.
	await bob.page.getByRole('button', { name: 'Your colour' }).first().click();
	const seats = bob.page.getByRole('dialog');
	await expect(seats.getByRole('heading', { name: 'Seats' })).toBeVisible();

	// Sixteen swatches have to fit the dialog they sit in, which is 448px
	// at its widest — a palette that reflowed to one long row would run
	// off the side of it.
	const palette = seats.getByRole('radiogroup', { name: 'Your colour' });
	const box = await palette.boundingBox();
	expect(box!.width).toBeLessThan(340);

	// Clicked through the dialog, so this fails if the palette is ever
	// covered again rather than merely present: a locator that only
	// proves a thing is in the accessibility tree proves nothing about
	// whether a person could have reached it.
	await palette.getByRole('radio', { name: 'Rose Pink' }).click();

	// The message Bob already sent recolours too: the colour is looked up
	// per render from the roster, so it is a property of who they are
	// rather than of what they said.
	await expect(nameOnGMsScreen).toHaveCSS('color', HEX.pink);

	// And it survives a reload, which is the half that proves it was
	// stored rather than only broadcast.
	await gm.page.reload();
	await expect(nameOnGMsScreen).toHaveCSS('color', HEX.pink);
});

// The GM's colour is fixed black rather than picked, so the two things
// worth proving are that it lands on somebody else's screen and that
// nothing anywhere offers to change it.
test("the GM's name is black, and nothing offers them a choice", async ({ table }) => {
	const gm = table.gm;
	const bob = await table.join('Bob', 'Lagoon Teal');

	await gm.page.getByPlaceholder('Say something, or /roll 2d6+3').fill('roll initiative');
	await gm.page.getByRole('button', { name: 'Send' }).click();

	// Read on Bob's screen, off the roster rather than out of any form.
	const name = bob.page
		.getByRole('list')
		.first()
		.locator('li')
		.filter({ hasText: 'roll initiative' })
		.locator('strong');
	await expect(name).toHaveText('Alice:');
	await expect(name).toHaveCSS('color', GM_BLACK);

	// The rail's swatch is a control for a Player and a plain dot for the
	// GM: there is nothing at the other end of the click for them.
	await expect(bob.page.getByRole('button', { name: 'Your colour' })).toBeVisible();
	await expect(gm.page.getByRole('button', { name: 'Your colour' })).toBeHidden();

	// And the palette is not at the foot of their Seats dialog either.
	await openSeatsDialog(gm.page);
	await expect(gm.page.getByRole('radiogroup', { name: 'Your colour' })).toBeHidden();
});

// The create-room form used to ask the GM for a colour before they had a
// room to be a GM of. It asks for a name and a password now.
test('creating a room does not ask the GM for a colour', async ({ page }) => {
	await page.goto('/');
	await page.waitForLoadState('networkidle');
	await page.getByRole('button', { name: 'Create a room' }).click();
	await expect(page.getByLabel('Room name')).toBeVisible();

	await expect(page.getByRole('radiogroup', { name: 'Your colour' })).toBeHidden();
});
