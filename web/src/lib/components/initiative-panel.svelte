<script lang="ts">
	// The turn order, in the rail's second panel.
	//
	// The GM owns every change — the server refuses all six commands from
	// anyone else — so a Player's version of this is the same list with
	// the controls left off rather than a different component. Keeping it
	// one component is what stops the two drifting into disagreeing about
	// what the order *is*.
	import { assetUrl } from '$lib/api';
	import type { InitiativeEntry, RoomClient } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import SkipBack from '@lucide/svelte/icons/skip-back';
	import SkipForward from '@lucide/svelte/icons/skip-forward';
	import Trash2 from '@lucide/svelte/icons/trash-2';
	import Eye from '@lucide/svelte/icons/eye';
	import EyeOff from '@lucide/svelte/icons/eye-off';

	let {
		room,
		isGM,
		selectedTokenId = $bindable(null)
	}: {
		room: RoomClient;
		isGM: boolean;
		/**
		 * Clicking an entry selects the token it stands for, which is the
		 * last checkbox of token-selection-highlight — it was waiting for
		 * entries to exist. Bindable because the selection belongs to the
		 * room page and the canvas reads it too; nothing about it goes on
		 * the wire.
		 */
		selectedTokenId?: string | null;
	} = $props();

	// The add form. `tokenId` is the empty string for a freestanding
	// entry rather than null, because that is what a <select> without a
	// value gives back — converted on the way out, once.
	let newTokenId = $state('');
	let newName = $state('');
	let newInitiative = $state('');
	let newHidden = $state(false);

	const tracker = $derived(room.initiative);
	// Only tokens on the scene in front of us: a token from another scene
	// would be an entry nobody can see, which reads as a bug rather than
	// as prep.
	const linkable = $derived(room.tokens);

	function handleAdd(event: SubmitEvent) {
		event.preventDefault();
		const value = Number(newInitiative);
		if (!Number.isFinite(value)) return;
		if (!newTokenId && !newName.trim()) return;

		room.addInitiativeEntry({
			tokenId: newTokenId || null,
			name: newName.trim(),
			initiative: value,
			hidden: newHidden
		});
		newTokenId = '';
		newName = '';
		newInitiative = '';
		newHidden = false;
	}

	// Sent on change rather than on every keystroke: a new value re-sorts
	// the list, and re-sorting under a cursor mid-number is how you end
	// up typing "1" into one entry and "5" into another.
	function handleValue(entry: InitiativeEntry, raw: string) {
		const value = Number(raw);
		if (!Number.isFinite(value) || value === entry.initiative) return;
		room.updateInitiativeEntry(entry.id, {
			name: entry.name,
			initiative: value,
			hidden: entry.hidden
		});
	}

	function toggleHidden(entry: InitiativeEntry) {
		room.updateInitiativeEntry(entry.id, {
			name: entry.name,
			initiative: entry.initiative,
			hidden: !entry.hidden
		});
	}

	// Two clicks, like removing a seat or deleting a scene: clearing is
	// the one action here that can't be undone by pressing the other
	// button.
	let confirmingClear = $state(false);
	function handleClear() {
		if (!confirmingClear) {
			confirmingClear = true;
			return;
		}
		room.clearInitiative();
		confirmingClear = false;
	}
</script>

