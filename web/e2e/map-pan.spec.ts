import type { Page } from '@playwright/test';
import { expect, test } from './fixtures/table';
import { openRoomMenu, selectTool } from './fixtures/room';
import { LAYER, canvasBox, createToken, layerInk, tokenInkAt, type Point } from './fixtures/map';

// Dragging the map with the right or middle mouse button, which has to
// work whatever tool is selected — that is the whole reason it isn't the
// left button, which every tool already owns.
//
// Konva draws to a canvas and the stage isn't reachable from the page,
// so "did the map move" is asked of pixels: a token is put on the map
// first and the tests watch where its ink ends up. A pan moves
// everything together, so the token is a marker for the whole scene.

/**
 * How far each of these gestures drags, in canvas pixels. Up and to the
 * left, so the marker token stays comfortably on screen from a spawn
 * near the middle — and far enough that the probe at the new position
 * can't be satisfied by the token having stayed where it was, whose
 * radius is 30.
 */
const PAN: Point = { x: -180, y: -120 };

const moved = (point: Point): Point => ({ x: point.x + PAN.x, y: point.y + PAN.y });

/**
 * What the page records about the contextmenu event it sees.
 *
 * All three fields are asserted, not just `prevented`, and the other two
 * are there to stop this passing or failing for the wrong reason.
 * `cancelable` guards against a false green: `preventDefault()` is a
 * silent no-op on an uncancelable event, so "prevented" being false
 * could mean either the app didn't try or the browser wouldn't listen.
 * `target` says the right-click landed on the map at all rather than on
 * something above it — the precondition the rest of the test rests on,
 * and the one this test failed on twice while the app was correct: the
 * `table` fixture used to leave the Scenes dialog open over the middle
 * of the map, and a bare `prevented: false` gave no way to see it.
 */
interface MenuEvent {
	prevented: boolean;
	cancelable: boolean;
	target: string;
}

/**
 * Drags on the map with a given mouse button, in canvas-relative
 * coordinates.
 *
 * Stepped rather than jumped, because the handler recomputes the stage's
 * position from the pointer on every move — a single jump would exercise
 * one sample where a real drag is dozens, and it is the accumulate-free
 * arithmetic across all of them that `src/lib/pan.test.ts` pins.
 */
async function dragWith(page: Page, button: 'left' | 'middle' | 'right', from: Point, to: Point) {
	const box = await canvasBox(page);
	await page.mouse.move(box.x + from.x, box.y + from.y);
	await page.mouse.down({ button });
	await page.mouse.move(box.x + to.x, box.y + to.y, { steps: 8 });
	await page.mouse.up({ button });
}

/**
 * Dismisses the Scenes dialog the `table` fixture leaves open over the
 * map.
 *
 * Creating a scene is a mode of that dialog, so it returns to the scene
 * list rather than closing, and `expect(canvas).toBeVisible()` says
 * nothing about what is on top of the canvas. Every other test here
 * dismisses it by accident — their first click lands on a toolbar button
 * outside it — but a right-click in the middle of the map lands on a
 * scene list item, which is exactly what this file's context-menu test
 * spent two rounds looking like an app bug.
 *
 * Escape rather than the Close button: that button is one of several in
 * the page answering to the same accessible name, and clicking it in the
 * fixture resolved to one belonging to a dialog that was already closed.
 */
async function dismissScenesDialog(page: Page) {
	await page.keyboard.press('Escape');
	await expect(page.getByRole('heading', { name: 'Scenes' })).toBeHidden();
}

/** The room's owner-only movement setting, from the GM's Manage room dialog. */
async function setMovement(page: Page, label: 'Anyone moves anything' | 'Only the owner') {
	await openRoomMenu(page);
	await page.getByRole('button', { name: 'Manage room' }).click();
	const choice = page.getByRole('button', { name: label });
	await choice.click();
	// The setting round-trips through the server, so the pressed state is
	// the signal it landed rather than merely that it was clicked.
	await expect(choice).toHaveAttribute('aria-pressed', 'true');
	await page.getByRole('button', { name: 'Close' }).click();
}

// The case the feature exists for. Every tool takes the left button
// exclusively, so before this a GM with a ruler in hand had to put it
// down to move the map and pick it up again afterwards.
test('right-dragging pans the map while a drawing tool is active', async ({ table }) => {
	const page = table.gm.page;
	const spawn = await createToken(page, 'Ogre');
	await selectTool(page, 'Line');

	// Started well away from the token, so this is unambiguously a gesture
	// on bare map, and below the floating toolbar's corner.
	const from: Point = { x: 900, y: 500 };
	await dragWith(page, 'right', from, moved(from));

	await expect.poll(() => tokenInkAt(page, moved(spawn))).toBeGreaterThan(0);
	expect(await tokenInkAt(page, spawn)).toBe(0);

	// The tool was live throughout and drew nothing, which is the half of
	// this that the right-click rules already promised.
	expect(await layerInk(page, LAYER.drawings)).toBe(0);

	// The positive control for that probe: the same drag on the left
	// button does draw, so the layer being empty above means the gesture
	// was a pan rather than that drawings are invisible here.
	await dragWith(page, 'left', from, moved(from));
	await expect.poll(() => layerInk(page, LAYER.drawings)).toBeGreaterThan(0);
});

