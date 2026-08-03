import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { fixture } from './fixtures';

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

	await pageA.getByRole('link', { name: 'Assets' }).click();
	await expect(pageA.getByRole('heading', { name: 'Assets' })).toBeVisible();

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
	await expect(pageA.getByText(/Nothing matches/)).toBeVisible();
	await search.fill('');
	await expect(pageA.getByText('Goblin archer')).toBeVisible();

	// Back at the table, both dialogs offer it — different components with
	// their own pickers, both backed by the room's real library.
	await pageA.getByRole('link', { name: 'Back to the table' }).click();
	await pageA.getByRole('button', { name: 'New scene' }).click();
	await expect(pageA.getByRole('button', { name: 'Goblin archer' })).toBeVisible();
	// The picker no longer uploads: adding art is the assets page's job, so
	// there's no longer a path here that can produce an unnamed asset.
	await expect(pageA.getByLabel('Upload an image')).toHaveCount(0);

	await pageA.getByLabel('Name').fill('Tavern');
	await pageA.getByRole('button', { name: 'Goblin archer' }).click();
	await pageA.getByRole('button', { name: 'Create scene' }).click();
	await expect(pageA.getByRole('button', { name: 'New token' })).toBeVisible();

	await pageA.getByRole('button', { name: 'New token' }).click();
	await expect(pageA.getByRole('button', { name: 'Goblin archer' })).toBeVisible();
	await pageA.getByRole('button', { name: 'Close' }).click();

	// A second, unrelated room must not see it at all — the scoping the hub
	// enforces server-side, reflected in the picker.
	const roomB = await browser.newContext();
	const pageB = await roomB.newPage();
	await createRoomAsGM(pageB, 'Library B');
	await pageB.getByRole('button', { name: 'New scene' }).click();
	await expect(pageB.getByText('Nothing in the library yet')).toBeVisible();
	await expect(pageB.getByRole('button', { name: 'Goblin archer' })).toHaveCount(0);

	await roomA.close();
	await roomB.close();
});

test('aligning a map bakes the offset into the image and pre-fills the scene grid size', async ({
	page
}) => {
	await createRoomAsGM(page, 'Grid Alignment');
	await page.getByRole('link', { name: 'Assets' }).click();

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
	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByRole('button', { name: 'Swamp' }).click();
	await expect(page.getByLabel('Grid size (px)')).toHaveValue('8');
	// Dimensions still come from the image itself — now the padded one.
	await expect(page.getByLabel('Width (px)')).toHaveValue('15');
});

test('a library entry can be renamed afterwards, and identical content is never duplicated', async ({
	page
}) => {
	await createRoomAsGM(page, 'Library Editing');
	await page.getByRole('link', { name: 'Assets' }).click();

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
	await expect(page.getByText('Ruined keep')).toHaveCount(1);
	// The earlier name survives, because an add that didn't set out to
	// rename anything shouldn't.
	await expect(page.getByText('map-again')).toHaveCount(0);
});
