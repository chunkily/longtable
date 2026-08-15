import { expect, test } from './fixtures/table';
import { openSeatPicker } from './fixtures/room';

// An identity colour is only worth anything on somebody else's screen,
// which is why these run two browsers: the point is that Bob's name is
// Bob's colour where *Alice* is looking.

/** The hex for a palette key, from web/src/lib/identity-color.ts. */
const HEX = { teal: 'rgb(20, 184, 166)', pink: 'rgb(236, 72, 153)' };

test('a name in chat wears the colour its owner picked', async ({ table }) => {
	const gm = table.gm;
	const bob = await table.join('Bob', 'Teal');

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
	await table.join('Bob', 'Teal');

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
	await table.join('Bob', 'Pink');
	const carol = await table.join('Carol', 'Pink');

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
	const bob = await table.join('Bob', 'Teal');
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