// Middle-drag is the other convention for this, and it needed Konva's
// own `dragButtons` narrowing to `[0]` to work: shipped as `[0, 1]`, the
// middle button drags any draggable node, so a middle-press on a token
// picked the token up and a middle-press on bare map ran Konva's stage
// drag alongside this one — the map travelling at twice the speed of the
// hand.
test('middle-dragging pans without picking up the token underneath', async ({ table }) => {
	const page = table.gm.page;
	const spawn = await createToken(page, 'Ogre');

	// Deliberately starting *on* the token, in the Hand tool, where it is
	// draggable — the exact press that used to move it.
	await dragWith(page, 'middle', spawn, moved(spawn));

	await expect.poll(() => tokenInkAt(page, moved(spawn))).toBeGreaterThan(0);

	// The token travelling with the map looks identical to the token
	// having been dragged to the pointer, so the discriminating assertion
	// is against the server: a pan is local to this browser and persists
	// nothing, and a reload puts the camera back at the origin with the
	// token still on the square it was created on.
	await page.reload();
	await expect(page.locator('canvas').first()).toBeVisible();
	await expect.poll(() => tokenInkAt(page, spawn)).toBeGreaterThan(0);
	expect(await tokenInkAt(page, moved(spawn))).toBe(0);
});

// A menu opening on top of the map the user has just finished dragging
// reads as the pan having gone wrong. Suppressed for every right-click
// on the stage rather than only for ones that travelled — see the note
// where it's bound.
test('the browser context menu never opens over the map', async ({ table }) => {
	const page = table.gm.page;
	await dismissScenesDialog(page);

	await page.evaluate(() => {
		const w = window as unknown as { menu: MenuEvent | null };
		w.menu = null;
		// Bubble phase on the window, so this runs after the map's own
		// handler and reads the result of it rather than racing it.
		window.addEventListener('contextmenu', (e) => {
			const element = e.target as Element | null;
			w.menu = {
				prevented: e.defaultPrevented,
				cancelable: e.cancelable,
				target: element?.tagName ?? 'none'
			};
		});
	});

	// The middle of the canvas, rather than a point measured from its
	// corner: the map shares the window with a 368px rail and a floating
	// toolbar, and a right-click that lands on either of those would be
	// answered by the browser exactly as this test fears — while proving
	// nothing about the map. Hence asserting the target too.
	const box = await canvasBox(page);
	await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2, { button: 'right' });

	const menu = await page.evaluate(() => (window as unknown as { menu: MenuEvent | null }).menu);
	expect(menu).toEqual({ prevented: true, cancelable: true, target: 'CANVAS' });
});

// The reason the pan is allowed to run during a left-button gesture at
// all: a ruler, a template or a rectangle that has reached the edge of
// the screen can be dragged further by shoving the map along under it.
// Nothing in the pan defends the stroke — it survives because a pan
// moves the stage by exactly what the pointer moved, so the world point
// under the cursor doesn't change while both buttons are down.
test('right-dragging mid-stroke pans the map and leaves the stroke to carry on', async ({
	table
}) => {
	const page = table.gm.page;
	const spawn = await createToken(page, 'Ogre');
	await selectTool(page, 'Line');

	const box = await canvasBox(page);
	const from: Point = { x: 900, y: 500 };
	const rubberBandedTo = moved(from);

	// A shape half dragged out, still held.
	await page.mouse.move(box.x + from.x, box.y + from.y);
	await page.mouse.down();
	await page.mouse.move(box.x + rubberBandedTo.x, box.y + rubberBandedTo.y, { steps: 8 });
	expect(await layerInk(page, LAYER.drawings)).toBe(0);

	// The right button joins in and shoves the map, then lets go. The
	// marker token travels with the map, which is how "the view moved" is
	// told apart from "nothing happened".
	await page.mouse.down({ button: 'right' });
	await page.mouse.move(box.x + rubberBandedTo.x + PAN.x, box.y + rubberBandedTo.y + PAN.y, {
		steps: 8
	});
	await page.mouse.up({ button: 'right' });

	await expect.poll(() => tokenInkAt(page, moved(spawn))).toBeGreaterThan(0);

	// The right release did not commit the shape — the rule
	// no-draw-on-right-click settled, re-asserted because this is the
	// change most likely to undo it.
	expect(await layerInk(page, LAYER.drawings)).toBe(0);

	// And the shape is still live: it goes on being dragged into the part
	// of the map the pan just brought into view, and commits on its own
	// left release.
	await page.mouse.move(box.x + rubberBandedTo.x + PAN.x - 100, box.y + rubberBandedTo.y + PAN.y, {
		steps: 6
	});
	await page.mouse.up();
	await expect.poll(() => layerInk(page, LAYER.drawings)).toBeGreaterThan(0);
});

// A token the movement lock puts out of reach swallows mousedown
// outright, so that an undraggable token doesn't hand the press to
// Konva's stage drag and pan the whole scene. That guard had to learn
// about the button: a right press on such a token is someone panning the
// map, and refusing it turned locked tokens into holes the map couldn't
// be dragged from.
test('panning can start on a token the room lock says you may not move', async ({ table }) => {
	const player = await table.join();
	const spawn = await createToken(table.gm.page, 'Goblin');
	await expect.poll(() => tokenInkAt(player.page, spawn)).toBeGreaterThan(0);

	await setMovement(table.gm.page, 'Only the owner');

	// Nobody's token, and the player owns nothing — so on their screen
	// this is exactly the token that swallows a press.
	await dragWith(player.page, 'right', spawn, moved(spawn));

	await expect.poll(() => tokenInkAt(player.page, moved(spawn))).toBeGreaterThan(0);

	// The GM is looking at the same room and their map hasn't moved: a
	// pan is one browser's camera and goes nowhere near the wire.
	expect(await tokenInkAt(table.gm.page, spawn)).toBeGreaterThan(0);
});
