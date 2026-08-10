import { test as base, expect, type BrowserContext, type Page } from '@playwright/test';
import { createRoom, joinAsNewPlayer, openNewSceneDialog } from './room';

/**
 * A room with people at it, as a Playwright fixture.
 *
 * Every spec used to build this by hand — `browser.newContext()`, create
 * the room, make a scene, join a player — and then close the contexts on
 * the last line of the test. **That last line is the problem this
 * fixture exists to remove.** It doesn't run when an assertion fails, so
 * a failing test leaks its browser contexts, and each leaked context
 * keeps a live WebSocket and a connected participant for the rest of the
 * run. One real failure quietly makes the tests after it stranger, which
 * is exactly the shape of "the suite is flaky when it's already unhappy".
 *
 * A fixture's teardown runs whatever the outcome, so the leak cannot
 * happen. The duplicated setup going away is the smaller half of the
 * win, though it's the visible one: nine copies of "open a room as GM"
 * were nine chances to wait for the wrong thing.
 *
 * A spec that wants a room with no scene on it:
 *
 * ```ts
 * test.use({ scene: false });
 * ```
 */

export interface Member {
	context: BrowserContext;
	page: Page;
}

export interface Table {
	/** The room's join code, for the specs that open it a third time. */
	slug: string;
	/** The GM's own browser, in the room, with a scene up unless opted out. */
	gm: Member;
	/**
	 * Another person, on their own device — a separate context, never a
	 * second tab. Tabs share localStorage, so a second tab is the same
	 * seat and proves nothing about two people.
	 */
	join(name?: string): Promise<Member>;
}

interface TableOptions {
	/** Whether the fixture makes a scene. True for anything touching the map. */
	scene: boolean;
}

export const test = base.extend<TableOptions & { table: Table }>({
	scene: [true, { option: true }],

	table: async ({ browser, scene }, use, testInfo) => {
		const opened: BrowserContext[] = [];

		async function device(): Promise<Member> {
			const context = await browser.newContext();
			opened.push(context);
			return { context, page: await context.newPage() };
		}

		const gm = await device();
		// Named after the test, so a room found in the database while
		// debugging says which test left it there.
		const slug = await createRoom(gm.page, testInfo.title.slice(0, 60));

		if (scene) {
			await openNewSceneDialog(gm.page);
			await gm.page.getByLabel('Name').fill('Map');
			await gm.page.getByRole('button', { name: 'Create scene' }).click();
			await expect(gm.page.locator('canvas').first()).toBeVisible();
		}

		await use({
			slug,
			gm,
			async join(name = 'Bob') {
				const member = await device();
				await member.page.goto(`/r/${slug}`);
				await joinAsNewPlayer(member.page, name);
				if (scene) await expect(member.page.locator('canvas').first()).toBeVisible();
				return member;
			}
		});

		// Runs whether the test passed, failed or timed out — which is the
		// whole point.
		for (const context of opened) await context.close();
	}
});

export { expect };
