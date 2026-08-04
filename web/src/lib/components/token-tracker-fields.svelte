<script lang="ts" module>
	// Kept in step with the server's own limits (maxTrackerLabel,
	// maxConditionText, maxTokenConditions in internal/ws/hub.go). These
	// are the polite half of the pair — the server refuses anything
	// longer, and a form that let someone type it first would turn a cap
	// into an error toast after they'd finished.
	export const TRACKER_LABEL_MAX = 16;
	export const CONDITION_MAX = 32;
	export const CONDITIONS_MAX = 12;
</script>

<script lang="ts">
	// The two fields on a token that a Player who owns it may change, so
	// this is the whole of the edit form they see. A GM gets it alongside
	// name, art, size, owner and visibility.
	import XIcon from '@lucide/svelte/icons/x';
	import { type Tracker } from '$lib/room.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';

	let {
		trackers = $bindable(),
		conditions = $bindable(),
		idPrefix = 'token'
	}: {
		/** Exactly TRACKER_SLOTS long. Bindable so the dialog reads it on submit. */
		trackers: Tracker[];
		conditions: string[];
		idPrefix?: string;
	} = $props();

	let draft = $state('');

	// The number input is bound as a string rather than through
	// `type="number"`'s valueAsNumber, because an empty box and a box
	// reading 0 both come back as NaN there — and those are exactly the
	// two states a tracker has to keep apart.
	function setValue(index: number, raw: string) {
		const text = raw.trim();
		trackers[index].value = text === '' ? null : Number(text);
	}

	function valueText(tracker: Tracker): string {
		return tracker.value === null ? '' : String(tracker.value);
	}

	function addCondition() {
		const text = draft.trim();
		if (!text || conditions.length >= CONDITIONS_MAX) return;
		// Deduped here as well as on the server, so adding one that's
		// already there reads as a no-op rather than as the tag silently
		// vanishing on the round trip.
		if (!conditions.some((c) => c.toLowerCase() === text.toLowerCase())) {
			conditions = [...conditions, text];
		}
		draft = '';
	}

	// Enter adds a tag rather than submitting the dialog — inside a form,
	// a bare Enter in a text input would otherwise save and close before
	// the tag was ever added.
	function handleDraftKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		addCondition();
	}
</script>

<fieldset class="flex flex-col gap-2">
	<legend class="mb-2 text-sm font-medium">Trackers</legend>
	<p class="text-xs text-muted-foreground">
		Three numbers of your choosing — hit points, armour class, whatever this creature needs. Leave a
		value blank to hide the slot.
	</p>
	<!-- Indexed rather than keyed by content: the array is exactly
	     TRACKER_SLOTS long and a slot's identity *is* its position, so
	     clearing one must not move the others. -->
	{#each trackers, i (i)}
		<div class="flex gap-2">
			<Input
				id="{idPrefix}-tracker-{i}-label"
				aria-label="Tracker {i + 1} label"
				placeholder={i === 0 ? 'HP' : 'Label'}
				maxlength={TRACKER_LABEL_MAX}
				bind:value={trackers[i].label}
				class="flex-1"
			/>
			<Input
				id="{idPrefix}-tracker-{i}-value"
				aria-label="Tracker {i + 1} value"
				type="number"
				inputmode="numeric"
				placeholder="—"
				value={valueText(trackers[i])}
				oninput={(e) => setValue(i, e.currentTarget.value)}
				class="w-24"
			/>
		</div>
	{/each}
</fieldset>

<div class="flex flex-col gap-2">
	<Label for="{idPrefix}-condition">Conditions</Label>
	{#if conditions.length}
		<div class="flex flex-wrap gap-1">
			{#each conditions as condition (condition)}
				<Badge variant="secondary" class="gap-1 pr-1">
					{condition}
					<button
						type="button"
						aria-label="Remove {condition}"
						class="rounded-sm opacity-70 hover:opacity-100"
						onclick={() => (conditions = conditions.filter((c) => c !== condition))}
					>
						<XIcon class="h-3 w-3" />
					</button>
				</Badge>
			{/each}
		</div>
	{/if}
	<div class="flex gap-2">
		<Input
			id="{idPrefix}-condition"
			placeholder="Prone, Poisoned, Concentrating…"
			maxlength={CONDITION_MAX}
			disabled={conditions.length >= CONDITIONS_MAX}
			bind:value={draft}
			onkeydown={handleDraftKeydown}
			class="flex-1"
		/>
		<Button
			type="button"
			variant="outline"
			aria-label="Add condition"
			disabled={!draft.trim() || conditions.length >= CONDITIONS_MAX}
			onclick={addCondition}
		>
			Add
		</Button>
	</div>
</div>
