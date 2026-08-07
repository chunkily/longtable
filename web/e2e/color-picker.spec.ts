import { expect, test, type Page } from '@playwright/test';
import { openNewSceneDialog, selectToolFamily } from './room';

// The selected colour used to be visible only as pixels, which meant
// nothing could assert it and a screen reader couldn't report it. It is
// now carried on aria-pressed, so both problems are one attribute.

async function createRoomWithScene(page: Page) {
	await page.goto('/');
	await page.getByLabel('Room name').fill('Colours');
	await page.getByLabel('Your name (GM)').fill('Alice');
	await page.getByLabel('GM password').fill('hunter2');
	await page.getByRole('button', { name: 'Create room' }).click();

	await expect(page).toHaveURL(/\/r\/[a-z0-9]+/);
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	// The swatches live on the draw family's contextual strip, so they
	// don't exist until the family is open.
	await selectToolFamily(page, 'Draw');
}

const swatch = (page: Page, label: string) =>
	page.getByRole('button', { name: label, exact: true });

test('exactly one colour reads as selected, and clicking moves it', async ({ page }) => {
	await createRoomWithScene(page);

	await expect(swatch(page, 'Black')).toHaveAttribute('aria-pressed', 'true');
	for (const other of ['Red', 'Green', 'Blue']) {
		await expect(swatch(page, other)).toHaveAttribute('aria-pressed', 'false');
	}

	await swatch(page, 'Green').click();
	await expect(swatch(page, 'Green')).toHaveAttribute('aria-pressed', 'true');
	await expect(swatch(page, 'Black')).toHaveAttribute('aria-pressed', 'false');
});

test('the selected swatch is still clickable and shows a ring outside itself', async ({ page }) => {
	await createRoomWithScene(page);

	// The selected swatch has to still receive pointer events. A trial
	// click runs every actionability check a real click would — including
	// that nothing is covering the target — without changing anything.
	// The previous indicator sat on top of the button, so this is the
	// check that would have caught it.
	const black = swatch(page, 'Black');
	await black.click({ trial: true });

	// The ring sits outside the swatch, so it costs the colour no area.
	const outline = await black.evaluate((el) => {
		const s = getComputedStyle(el);
		return { style: s.outlineStyle, width: s.outlineWidth, offset: s.outlineOffset };
	});
	expect(outline.style).toBe('solid');
	expect(outline.width).toBe('2px');
	expect(outline.offset).toBe('2px');

	// An unselected swatch has no ring at all.
	const red = await swatch(page, 'Red').evaluate((el) => getComputedStyle(el).outlineStyle);
	expect(red).toBe('none');
});
