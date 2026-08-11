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
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import type { RoomClient } from '$lib/room.svelte';
	import Menu from '@lucide/svelte/icons/menu';
	import Images from '@lucide/svelte/icons/images';
	import Layers from '@lucide/svelte/icons/layers';
	import Settings from '@lucide/svelte/icons/settings';
	import LogOut from '@lucide/svelte/icons/log-out';
	import Redo from '@lucide/svelte/icons/redo';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';

	let {
		room,
		slug,
		isGM,
		onOpenScenes,
		onOpenManageRoom,
		onOpenRoomCode,
		onLeave,
		onResetView
	}: {
		room: RoomClient;
		slug: string;
		isGM: boolean;
		onOpenScenes: () => void;
		onOpenManageRoom: () => void;
		onOpenRoomCode: () => void;
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
			<!-- The code sits at the top of the menu and shows itself, so the
			     answer to "what's the code?" is one tap away and readable
			     without opening anything. Monospace because it gets read out
			     character by character, and a proportional font makes that
			     harder than it needs to be. Everyone gets it, not just the
			     GM: a Player is as likely to be the one messaging whoever is
			     running late, and can read it out of their own address bar
			     anyway — see ADR-0007. -->
			<Button
				variant="ghost"
				class="h-auto justify-start py-2"
				onclick={() => {
					close();
					onOpenRoomCode();
				}}
			>
				<span class="flex flex-col items-start gap-0.5">
					<span class="text-xs font-normal text-muted-foreground">Room code</span>
					<span class="font-mono text-base tracking-widest">{slug}</span>
				</span>
			</Button>
			<div class="border-t"></div>
			<!-- One entry, not two. New scene used to sit beside this as its
			     own dialog, which meant the menu asked whether you wanted the
			     scenes or a new one before showing you either. It is a mode
			     of the Scenes dialog now — reached from the foot of the list
			     it will join — so nothing here opens a second dialog over a
			     first, which was the original reason for keeping them apart. -->
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
			<!-- Down here with Leave room rather than up with Scenes and
			     Assets, because those change the room and these two change
			     only this browser. The room page is where anyone spends the
			     evening, so this is where the light gets turned down —
			     there is no settings page to send them to, and a route
			     holding one control would be a page that lies about how
			     much is on it. -->
			<div class="border-t px-1 pt-2 pb-1">
				<ThemeToggle />
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
