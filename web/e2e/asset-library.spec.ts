import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { fixture } from './fixtures';
import { openAssetsPage, openNewSceneDialog } from './room';

// Assets are prepared on their own page and only picked in the room, so
// this covers the seam between the two: what the assets page stores has
// to be what the scene and token dialogs then offer. The unit, API and ws
// tests prove the scoping and the re-encode at their own layers — a UI
// can reflect the wrong thing even when the API underneath it is right.

// Uploads come from e2e/fixtures — real encoded images in colours no
// other fixture uses, which their README explains the reasons for.

async function createRoomAsGM(page: Page, roomName: string) {
	await page.goto('/');
	await page.getByLabel('Room name').fill(roomName);
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();
	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
}

test('an asset added on the assets page is named, searchable, and offered in both dialogs', async ({
	browser
}) => {
	const roomA = await browser.newContext();
	const pageA = await roomA.newPage();
	await createRoomAsGM(pageA, 'Library A');

	await openAssetsPage(pageA);

	// The name defaults to the filename minus its extension, which is the
	// point of it being a real editable field rather than a label derived
	// at render time.
	await pageA.getByLabel('Choose images to add').setInputFiles(fixture('goblin.png'));
	await expect(pageA.getByLabel('Name')).toHaveValue('goblin');
	await pageA.getByLabel('Name').fill('Goblin archer');
	await pageA.getByLabel('Attribution or licence (optional)').fill('by Alice, CC-BY');
	await pageA.getByRole('button', { name: 'Add to library' }).click();

	// Once added the staging card is gone, and the entry is in the library
	// under the name it was given — not under goblin.webp, which is only
	// what the re-encode called the file.
	await expect(pageA.getByRole('button', { name: 'Add to library' })).toHaveCount(0);
	await expect(pageA.getByText('Goblin archer')).toBeVisible();

	// A map as well, since the two dialogs now ask for different halves of
	// the library and one asset can't prove both. The tab is picked first
	// and decides what the upload is — there is no per-file control to
	// forget about afterwards.
	await pageA.getByRole('tab', { name: /^Maps/ }).click();
	// The upload card follows the tab, so what's about to happen is stated
	// before a file is chosen rather than after.
	await expect(pageA.getByText('Add maps')).toBeVisible();
	await expect(pageA.getByRole('button', { name: 'Choose maps' })).toBeVisible();
	await pageA.getByLabel('Choose images to add').setInputFiles(fixture('swamp.png'));
	await pageA.getByLabel('Name').fill('Swamp road');
	await pageA.getByRole('button', { name: 'Add to library' }).click();
	await expect(pageA.getByText('Swamp road')).toBeVisible();

	await pageA.getByRole('tab', { name: 'Tokens 1' }).click();
	await expect(pageA.getByText('Goblin archer')).toBeVisible();

	// Search narrows live, over the name and the credit both.
	const search = pageA.getByLabel('Search the library');
	await search.fill('archer');
	await expect(pageA.getByText('Goblin archer')).toBeVisible();
	await search.fill('cc-by');
	await expect(pageA.getByText('Goblin archer')).toBeVisible();

	// A query with no matches says so, distinctly from an empty library —
	// "you have nothing" and "none of your things are this" need different
	// next steps.
	await search.fill('tavern');
	await expect(pageA.getByText(/Nothing here matches/)).toBeVisible();

	// And when the thing being searched for is sitting in the other tab,
	// the empty state points at it rather than reading as "this room
	// doesn't have that", which is the one way a split library can lose
	// something.
	await search.fill('swamp');
	await pageA.getByRole('button', { name: /match(es)? in Maps/ }).click();
	await expect(pageA.getByText('Swamp road')).toBeVisible();

	await search.fill('');
	await pageA.getByRole('tab', { name: 'Tokens 1' }).click();
	await expect(pageA.getByText('Goblin archer')).toBeVisible();

	// Back at the table, each dialog offers the half of the library it's
	// asking for — different components with their own pickers, both
	// backed by the room's real library.
	await pageA.getByRole('link', { name: 'Back to the table' }).click();
	await openNewSceneDialog(pageA);
	await expect(pageA.getByRole('button', { name: 'Swamp road' })).toBeVisible();
	await expect(pageA.getByRole('button', { name: 'Goblin archer' })).toHaveCount(0);
	// The picker no longer uploads: adding art is the assets page's job, so
	// there's no longer a path here that can produce an unnamed asset.
	await expect(pageA.getByLabel('Upload an image')).toHaveCount(0);

	await pageA.getByLabel('Name').fill('Tavern');
	await pageA.getByRole('button', { name: 'Swamp road' }).click();
	await pageA.getByRole('button', { name: 'Create scene' }).click();
	await expect(pageA.getByRole('button', { name: 'New token' })).toBeVisible();

	await pageA.getByRole('button', { name: 'New token' }).click();
	await expect(pageA.getByRole('button', { name: 'Goblin archer' })).toBeVisible();
	// A picker shows the kind it is asking for and nothing else — no tabs,
	// and the other half of the library isn't in the grid to be found. The
	// way to art filed under the wrong kind is the assets page, which the
	// link below carries the right tab into.
	await expect(pageA.getByRole('tab')).toHaveCount(0);
	await expect(pageA.getByRole('button', { name: 'Swamp road' })).toHaveCount(0);
	await pageA.getByRole('button', { name: 'Close' }).click();

	// A second, unrelated room must not see it at all — the scoping the hub
	// enforces server-side, reflected in the picker.
	const roomB = await browser.newContext();
	const pageB = await roomB.newPage();
	await createRoomAsGM(pageB, 'Library B');
	await openNewSceneDialog(pageB);
	await expect(pageB.getByText('Nothing in the library yet')).toBeVisible();
	await expect(pageB.getByRole('button', { name: 'Swamp road' })).toHaveCount(0);

	await roomA.close();
	await roomB.close();
});

