import { expect, type Page } from '@playwright/test';

/**
 * Shared helpers for driving the room page.
 *
 * These used to be copy-pasted into every spec that needed them. The
 * full-bleed layout made that untenable: tools now live behind a family
 * on the tool row, and scene actions moved into the room menu, so
 * "click the button called X" is two or three steps rather than one.
 * Centralising it means the next layout change is one edit here, not
 * twenty across the suite.
 */

/**
 * The pre-join screen asks which side of the screen you're on before it
 * asks anything else, so every arrival is at least two clicks now. These
 * three helpers are the three ways in; specs should use them rather than
 * spelling the steps out, because the next change to that flow should be
 * one edit here.
 */

/** Opens the Player side of the pre-join screen: the room's seats. */
export async function openSeatPicker(page: Page) {
	await page.getByRole('button', { name: 'Player', exact: true }).click();
	// Waits on the "I'm new here" slot rather than on a seat: the list is
	// fetched, that slot is the one thing on it that renders whatever
	// comes back, and specs asserting a seat is *absent* would otherwise
	// pass against a list that simply hadn't arrived.
	await expect(page.getByRole('button', { name: "I'm new here" })).toBeVisible();
}

/** Takes a seat someone has already sat in, by the name on it. */
export async function takeSeat(page: Page, seatName: string) {
	await openSeatPicker(page);
	await page.getByRole('button', { name: `Take ${seatName}'s seat` }).click();
	await expect(page.getByText('playing as')).toBeVisible();
}

/** Joins as someone the room has never seen: Player → I'm new here. */
export async function joinAsNewPlayer(page: Page, name: string) {
	await openSeatPicker(page);
	await page.getByRole('button', { name: "I'm new here" }).click();
	await page.getByLabel('Your name').fill(name);
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Join', exact: true })).toBeHidden();
}

/**
 * Signs in to the room's GM seat with the room password — the one seat
 * that is never on the picker, because it's a role boundary.
 */
export async function joinAsGM(page: Page, name: string, password: string) {
	await page.getByRole('button', { name: "I'm the GM" }).click();
	await page.getByLabel('Your name').fill(name);
	await page.getByLabel('GM password').fill(password);
	await page.getByRole('button', { name: 'Join', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Join', exact: true })).toBeHidden();
}

/**
 * Which family a tool variant lives under, for the tools that are only
 * reachable once their family is picked. A tool that *is* a family —
 * Hand, Ping — isn't here: clicking it on the tool row selects it
 * outright.
 */
const TOOL_FAMILY: Record<string, string> = {
	Freehand: 'Draw',
	Line: 'Draw',
	Rectangle: 'Draw',
	Ellipse: 'Draw',
	Erase: 'Draw',
	Distance: 'Measure',
	'Circle template': 'Measure',
	'Cone template': 'Measure',
	'Line template': 'Measure',
	'Cube template': 'Measure',
	'Reveal fog': 'Fog',
	'Hide fog': 'Fog'
};

/**
 * Selects a map tool, opening its family first when it has one.
 *
 * The wait at the end matters and isn't decoration: the canvas rebinds
 * its pointer handlers in a Svelte effect, so a drag issued in the same
 * tick as the click still runs under the previous tool. Waiting for the
 * button's own active styling is the observable signal that the state
 * change has been applied.
 *
 * Neither family nor variant buttons toggle, so this leaves the
 * requested tool selected whatever was selected before.
 */
export async function selectTool(page: Page, name: string) {
	const family = TOOL_FAMILY[name];
	if (family) {
		const familyButton = page.getByRole('button', { name: family, exact: true });
		await familyButton.click();
		await expect(familyButton).toHaveClass(/bg-primary/);
	}

	const button = page.getByRole('button', { name, exact: true });
	await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

/**
 * Picks a tool family without picking a variant, for the controls that
 * live on a family's strip but aren't tools — the fog family's
 * `Reveal all` and `Reset fog`, which are one-shot buttons rather than
 * modes.
 */
export async function selectToolFamily(page: Page, family: string) {
	const button = page.getByRole('button', { name: family, exact: true });
	await button.click();
	await expect(button).toHaveClass(/bg-primary/);
}

/** Opens the menu behind the third icon at the foot of the side panel. */
export async function openRoomMenu(page: Page) {
	await page.getByRole('button', { name: 'Menu', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Leave room', exact: true })).toBeVisible();
}

/**
 * Opens the New scene dialog, which lives in the room menu now that the
 * toolbar is five tool families and nothing else.
 */
export async function openNewSceneDialog(page: Page) {
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'New scene', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Create scene', exact: true })).toBeVisible();
}

/**
 * Navigates to the room's assets page, which is reached from the room
 * menu now that there is no page header to hang a link off.
 */
export async function openAssetsPage(page: Page) {
	await openRoomMenu(page);
	await page.getByRole('link', { name: 'Assets' }).click();
	await expect(page.getByRole('heading', { name: 'Assets' })).toBeVisible();
}

/** Opens the Scenes dialog from the room menu. */
export async function openScenesDialog(page: Page) {
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Scenes', exact: true }).click();
}

/**
 * Creates a scene with the given name, leaving it active. The room's
 * first scene activates on its own, which is what most specs rely on.
 */
export async function createScene(page: Page, name: string) {
	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill(name);
	await page.getByRole('button', { name: 'Create scene', exact: true }).click();
	await expect(page.getByRole('button', { name: 'Create scene', exact: true })).toBeHidden();
}

/**
 * How far down the canvas a gesture has to start to clear the floating
 * toolbar. Since the full-bleed layout the map reaches the top-left
 * corner of the window, and the tool row plus the active family's
 * contextual strip sit over it — roughly the first 380x110px belongs to
 * the toolbar, not to the map. A drag starting inside that lands on a
 * button instead of the canvas, which reads as "drawing silently stopped
 * working" rather than as a mis-aimed test.
 */
export const TOOLBAR_CLEARANCE_Y = 140;

/**
 * The point gestures meant for the map should be measured from: the
 * canvas's top-left corner, pushed below the floating toolbar.
 *
 * Every caller is dragging *on* the map and comparing ink before and
 * after, so none of them cares where the world origin is — they care
 * that the pointer lands on the canvas. Baking the clearance in here
 * keeps the reason in one place instead of an unexplained `+ 140` in a
 * dozen specs.
 */
export async function mapGestureOrigin(page: Page) {
	const box = await page.locator('canvas').first().boundingBox();
	if (!box) throw new Error('canvas has no bounding box');
	return { x: box.x, y: box.y + TOOLBAR_CLEARANCE_Y };
}
