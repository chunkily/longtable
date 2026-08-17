import { expect, test, type Page } from '@playwright/test';
import { createRoom, openNewSceneDialog, selectTool, selectToolFamily } from './fixtures/room';

// The selected colour used to be visible only as pixels, which meant
// nothing could assert it and a screen reader couldn't report it. It is
// now carried on aria-pressed in the panel and on the button's own name,
// so both problems are two attributes.

async function createRoomWithScene(page: Page) {
	await createRoom(page, 'Colours');
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	// The colour button lives on the draw family's contextual strip, so it
	// doesn't exist until the family is open.
	await selectToolFamily(page, 'Draw');
}

/** The strip's one colour button, which names the colour it is on. */
const colorTrigger = (page: Page) => page.getByRole('button', { name: 'Stroke colour:' });

const swatch = (page: Page, label: string) =>
	page.getByRole('button', { name: label, exact: true });

/**
 * Picks a colour through the popup, and waits for the popup to be gone.
 *
 * The panel is portalled over the map, so a gesture issued while it is
 * still closing lands on the panel rather than the canvas — silently,
 * since raw mouse coordinates make no actionability checks.
 */
async function pickColor(page: Page, label: string) {
	await colorTrigger(page).click();
	await swatch(page, label).click();
	await expect(swatch(page, label)).toBeHidden();
}

// The eraser takes whole strokes rather than making one, so a colour
// there would sit inert — the same reason the width button drops out,
// and they now come and go together.
test('the colour picker is offered to every shape but the eraser', async ({ page }) => {
	await createRoomWithScene(page);

	await selectTool(page, 'Freehand');
	await expect(colorTrigger(page)).toBeVisible();

	await selectTool(page, 'Rectangle');
	await expect(colorTrigger(page)).toBeVisible();

	await selectTool(page, 'Erase');
	await expect(colorTrigger(page)).toBeHidden();
});

// The strip carries the current colour without being opened — that is
// what makes one button an acceptable trade for eight swatches.
test('the colour button names the colour it is on, and the popup rings it', async ({ page }) => {
	await createRoomWithScene(page);

	await expect(colorTrigger(page)).toHaveAccessibleName('Stroke colour: Black');

	await colorTrigger(page).click();
	await expect(swatch(page, 'Black')).toHaveAttribute('aria-pressed', 'true');
	for (const other of ['Red', 'Green', 'Blue']) {
		await expect(swatch(page, other)).toHaveAttribute('aria-pressed', 'false');
	}

	await swatch(page, 'Green').click();
	await expect(colorTrigger(page)).toHaveAccessibleName('Stroke colour: Green');

	// Escape rather than a click on a swatch, so both ways out are
	// exercised between this test and pickColor. Closing puts focus back on
	// the button it was opened from — the half of the popover that a
	// hand-rolled panel is most likely to be missing.
	await colorTrigger(page).click();
	await page.keyboard.press('Escape');
	await expect(swatch(page, 'Green')).toBeHidden();
	await expect(colorTrigger(page)).toBeFocused();
});

// The dark-map row is a second set of swatches in the same panel, not a
// mode: picking one has to move the selection off the light row rather
// than leaving both rows with something pressed.
test('the dark-map row picks from the same selection as the light one', async ({ page }) => {
	await createRoomWithScene(page);

	await pickColor(page, 'Bright green');
	await expect(colorTrigger(page)).toHaveAccessibleName('Stroke colour: Bright green');

	await colorTrigger(page).click();
	await expect(swatch(page, 'Bright green')).toHaveAttribute('aria-pressed', 'true');
	await expect(swatch(page, 'Black')).toHaveAttribute('aria-pressed', 'false');
	await expect(swatch(page, 'Green')).toHaveAttribute('aria-pressed', 'false');
	await page.keyboard.press('Escape');

	// And back, so the light row isn't left unreachable once the dark one
	// has been used.
	await pickColor(page, 'Red');
	await expect(colorTrigger(page)).toHaveAccessibleName('Stroke colour: Red');
});

test('the selected swatch is still clickable and shows a ring outside itself', async ({ page }) => {
	await createRoomWithScene(page);
	await colorTrigger(page).click();

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

// Below lg the strip docks into the sheet's horizontally scrolling bar,
// close enough to the bottom edge that the panel has to open upward, and
// inside a box whose vertical axis is `auto`. The width picker's spec has
// the long version of why that is the case a portalled panel exists for;
// this is the same check for the second panel on that strip.
test('the popup opens clear of the strip on a phone', async ({ page }) => {
	await createRoomWithScene(page);
	await page.setViewportSize({ width: 375, height: 812 });

	await colorTrigger(page).click();
	await expect(swatch(page, 'Bright blue')).toBeInViewport();

	// Clicking is the real check: Playwright verifies the click lands on
	// the element it aimed at, so a panel cut off by the scroller fails
	// here rather than quietly picking nothing.
	await swatch(page, 'Bright blue').click();
	await expect(colorTrigger(page)).toHaveAccessibleName('Stroke colour: Bright blue');
});
