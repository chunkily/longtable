<script lang="ts">
	// The third icon at the foot of the side panel. Opens *upward*,
	// because it sits at the bottom of the rail on a desktop and on the
	// bottom edge of the screen on a phone — there is nothing below it to
	// open into.
	//
	// On the popover primitive, where this used to hand-roll a
	// transparent full-screen backdrop, its own Escape handler and an
	// absolutely positioned panel. All three worked. What none of them
	// did was move focus into a menu of eight controls, or put it back on
	// this button afterwards — which is the half you can't retrofit and
	// the half a keyboard notices. `ui/popover` wraps the bits-ui that
	// was already behind the dialog and the slider, so this costs no
	// dependency; stroke-width-picker.svelte is where the argument was
	// had.
	import { resolve } from '$app/paths';
	import * as Popover from '$lib/components/ui/popover';
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

<!-- Disarmed on the way out, however it closes — Escape and a click on
     the map get here as well as `close()` does, and a menu reopening
     already asking "Really leave?" is the one state this must never come
     back in. -->
<Popover.Root
	bind:open
	onOpenChange={(next) => {
		if (!next) confirmingLeave = false;
	}}
>
	<Popover.Trigger>
		<!-- The child snippet so the trigger stays the app's own Button:
		     `props` carries the primitive's own attributes, aria-expanded
		     and aria-controls among them. -->
		{#snippet child({ props })}
			<Button
				{...props}
				variant={open ? 'default' : 'ghost'}
				size="sm"
				aria-label="Menu"
				title="Scenes, assets and room settings"
			>
				<Menu class="h-4 w-4" />
			</Button>
		{/snippet}
	</Popover.Trigger>
	<!-- Above the button and aligned to its right-hand edge, which is
	     where it sat when it placed itself: it is the last icon in a row
	     pinned to the bottom of the rail. -->
	<Popover.Content class="w-56 p-1" side="top" align="end">
		<div class="flex flex-col gap-1">
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
			<div class="border-t py-2">
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
	</Popover.Content>
</Popover.Root>