test('aligning a map bakes the offset into the image and pre-fills the scene grid size', async ({
	page
}) => {
	await createRoomAsGM(page, 'Grid Alignment');
	await openAssetsPage(page);

	// Alignment belongs to maps, so it's the Maps tab that offers it — it
	// used to be the other way round, with aligning the thing that made
	// something a map.
	await page.getByRole('tab', { name: /^Maps/ }).click();
	await page.getByLabel('Choose images to add').setInputFiles(fixture('swamp.png'));
	await page.getByLabel('Name').fill('Swamp');
	await page.getByRole('button', { name: 'Align to grid' }).click();

	// 8px squares starting 1px in. The fixtures are 8x8 and the server
	// floors a square at 8px — anything smaller is a measurement nobody
	// meant — so this is the one grid these images can carry: padding 7px
	// on each axis takes it to 15x15, where the art's own lines land on
	// multiples of 8.
	await page.getByLabel('Square size (px)').fill('8');
	await page.getByLabel('Offset across').fill('1');
	await page.getByLabel('Offset down').fill('1');
	await page.getByRole('button', { name: 'Add to library' }).click();

	await expect(page.getByText('Swamp')).toBeVisible();
	// The measured square size rides along with the asset, which is what
	// makes aligning worth doing — a scene created at the wrong grid size
	// undoes the alignment just as thoroughly as a wrong offset would.
	await expect(page.getByText('8px grid')).toBeVisible();

	// The stored image is the padded one. Asking the browser for its real
	// dimensions is the only way to see the offset went into the pixels
	// rather than into a column somewhere.
	const stored = await page.evaluate(async () => {
		const slug = location.pathname.split('/')[2];
		const session = JSON.parse(localStorage.getItem(`longtable:session:${slug}`)!);
		const library = await (
			await fetch(`/api/rooms/${slug}/assets`, {
				headers: { Authorization: `Bearer ${session.sessionToken}` }
			})
		).json();
		return new Promise<[number, number]>((resolve) => {
			const img = new Image();
			img.onload = () => resolve([img.naturalWidth, img.naturalHeight]);
			img.src = `/api/assets/${library[0].id}`;
		});
	});
	expect(stored).toEqual([15, 15]);

	await page.getByRole('link', { name: 'Back to the table' }).click();
	await openNewSceneDialog(page);
	await page.getByRole('button', { name: 'Swamp' }).click();
	await expect(page.getByLabel('Grid size (px)')).toHaveValue('8');
	// Dimensions still come from the image itself — now the padded one.
	await expect(page.getByLabel('Width (px)')).toHaveValue('15');
});

