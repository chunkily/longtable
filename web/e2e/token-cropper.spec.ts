import { expect, test } from './fixtures/table';
import type { Page } from '@playwright/test';
import { fixture } from './fixtures/images';
import { openAssetsPage } from './fixtures/room';

// Framing token art at upload. portrait.png is 40x120 in three bands, red
// over green over blue, so the square that was kept can be read straight
// off the stored image: the server cuts it during the re-encode, and what
// the library serves afterwards is the proof.

/**
 * The stored image behind a library tile, measured in the page: its size,
 * and which colour dominates its middle. Dominant rather than exact,
 * because the server's WebP encode is lossy.
 */
async function storedArt(page: Page, name: string) {
	const src = await page.getByRole('img', { name, exact: true }).getAttribute('src');
	return page.evaluate(async (url) => {
		const img = new Image();
		img.src = url!;
		await img.decode();
		const canvas = document.createElement('canvas');
		canvas.width = img.naturalWidth;
		canvas.height = img.naturalHeight;
		const ctx = canvas.getContext('2d')!;
		ctx.drawImage(img, 0, 0);
		const [r, g, b] = ctx.getImageData(img.naturalWidth / 2, img.naturalHeight / 2, 1, 1).data;
		const middle = r > g && r > b ? 'red' : g > r && g > b ? 'green' : 'blue';
		return { width: img.naturalWidth, height: img.naturalHeight, middle };
	}, src);
}

async function stage(page: Page, file: string, name: string) {
	await page.getByLabel('Choose images to add').setInputFiles(fixture(file));
	await page.getByLabel('Name').fill(name);
}

test('tall token art is framed from the top, and the frame can be moved', async ({ table }) => {
	const gm = table.gm.page;
	await gm.keyboard.press('Escape');
	await openAssetsPage(gm);

	// Staged under Tokens, a portrait gets the cropper — already framed, so
	// accepting the default is one click.
	await stage(gm, 'portrait.png', 'Top');
	await expect(gm.getByText('Frame the token')).toBeVisible();
	await gm.getByRole('button', { name: 'Add to library' }).click();
	await expect(gm.getByRole('img', { name: 'Top', exact: true })).toBeVisible();
	expect(await storedArt(gm, 'Top')).toEqual({ width: 40, height: 40, middle: 'red' });

	// The same file again, with the frame moved down a band. A twentieth of
	// a 40px frame is 2px a press, so twenty presses is one band.
	await stage(gm, 'portrait.png', 'Middle');
	await gm.getByRole('button', { name: 'Move the crop' }).focus();
	for (let i = 0; i < 20; i++) await gm.keyboard.press('ArrowDown');
	await gm.getByRole('button', { name: 'Add to library' }).click();
	await expect(gm.getByRole('img', { name: 'Middle', exact: true })).toBeVisible();
	expect(await storedArt(gm, 'Middle')).toEqual({ width: 40, height: 40, middle: 'green' });
});

// Square art has nothing to choose, so no cropper; and maps are aligned
// rather than framed.
test('square art and maps are not offered the cropper', async ({ table }) => {
	const gm = table.gm.page;
	await gm.keyboard.press('Escape');
	await openAssetsPage(gm);

	await stage(gm, 'goblin.png', 'Square');
	await expect(gm.getByRole('button', { name: 'Add to library' })).toBeEnabled();
	await expect(gm.getByText('Frame the token')).toHaveCount(0);
	await gm.getByRole('button', { name: 'Discard' }).click();

	await gm.getByRole('tab', { name: /^Maps/ }).click();
	await stage(gm, 'portrait.png', 'Tall map');
	await expect(gm.getByText('Frame the token')).toHaveCount(0);
	await expect(gm.getByRole('button', { name: 'Move the crop' })).toHaveCount(0);
});
