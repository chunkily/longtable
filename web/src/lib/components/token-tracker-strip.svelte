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
	import { tokenTrackers, type RoomClient, type Token } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import FloatingNumberInput from '$lib/components/floating-number-input.svelte';

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
		 * only turns the box's own readonly flag on or off, so a Player
		 * sizing up a token they don't own reads the same box its owner
		 * would, just one that won't take a step from them.
		 */
		editable: boolean;
	} = $props();

	const trackers = $derived(tokenTrackers(token));

	// When a value is finished — a blur, an Enter, or one of the box's
	// step buttons. Parsing, the per-keystroke question and the Enter key
	// all live in FloatingNumberInput now; what's left here is the part
	// that knows about tokens.
	function commit(index: number, value: number | null) {
		const next = trackers.map((t, i) => (i === index ? { ...t, value } : { ...t }));
		room.setTokenTrackers(token.id, next, token.conditions ?? []);
	}
</script>

<!-- All three slots, always, unlike the hover card on the map which
     shows only the ones carrying a number. The panel is read while a
     token is being worked on, so three fixed positions in a fixed order
     is what makes it scannable — a box that appeared and shifted its
     neighbours along as slots were filled in would not be. Keyed by
     index for the same reason: a slot's identity is its position, and
     three empty ones are otherwise indistinguishable.

     One template for both roles, too — a Player who can't touch this
     token gets the identical box its owner does, just with `readonly`
     set, rather than a second, smaller rendering kept in step with the
     first by hand. -->
<div class="mt-1.5 grid grid-cols-3 gap-1.5">
	{#each trackers as tracker, i (i)}
		<FloatingNumberInput
			value={tracker.value}
			label={tracker.label || `Tracker ${i + 1}`}
			ariaLabel="{tracker.label || `Tracker ${i + 1}`} current value"
			readonly={!editable}
			oncommit={(next) => commit(i, next)}
		/>
	{/each}
</div>

{#if (token.conditions ?? []).length > 0}
	<div class="mt-1.5 flex flex-wrap items-center gap-1">
		{#each token.conditions ?? [] as condition (condition)}
			<Badge variant="secondary" class="text-xs">{condition}</Badge>
		{/each}
	</div>
{/if}
