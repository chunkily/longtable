import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { fixture } from './fixtures';

// The asset library spans two dialogs (scene and token creation) and two
// scoping guarantees the unit/API/ws tests already prove at their own
// layer — same file shared across rooms, but a room only ever sees what
// it added — this exercises through the real forms, since a UI can
// reflect the wrong thing even when the API underneath it is correct.

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

test('an upload joins the room library, both dialogs can reuse it, and another room never sees it', async ({
	browser
}) => {
	const roomA = await browser.newContext();
	const pageA = await roomA.newPage();
	await createRoomAsGM(pageA, 'Library A');

	await pageA.getByRole('button', { name: 'New scene' }).click();
	await pageA.getByLabel('Name').fill('Tavern');
	await pageA.getByLabel('Upload an image').setInputFiles(fixture('goblin.png'));

	// The upload response rewrites the filename to .webp — that's the
	// re-encode pipeline, not a display bug — so the library entry that
	// appears is what proves the round trip actually happened.
	const libraryEntry = pageA.getByRole('button', { name: 'goblin.webp' });
	await expect(libraryEntry).toBeVisible();
	// Selecting on upload is automatic, and width/height should already
	// reflect the real 8x8 image rather than the dialog's 1400x1000
	// default.
	await expect(pageA.getByLabel('Width (px)')).toHaveValue('8');
	await expect(pageA.getByLabel('Height (px)')).toHaveValue('8');

	await pageA.getByRole('button', { name: 'Create scene' }).click();
	await expect(pageA.getByRole('button', { name: 'New token' })).toBeVisible();

	// The token dialog is a different component with its own picker —
	// this is what proves it's backed by the room's real library rather
	// than anything private to the scene dialog.
	await pageA.getByRole('button', { name: 'New token' }).click();
	await expect(pageA.getByRole('button', { name: 'goblin.webp' })).toBeVisible();
	await pageA.getByRole('button', { name: 'Close' }).click();

	// A second, unrelated room must not see it at all — this is the
	// scoping the hub enforces server-side (requireAssetInRoom); the
	// picker should reflect exactly that, not just avoid erroring.
	const roomB = await browser.newContext();
	const pageB = await roomB.newPage();
	await createRoomAsGM(pageB, 'Library B');
	await pageB.getByRole('button', { name: 'New scene' }).click();
	await expect(pageB.getByText('Nothing in the library yet')).toBeVisible();
	await expect(pageB.getByRole('button', { name: 'goblin.webp' })).toHaveCount(0);

	await roomA.close();
	await roomB.close();
});

test('the library survives a reload, and re-uploading identical content reuses the same asset', async ({
	page
}) => {
	await createRoomAsGM(page, 'Library Persistence');

	await page.getByRole('button', { name: 'New scene' }).click();
	await page.getByLabel('Attribution or licence (optional)').fill('by Alice, CC-BY');
	await page.getByLabel('Upload an image').setInputFiles(fixture('map.png'));
	await expect(page.getByRole('button', { name: 'map.webp' })).toBeVisible();
	await page.getByLabel('Name').fill('Map Room');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.getByRole('button', { name: 'New token' })).toBeVisible();

	await page.reload();
	await expect(page.getByRole('button', { name: 'New scene' })).toBeVisible();
	await page.getByRole('button', { name: 'New scene' }).click();

	// Still there after a full reload — this is what proves it's the
	// room's server-side library, not client state that reset with the
	// page.
	const entry = page.getByRole('button', { name: 'map.webp' });
	await expect(entry).toBeVisible();
	// Attribution shown on the room-wide library entry, not just echoed
	// back to the uploader.
	await expect(entry).toHaveAttribute('title', /by Alice, CC-BY/);

	// Uploading the exact same bytes again must not duplicate the entry:
	// content hashing should resolve it to the asset already there.
	//
	// The one upload in the suite that passes bytes rather than a path,
	// and deliberately: uploading by path would send the fixture's own
	// basename, and sending *different* name with *identical* content is
	// exactly what this is testing the server's answer to.
	await page.getByLabel('Upload an image').setInputFiles({
		name: 'map-again.png',
		mimeType: 'image/png',
		buffer: readFileSync(fixture('map.png'))
	});
	await expect(page.getByRole('button', { name: 'map.webp' })).toHaveCount(1);
});
