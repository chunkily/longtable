import { expect, test, type Page, type WebSocketRoute } from '@playwright/test';
import { createRoom, openNewSceneDialog } from './fixtures/room';

// A dropped socket used to end the session until someone reloaded, with
// every command silently doing nothing and only a small status badge to
// say so. What matters here is that the client comes back on its own,
// converges on server state when it does, and says something useful when
// it can't — none of which is visible from unit tests, because it needs
// a real socket to lose.

// Lets a test cut a live connection from the outside, the way a wifi
// handover or a server restart would. The routed socket is a real proxy
// to the server, so everything works normally until close() is called.
async function interceptableSocket(page: Page) {
	const routes: WebSocketRoute[] = [];
	let refusing = false;

	await page.routeWebSocket(/\/ws\?/, (route) => {
		// Refusing has to happen here rather than by adding a second route
		// later: routeWebSocket only sees connections opened after it is
		// installed, so the socket a test wants to break must have been
		// routed all along.
		if (refusing) {
			void route.close({ code: 1006 });
			return;
		}
		routes.push(route);
		const server = route.connectToServer();
		route.onMessage((m) => server.send(String(m)));
		server.onMessage((m) => route.send(String(m)));
	});

	return {
		/** Drops the newest connection, without a goodbye. */
		cut: async () => {
			await routes.at(-1)!.close({ code: 1006 });
		},
		/** Makes every future attempt fail, as an unreachable server would. */
		refuseFromNowOn: () => {
			refusing = true;
		},
		count: () => routes.length
	};
}

async function createRoomWithScene(page: Page, name: string) {
	await createRoom(page, name);

	await openNewSceneDialog(page);
	await page.getByLabel('Name').fill('Map');
	await page.getByRole('button', { name: 'Create scene' }).click();
	await expect(page.locator('canvas').first()).toBeVisible();
}

test('a dropped connection comes back on its own, and says so while it is gone', async ({
	page
}) => {
	const socket = await interceptableSocket(page);
	await createRoomWithScene(page, 'Reconnect');
	await expect(page.getByText('open', { exact: true })).toBeVisible();

	await socket.cut();

	// The banner is the point: before this, a dead socket was invisible
	// beyond the badge while every command quietly did nothing.
	await expect(page.getByRole('status')).toContainText('Reconnecting');

	// And it comes back without anyone reloading.
	await expect(page.getByText('open', { exact: true })).toBeVisible({ timeout: 15_000 });
	await expect(page.getByRole('status')).toBeHidden();
	expect(socket.count()).toBeGreaterThan(1);
});

// state.sync replaces the whole picture and drops anything in flight, so
// a reconnect converges on server state by construction. This is that
// claim actually exercised: chat sent while the socket was down never
// arrives, and the log that comes back is the server's.
test('the room is whole again after a reconnect', async ({ page }) => {
	const socket = await interceptableSocket(page);
	await createRoomWithScene(page, 'Reconnect State');

	await page.getByPlaceholder('Say something').fill('before');
	await page.getByRole('button', { name: 'Send' }).click();
	await expect(page.getByText('before')).toBeVisible();

	await socket.cut();
	await expect(page.getByRole('status')).toContainText('Reconnecting');

	// Sent into a closed socket: it goes nowhere, which is the behaviour
	// the banner is warning about.
	await page.getByPlaceholder('Say something').fill('into the void');
	await page.getByRole('button', { name: 'Send' }).click();

	await expect(page.getByText('open', { exact: true })).toBeVisible({ timeout: 15_000 });

	// The history the server had is back, and the message that never made
	// it out is not in it.
	await expect(page.getByText('before')).toBeVisible();
	await expect(page.getByText('into the void')).toBeHidden();

	// Still a working room, not just a reconnected socket.
	await page.getByPlaceholder('Say something').fill('after');
	await page.getByRole('button', { name: 'Send' }).click();
	await expect(page.getByText('after')).toBeVisible();
});

// The case retrying can't fix. The probe is what separates it from a
// server that is merely restarting, and getting it wrong either bounces
// someone out over a blip or spins forever against a room that has
// forgotten them.
test('a session the server no longer knows stops the retrying and says to rejoin', async ({
	page
}) => {
	const socket = await interceptableSocket(page);
	// The room has to exist and be joined before the session goes bad —
	// with no state.sync there is no room page to put a banner on.
	await createRoomWithScene(page, 'Reconnect Expired');

	await page.route('**/api/rooms/*/session', (route) =>
		route.fulfill({ status: 401, json: { error: 'invalid session' } })
	);
	socket.refuseFromNowOn();
	await socket.cut();

	await expect(page.getByRole('status')).toContainText('no longer valid');
	await expect(page.getByRole('button', { name: 'Rejoin' })).toBeVisible();

	// And it really has stopped rather than quietly carrying on: the one
	// connection that existed is still the only one.
	await page.waitForTimeout(2000);
	expect(socket.count()).toBe(1);
});
