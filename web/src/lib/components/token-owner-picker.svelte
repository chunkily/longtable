<script lang="ts">
	// Who a token belongs to. Shared by the create and edit dialogs, the
	// way the size picker is, so the two can't drift into disagreeing
	// about what "unowned" is called.
	//
	// A native <select> rather than the row of buttons the other pickers
	// use: size and visibility have four options and two, and a room has
	// as many members as it has people. A button per player is fine at
	// four and unusable at twelve.
	//
	// The list is whoever is connected — see `ownerOptions`. You're always
	// on it yourself, since the hub registers a connection before it sends
	// the state that lists them, so it is never empty.
	import type { OwnerOption } from '$lib/token-owner';
	import { Label } from '$lib/components/ui/label';

	let {
		ownerId = $bindable(null),
		options,
		idPrefix = 'token'
	}: {
		/** The owner's participant id, or null for nobody. Bindable so a dialog can read it on submit. */
		ownerId?: string | null;
		/**
		 * Who may be picked, from `ownerOptions` — the people connected
		 * right now, plus this token's owner if they've since left. The
		 * rule lives there rather than here so it can be tested and so both
		 * dialogs get the same answer.
		 */
		options: OwnerOption[];
		idPrefix?: string;
	} = $props();

	/**
	 * The empty string stands in for null inside the select, because an
	 * <option> value is always a string. It goes back to null on the way
	 * out, so nothing downstream has to know about the placeholder.
	 */
	const UNOWNED = '';
</script>

<div class="flex flex-col gap-2">
	<Label for="{idPrefix}-owner">Owner</Label>
	<select
		id="{idPrefix}-owner"
		class="h-9 w-full min-w-0 rounded-md border border-input bg-transparent px-2.5 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 md:text-sm dark:bg-input/30"
		value={ownerId ?? UNOWNED}
		onchange={(event) => {
			const chosen = event.currentTarget.value;
			ownerId = chosen === UNOWNED ? null : chosen;
		}}
	>
		<!-- First and default, because most tokens are monsters. -->
		<option value={UNOWNED}>Nobody (monster or prop)</option>
		{#each options as { participant, online } (participant.id)}
			<option value={participant.id}>
				{participant.displayName}{participant.role === 'gm' ? ' (GM)' : ''}{online
					? ''
					: ' — not connected'}
			</option>
		{/each}
	</select>
</div>