<div class="flex min-h-0 flex-1 flex-col gap-2">
	<div class="flex items-center gap-2 border-b pb-2">
		<span class="text-sm font-medium">Round {tracker.round}</span>
		{#if isGM}
			<div class="ml-auto flex items-center gap-1">
				<Button
					variant="ghost"
					size="sm"
					aria-label="Previous turn"
					title="Back to the previous turn"
					disabled={tracker.entries.length === 0}
					onclick={() => room.advanceInitiative('previous')}
				>
					<SkipBack class="h-4 w-4" />
				</Button>
				<Button
					size="sm"
					aria-label="Next turn"
					title="Hand the turn on"
					disabled={tracker.entries.length === 0}
					onclick={() => room.advanceInitiative('next')}
				>
					<SkipForward class="h-4 w-4" />
				</Button>
			</div>
		{/if}
	</div>

	<ul class="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto">
		{#each tracker.entries as entry (entry.id)}
			{@const isCurrent = entry.id === tracker.currentEntryId}
			<!-- Keyed by id so a re-sort moves the row rather than rebuilding
			     it: the GM's value box is inside, and a rebuild mid-edit
			     would take the caret with it. -->
			<li
				class={[
					'flex items-center gap-2 rounded-md border p-1 text-sm',
					isCurrent && 'border-primary bg-accent'
				]}
			>
				{#if isGM}
					<!-- The value is the order, so it is the one field editable
					     in place — changing it re-sorts the list under you,
					     which is the point. -->
					<Input
						type="number"
						aria-label="{entry.name} initiative"
						class="h-8 w-14 text-center"
						value={String(entry.initiative)}
						onchange={(e) => handleValue(entry, e.currentTarget.value)}
					/>
				{:else}
					<span class="w-8 text-center font-medium tabular-nums">{entry.initiative}</span>
				{/if}

				<!-- A token-linked entry selects its token on the map; a
				     freestanding one has nothing to select, so it isn't a
				     button at all rather than a button that does nothing. -->
				{#if entry.tokenId}
					{@const tokenId = entry.tokenId}
					<!-- Named rather than left to its contents: the visible text
					     is just the combatant's name, which is also what the row
					     beside it and half the map are called.

					     The entry also *reads* the selection, not only sets it, so
					     the panel says which combatant is the token currently
					     ringed on the map — the two are the same answer and used
					     to be spelled out in one direction only. -->
					<button
						type="button"
						class={[
							'flex min-w-0 flex-1 items-center gap-2 text-left',
							tokenId === selectedTokenId && 'text-primary'
						]}
						aria-label="Find {entry.name} on the map"
						aria-current={tokenId === selectedTokenId ? 'true' : undefined}
						title="Find {entry.name} on the map"
						onclick={() => (selectedTokenId = tokenId)}
					>
						{#if entry.imageAssetId}
							<img
								src={assetUrl(entry.imageAssetId)}
								alt=""
								class="h-6 w-6 shrink-0 rounded-full object-cover"
							/>
						{/if}
						<span class="truncate">{entry.name}</span>
					</button>
				{:else}
					<span class="min-w-0 flex-1 truncate">{entry.name}</span>
				{/if}

				{#if isCurrent}
					<Badge variant="secondary">now</Badge>
				{/if}

				{#if isGM}
					{#if entry.hidden}
						<Badge variant="outline">hidden</Badge>
					{/if}
					<div class="flex shrink-0 items-center">
						<Button
							variant="ghost"
							size="sm"
							class="h-7 w-7 p-0"
							aria-label="Move {entry.name} up"
							title="Move up, among the entries tied with it"
							onclick={() => room.reorderInitiativeEntry(entry.id, 'up')}
						>
							<ChevronUp class="h-3 w-3" />
						</Button>
						<Button
							variant="ghost"
							size="sm"
							class="h-7 w-7 p-0"
							aria-label="Move {entry.name} down"
							title="Move down, among the entries tied with it"
							onclick={() => room.reorderInitiativeEntry(entry.id, 'down')}
						>
							<ChevronDown class="h-3 w-3" />
						</Button>
						<!-- Only for a freestanding entry: a linked one's
						     visibility is its token's, and two switches for one
						     answer is how they end up disagreeing. -->
						{#if !entry.tokenId}
							<Button
								variant="ghost"
								size="sm"
								class="h-7 w-7 p-0"
								aria-label={entry.hidden ? `Show ${entry.name}` : `Hide ${entry.name}`}
								title={entry.hidden ? 'Show this to the players' : 'Hide this from the players'}
								onclick={() => toggleHidden(entry)}
							>
								{#if entry.hidden}
									<EyeOff class="h-3 w-3" />
								{:else}
									<Eye class="h-3 w-3" />
								{/if}
							</Button>
						{/if}
						<Button
							variant="ghost"
							size="sm"
							class="h-7 w-7 p-0"
							aria-label="Remove {entry.name}"
							title="Take this combatant out of the order"
							onclick={() => room.removeInitiativeEntry(entry.id)}
						>
							<Trash2 class="h-3 w-3" />
						</Button>
					</div>
				{/if}
			</li>
		{:else}
			<li class="p-2 text-sm text-muted-foreground">
				{#if isGM}
					Nobody in the order yet. Add a token or a name below, then hand out turns.
				{:else}
					The GM hasn't started an encounter.
				{/if}
			</li>
		{/each}
	</ul>

	{#if isGM}
		<form class="flex flex-col gap-2 border-t pt-2" onsubmit={handleAdd}>
			<div class="flex gap-2">
				<div class="flex min-w-0 flex-1 flex-col gap-1">
					<Label for="initiative-token" class="text-xs">Combatant</Label>
					<select
						id="initiative-token"
						bind:value={newTokenId}
						class="h-9 rounded-md border bg-background px-2 text-sm"
					>
						<option value="">Something else…</option>
						{#each linkable as token (token.id)}
							<option value={token.id}>{token.name}</option>
						{/each}
					</select>
				</div>
				<div class="flex w-16 flex-col gap-1">
					<Label for="initiative-value" class="text-xs">Rolled</Label>
					<Input
						id="initiative-value"
						type="number"
						class="h-9 text-center"
						bind:value={newInitiative}
						required
					/>
				</div>
			</div>

			<!-- Only for a freestanding entry: a token brings its own name,
			     and a name box that silently did nothing would be worse than
			     no box. -->
			{#if !newTokenId}
				<div class="flex items-end gap-2">
					<div class="flex min-w-0 flex-1 flex-col gap-1">
						<!-- "Call it" rather than "Name", and not for style: both
						     side panels stay mounted, so this box shares the page
						     with every dialog the room can open — and Playwright's
						     getByLabel matches on a substring, so a second "Name"
						     anywhere makes `getByLabel('Name')` ambiguous in
						     fifteen existing specs at once. -->
						<Label for="initiative-name" class="text-xs">Call it</Label>
						<Input
							id="initiative-name"
							class="h-9"
							placeholder="Lair action, hazard…"
							bind:value={newName}
						/>
					</div>
					<Button
						type="button"
						variant={newHidden ? 'default' : 'outline'}
						size="sm"
						class="h-9"
						aria-pressed={newHidden}
						title="Keep this off the players' tracker"
						onclick={() => (newHidden = !newHidden)}
					>
						{#if newHidden}
							<EyeOff class="h-4 w-4" />
						{:else}
							<Eye class="h-4 w-4" />
						{/if}
					</Button>
				</div>
			{/if}

			<div class="flex gap-2">
				<Button type="submit" size="sm" class="flex-1">Add to order</Button>
				<Button
					type="button"
					variant={confirmingClear ? 'destructive' : 'outline'}
					size="sm"
					aria-label={confirmingClear ? 'Confirm clearing the tracker' : 'Clear the tracker'}
					title="Empty the order for the next encounter — the tokens stay on the map"
					disabled={tracker.entries.length === 0}
					onclick={handleClear}
				>
					{confirmingClear ? 'Really clear?' : 'Clear'}
				</Button>
			</div>
		</form>
	{/if}
</div>