// Adding the same bytes twice makes one entry, not two — and the second
// add renames it, because the page always supplies a name whether or not
// anyone typed one. That last part is deliberate rather than a bug worked
// around: an upload is a statement about what this image is called now.
test('a library entry can be renamed, and re-adding the same bytes renames rather than duplicates', async ({
	page
}) => {
	await createRoomAsGM(page, 'Library Editing');
	await openAssetsPage(page);

	await page.getByLabel('Choose images to add').setInputFiles(fixture('map.png'));
	await page.getByRole('button', { name: 'Add to library' }).click();
	await expect(page.getByText('map', { exact: true })).toBeVisible();

	// Names default from filenames, so a first pass is full of things
	// nobody would search for. Renaming has to work without re-uploading.
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.getByLabel('Name').fill('Ruined keep');
	await page.getByLabel('Attribution or licence').fill('by Bob');
	await page.getByRole('button', { name: 'Save' }).click();
	await expect(page.getByText('Ruined keep')).toBeVisible();

	// Survives a reload: it's the room's server-side library, not client
	// state that reset with the page.
	await page.reload();
	await expect(page.getByText('Ruined keep')).toBeVisible();
	await expect(page.getByText('by Bob')).toBeVisible();

	// Adding the exact same bytes again resolves to the asset already
	// there, and must not produce a second entry.
	//
	// The one upload in the suite that passes bytes rather than a path,
	// and deliberately: uploading by path would send the fixture's own
	// basename, and sending a *different* name with *identical* content is
	// exactly what this is testing the server's answer to.
	await page.getByLabel('Choose images to add').setInputFiles({
		name: 'map-again.png',
		mimeType: 'image/png',
		buffer: readFileSync(fixture('map.png'))
	});
	await page.getByRole('button', { name: 'Add to library' }).click();

	// Re-adding *does* rename, and the positive assertion goes first on
	// purpose. `map-again` arriving is the only proof the library has
	// finished refreshing; asserting the absence of something first would
	// pass against the list as it stood before the upload landed, which is
	// exactly how this test used to fail about one run in three.
	await expect(page.getByText('map-again')).toHaveCount(1);
	await expect(page.getByText('Ruined keep')).toHaveCount(0);

	// But it is still one entry, not two: identical bytes resolve to the
	// asset already there, and the room already having it makes the add an
	// update rather than an insert.
	await expect(page.getByRole('button', { name: /^Remove / })).toHaveCount(1);

	// The credit survives, which is the rule the name is obeying too:
	// `AddAssetToRoom` overwrites a field only when the upload supplied
	// one. This page always supplies a name — it defaults the box to the
	// filename — and only supplies a credit if someone typed it.
	await expect(page.getByText('by Bob')).toBeVisible();
});

test('the library keeps tokens and maps apart, shows token art whole, and can be corrected', async ({
	page
}) => {
	await createRoomAsGM(page, 'Two Tabs');
	await openAssetsPage(page);

	await page.getByLabel('Choose images to add').setInputFiles(fixture('goblin.png'));
	await page.getByLabel('Name').fill('Goblin archer');
	await page.getByRole('button', { name: 'Add to library' }).click();
	await expect(page.getByText('Goblin archer')).toBeVisible();

	await page.getByRole('tab', { name: /^Maps/ }).click();
	await page.getByLabel('Choose images to add').setInputFiles(fixture('map.png'));
	await page.getByLabel('Name').fill('Ruined keep');
	await page.getByRole('button', { name: 'Add to library' }).click();

	// The Maps tab is open because that's what was just added — a library
	// that stayed on the other tab would look like the upload did nothing —
	// and it holds the map alone.
	await expect(page.getByText('Ruined keep')).toBeVisible();
	await expect(page.getByText('Goblin archer')).toHaveCount(0);

	// A map keeps the wide crop: it's never square and never legible at
	// thumbnail size anyway, so the tile buys screen space instead.
	const mapArt = (await page.getByRole('img', { name: 'Ruined keep' }).boundingBox())!;
	expect(mapArt.height).toBeLessThan(mapArt.width);

	await page.getByRole('tab', { name: 'Tokens 1' }).click();
	await expect(page.getByText('Ruined keep')).toHaveCount(0);
	// Token art is square and uncropped, because a token *is* square on the
	// table and a crop hides the thing being chosen. Measured rather than
	// asserted on a class name, since what matters is the box on screen.
	const tokenArt = (await page.getByRole('img', { name: 'Goblin archer' }).boundingBox())!;
	expect(Math.abs(tokenArt.width - tokenArt.height)).toBeLessThan(1);

	// Which kind a picture is filed under is this room's opinion of it, not
	// a fact about the pixels, so it can be corrected without re-uploading.
	// This is also the only repair for whatever the migration guessed about
	// a library added before the split existed.
	await page.getByRole('button', { name: 'Edit' }).click();
	await page.getByRole('button', { name: 'Maps', exact: true }).click();
	await page.getByRole('button', { name: 'Save' }).click();

	await expect(page.getByRole('tab', { name: 'Maps 2' })).toBeVisible();
	await expect(page.getByText('Goblin archer')).toBeVisible();
	await page.getByRole('tab', { name: 'Tokens 0' }).click();
	await expect(page.getByText('Nothing filed as token art yet')).toBeVisible();

	// And the split survives a reload, because it's a column on the room's
	// copy of the asset rather than something the page worked out.
	await page.reload();
	await expect(page.getByRole('tab', { name: 'Maps 2' })).toBeVisible();
});

