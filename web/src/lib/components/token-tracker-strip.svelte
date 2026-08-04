<script lang="ts">
	// The three tracker slots as they appear in the selected-token panel,
	// with their values editable in place. Damage is the thing that
	// changes most often at a table and the thing you most want to change
	// without losing sight of the map, so it doesn't go behind a dialog.
	//
	// Values only. A slot's *label* stays in the edit dialog: it's set
	// once when a creature arrives and then read all evening, and putting
	// a text box for it here would double the width of a strip that has to
	// fit beside the token's name.
	import { tokenTrackers, trackerText, type RoomClient, type Token } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';

	let {
		room,
		token,
		editable
	}: {
		room: RoomClient;
		token: Token;
		/**
		 * Whether this client may change the values — a GM on any token, a
		 * Player on one they own. The server enforces it either way; this
		 * only decides between a box and a badge.
		 */
		editable: boolean;
	} = $props();

	const trackers = $derived(tokenTrackers(token));

	function valueText(value: number | null): string {
		return value === null ? '' : String(value);
	}

	// Committed on `change` rather than `input`: `input` fires on every
	// keystroke, so typing "12" would send an 11-point heal followed by
	// the real value, and holding a key down would flood the socket. A
	// change event is the blur or the Enter, which is when someone has
	// finished saying what they meant.
	function commit(index: number, raw: string) {
		const text = raw.trim();
		// Anything that isn't a number is dropped rather than stored as
		// NaN — the field re-renders from the token, so it snaps back to
		// what the room actually holds.
		const value = text === '' ? null : Number(text);
		if (value !== null && !Number.isFinite(value)) return;

		const next = trackers.map((t, i) => (i === index ? { ...t, value } : { ...t }));
		room.setTokenTrackers(token.id, next, token.conditions ?? []);
	}

	// Enter commits without waiting for a blur, which is what anyone
	// typing a new hit point total expects. The input isn't in a form, so
	// nothing else is listening for it.
	function handleKeydown(event: KeyboardEvent & { currentTarget: HTMLInputElement }) {
		if (event.key === 'Enter') event.currentTarget.blur();
	}
</script>

<!-- All three slots, always, unlike the hover card on the map which
     shows only the ones carrying a number. The panel is read while a
     token is being worked on, so three fixed positions in a fixed order
     is what makes it scannable — a box that appeared and shifted its
     neighbours along as slots were filled in would not be. Keyed by
     index for the same reason: a slot's identity is its position, and
     three empty ones are otherwise indistinguishable. -->
<div class="mt-1 flex flex-wrap items-center gap-1">
	{#each trackers as tracker, i (i)}
		{#if editable}
			<div class="flex items-center gap-1 rounded-md border px-1.5 py-0.5">
				<span class="text-xs text-muted-foreground">{tracker.label || i + 1}</span>
				<input
					type="number"
					inputmode="numeric"
					aria-label="{tracker.label || `Tracker ${i + 1}`} current value"
					placeholder="—"
					value={valueText(tracker.value)}
					onchange={(e) => commit(i, e.currentTarget.value)}
					onkeydown={handleKeydown}
					class="w-10 [appearance:textfield] bg-transparent text-center font-mono text-xs outline-none focus:ring-1 focus:ring-ring [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
				/>
			</div>
		{:else}
			<Badge variant="outline" class="font-mono text-xs">{trackerText(tracker)}</Badge>
		{/if}
	{/each}
	{#each token.conditions ?? [] as condition (condition)}
		<Badge variant="secondary" class="text-xs">{condition}</Badge>
	{/each}
</div>
