import { expect, test, type Page } from '@playwright/test';
import { createRoom, openNewSceneDialog } from './fixtures/room';

// The Host's banner. The server half — a flag becoming an endpoint — is
// covered by `internal/api/notice_test.go`; what needs a browser is the
// rest: that it appears on a page belonging to nobody, that dismissing
// it sticks, that a *new* message comes back, and that it doesn't sit on
// top of the room's toolbar.
//
// The endpoint is stubbed rather than the server restarted with a flag:
// the e2e harness builds and runs one binary for the whole suite, and a
// second one started differently would be a second harness.

async function serveNotice(page: Page, notice: string) {
	await page.route('**/api/notice', async (route) => {
		await route.fulfill({ json: { notice } });
	});
}

const banner = (page: Page) => page.getByRole('status').filter({ hasText: 'Back up at 9pm' });

test('the banner shows up, dismisses, and stays dismissed', async ({ page }) => {
	await serveNotice(page, 'Back up at 9pm');
	await page.goto('/');

	// The home page of a browser that has never joined anything: this is
	// the Host's message, not a room's.
	await expect(banner(page)).toBeVisible();

	await page.getByRole('button', { name: 'Dismiss this message' }).click();
	await expect(banner(page)).toHaveCount(0);

	// Still gone after a reload — dismissing is remembered, or every page
	// load would put it back and the button would be decoration.
	await page.reload();
	await expect(page.getByRole('button', { name: 'Dismiss this message' })).toHaveCount(0);
	await expect(page.getByText('Back up at 9pm')).toHaveCount(0);
});

// Dismissal is keyed by the message itself: a Host who changes the
// banner is saying something new, and it has to reach the people who
// dismissed the last one.
test('a new message comes back for someone who dismissed the old one', async ({ page }) => {
	await serveNotice(page, 'Back up at 9pm');
	await page.goto('/');
	await page.getByRole('button', { name: 'Dismiss this message' }).click();
	await expect(banner(page)).toHaveCount(0);

	await serveNotice(page, 'Moving to a new address on Friday');
	await page.reload();

	await expect(page.getByText('Moving to a new address on Friday')).toBeVisible();
});

// The room page is `fixed inset-0`, so it paints over anything the
// layout puts above it — the map and the toolbar have to be moved down
// rather than left to overlap.
test('the room makes room for it rather than sliding under it', async ({ page }) => {
	await serveNotice(page, 'Back up at 9pm');

	await createRoom(page, 'Notice Room');

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();

	const noticeBox = await banner(page).boundingBox();
	const toolbarBox = await page.getByRole('button', { name: 'Hand', exact: true }).boundingBox();
	const canvasBox = await page.locator('canvas').first().boundingBox();
	if (!noticeBox || !toolbarBox || !canvasBox) throw new Error('missing bounding box');

	expect(toolbarBox.y).toBeGreaterThanOrEqual(noticeBox.y + noticeBox.height);
	expect(canvasBox.y).toBeGreaterThanOrEqual(noticeBox.y + noticeBox.height);

	// And dismissing gives the space back rather than leaving a gap.
	await page.getByRole('button', { name: 'Dismiss this message' }).click();
	const reclaimed = await page.locator('canvas').first().boundingBox();
	expect(reclaimed!.y).toBeLessThan(canvasBox.y);
});