// A picker sends you to the assets page when the library hasn't got what
// you need — and it has to send you to the half that adds it. Landing on
// Tokens after following a link out of the scene dialog is how a map
// gets filed as token art, which is the thing the tabs exist to prevent.
test('a picker links to the half of the assets page it is asking for', async ({ page }) => {
	await createRoomAsGM(page, 'Picker Link');

	await openNewSceneDialog(page);
	const toMaps = page.getByRole('link', { name: 'Add maps' });
	await expect(toMaps).toHaveAttribute('href', /\?kind=map$/);

	// Following it opens the page already on Maps, ready to add one. The
	// link opens a new tab in the app, so the spec walks the href itself
	// rather than juggling a popup.
	await page.goto((await toMaps.getAttribute('href'))!);
	await expect(page.getByRole('tab', { name: /^Maps/ })).toHaveAttribute('aria-selected', 'true');
	await expect(page.getByRole('button', { name: 'Choose maps' })).toBeVisible();

	// The token side asks for the other half, from the same component.
	await page.getByRole('link', { name: 'Back to the table' }).click();
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Tavern');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await page.getByRole('button', { name: 'New token' }).click();

	const toTokens = page.getByRole('link', { name: 'Add token art' });
	await expect(toTokens).toHaveAttribute('href', /\?kind=token$/);
	await page.goto((await toTokens.getAttribute('href'))!);
	await expect(page.getByRole('tab', { name: /^Tokens/ })).toHaveAttribute('aria-selected', 'true');
});

test('a staged file whose shape disagrees with the open tab says so, and can be moved', async ({
	page
}) => {
	await createRoomAsGM(page, 'Shape Guess');
	await openAssetsPage(page);

	// Staged on the Tokens tab, so that's what it would be filed as — but
	// it's 40x12, which is not a shape token art comes in.
	await page.getByLabel('Choose images to add').setInputFiles(fixture('wide-map.png'));
	await expect(page.getByText('Adding as token art')).toBeVisible();
	await expect(page.getByText('40×12 is shaped more like a map')).toBeVisible();

	// The guess only ever asks. Taking it up is one click, and it carries
	// the alignment step over with it, since that belongs to maps.
	await page.getByRole('button', { name: 'File it as a map' }).click();
	await expect(page.getByText('Adding as a map')).toBeVisible();
	await expect(page.getByRole('button', { name: 'Align to grid' })).toBeVisible();

	await page.getByLabel('Name').fill('Long hall');
	await page.getByRole('button', { name: 'Add to library' }).click();
	await expect(page.getByRole('tab', { name: 'Maps 1' })).toBeVisible();
	await expect(page.getByText('Long hall')).toBeVisible();

	// A square image on the Tokens tab is what the tab said it would be,
	// and gets asked nothing.
	await page.getByRole('tab', { name: /^Tokens/ }).click();
	await page.getByLabel('Choose images to add').setInputFiles(fixture('goblin.png'));
	await expect(page.getByText('Adding as token art')).toBeVisible();
	await expect(page.getByText(/shaped more like/)).toHaveCount(0);
});

test('an asset can be taken off the room shelf without deleting the picture', async ({ page }) => {
	await createRoomAsGM(page, 'Asset Removal');
	await openAssetsPage(page);

	await page.getByLabel('Choose images to add').setInputFiles(fixture('goblin.png'));
	await page.getByLabel('Name').fill('Goblin archer');
	await page.getByRole('button', { name: 'Add to library' }).click();
	await expect(page.getByText('Goblin archer')).toBeVisible();

	// The image's own URL, checked afterwards: removal is about this
	// room's shelf, and the file is content-addressed and shared with
	// every other room that has it.
	const assetSrc = await page.getByRole('img', { name: 'Goblin archer' }).getAttribute('src');

	// Two clicks, so a stray one can't empty a library.
	await page.getByRole('button', { name: 'Remove Goblin archer' }).click();
	await expect(page.getByRole('button', { name: 'Remove Goblin archer' })).toHaveCount(0);
	await page.getByRole('button', { name: 'Confirm removing Goblin archer' }).click();

	await expect(page.getByText('Goblin archer')).toHaveCount(0);
	await expect(page.getByRole('tab', { name: 'Tokens 0' })).toBeVisible();

	// Gone from the server, not just from this page's state.
	await page.reload();
	await expect(page.getByText('Goblin archer')).toHaveCount(0);

	// The picture is still served, which is what keeps a scene or token
	// that was already using it from turning into a broken image.
	const stillServed = await page.evaluate(async (src) => (await fetch(src!)).status, assetSrc);
	expect(stillServed).toBe(200);

	// And it's gone from the pickers in the room too — the same library,
	// fetched fresh.
	await page.getByRole('link', { name: 'Back to the table' }).click();
	await openNewSceneDialog(page);
	await expect(page.getByText('Nothing in the library yet')).toBeVisible();
});
