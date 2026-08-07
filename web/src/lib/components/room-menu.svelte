<script lang="ts">
	// The third icon at the foot of the side panel. Opens *upward*,
	// because it sits at the bottom of the rail on a desktop and on the
	// bottom edge of the screen on a phone — there is nothing below it to
	// open into.
	//
	// Hand-rolled rather than a dropdown-menu primitive: `ui/` has no
	// menu component, and this needs four items, an upward anchor and a
	// destructive action that arms in place. A full menu primitive would
	// be more machinery than the thing it holds.
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import type { RoomClient } from '$lib/room.svelte';
	import Menu from '@lucide/svelte/icons/menu';
	import Images from '@lucide/svelte/icons/images';
	import Layers from '@lucide/svelte/icons/layers';
	import Plus from '@lucide/svelte/icons/plus';
	import Settings from '@lucide/svelte/icons/settings';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Redo from '@lucide/svelte/icons/redo';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';

	let {
		room,
		slug,
		isGM,
		onOpenScenes,
		onOpenNewScene,
		onOpenManageRoom,
		onLeave,
		onResetView
	}: {
		room: RoomClient;
		slug: string;
		isGM: boolean;
		onOpenScenes: () => void;
		onOpenNewScene: () => void;
		onOpenManageRoom: () => void;
		onLeave: () => void;
		onResetView: () => void;
	} = $props();

	let open = $state(false);
	// Leaving forgets this browser's session, which is the whole of what
	// being in a room is — rejoining afterwards mints a *new* participant
	// rather than picking the old one back up, so anything owned by the
	// old one stays owned by a person who is no longer you. Worth arming
	// rather than firing on one click, the same way deleting a scene is.
	let confirmingLeave = $state(false);

	function close() {
		open = false;
		confirmingLeave = false;
	}

	function handleLeave() {
		if (!confirmingLeave) {
			confirmingLeave = true;
			return;
		}
		close();
		onLeave();
	}
</script>

<svelte:window
	onkeydown={(event) => {
		if (event.key === 'Escape') close();
	}}
/>

<div class="relative">
	{#if open}
		<!-- A full-screen backdrop rather than outside-click bookkeeping on
		     the menu itself: it catches the click anywhere, including over
		     the canvas, without the menu having to know what else is on
		     screen. Transparent, so nothing about the map is obscured. -->
		<button
			type="button"
			class="fixed inset-0 z-40 cursor-default"
			aria-label="Close menu"
			onclick={close}
		></button>
		<div
			class="absolute right-0 bottom-full z-50 mb-1 flex w-56 flex-col gap-1 rounded-md border bg-popover p-1 shadow-md"
		>
			<!-- Both Scenes and New scene live here, which is what the design
			     session meant by them leaving the toolbar. New scene is a
			     second entry rather than a button inside the Scenes dialog:
			     opening one dialog from another leaves two stacked, each with
			     its own focus trap, and the list underneath is not something
			     you are still reading while naming a new scene. -->
			{#if isGM}
				<Button
					variant="ghost"
					class="justify-start"
					onclick={() => {
						close();
						onOpenScenes();
					}}
				>
					<Layers class="h-4 w-4" />
					Scenes
				</Button>
				<Button
					variant="ghost"
					class="justify-start"
					onclick={() => {
						close();
						onOpenNewScene();
					}}
				>
					<Plus class="h-4 w-4" />
					New scene
				</Button>
			{/if}
			<!-- Anyone in the room, not just the GM: players bring their own
			     token art, and the library is shared. A link rather than a
			     dialog — folding the assets page into a 380px rail was
			     considered and rejected, since its upload flow asks for name,
			     credit and grid alignment, and alignment wants width. -->
			<Button variant="ghost" class="justify-start" href={resolve('/r/[slug]/assets', { slug })}>
				<Images class="h-4 w-4" />
				Assets
			</Button>
			{#if isGM}
				<Button
					variant="ghost"
					class="justify-start"
					onclick={() => {
						close();
						onOpenManageRoom();
					}}
				>
					<Settings class="h-4 w-4" />
					Manage room
				</Button>
			{/if}
			<!-- Redo and reset view live on the toolbar at lg and up; below
			     that the toolbar's second cluster shrinks to undo alone and
			     they turn up here, so every desktop action stays reachable
			     on a phone even if it takes an extra tap. -->
			<div class="flex flex-col gap-1 border-t pt-1 lg:hidden">
				<Button
					variant="ghost"
					class="justify-start"
					disabled={!room.canRedo}
					onclick={() => {
						close();
						room.redo();
					}}
				>
					<Redo class="h-4 w-4" />
					Redo
				</Button>
				<Button
					variant="ghost"
					class="justify-start"
					onclick={() => {
						close();
						onResetView();
					}}
				>
					<RefreshCw class="h-4 w-4" />
					Reset view
				</Button>
			</div>
			<div class="border-t pt-1">
				<Button
					variant={confirmingLeave ? 'destructive' : 'ghost'}
					class="w-full justify-start"
					aria-label={confirmingLeave ? 'Confirm leaving the room' : 'Leave room'}
					onclick={handleLeave}
				>
					<LogOut class="h-4 w-4" />
					{confirmingLeave ? 'Really leave?' : 'Leave room'}
				</Button>
			</div>
		</div>
	{/if}

	<Button
		variant={open ? 'default' : 'ghost'}
		size="sm"
		aria-label="Menu"
		aria-expanded={open}
		title="Scenes, assets and room settings"
		onclick={() => (open ? close() : (open = true))}
	>
		<Menu class="h-4 w-4" />
	</Button>
</div>
