<script lang="ts">
	// A tracker value big enough to read at arm's length, with its
	// adjust-by-N control kept off the strip until someone wants it.
	//
	// The floating panel is what makes the bigger box affordable at all:
	// three of these share a sidebar with the token's name, and a visible
	// pair of step buttons per slot would cost exactly the width the
	// larger numbers just bought. Overlaying them on focus spends no
	// layout, and they're only useful while a slot is being edited anyway.
	import MinusIcon from '@lucide/svelte/icons/minus';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { autoUpdate, computePosition, flip, offset, shift } from '@floating-ui/dom';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';

	let {
		value,
		label,
		ariaLabel,
		oncommit
	}: {
		/** The room's value for this slot, or null when it's unset. */
		value: number | null;
		/** The slot's name, shown above the box. */
		label: string;
		/**
		 * The accessible name. The e2e specs pick these boxes out from the
		 * edit dialog's identically-purposed fields by it, so callers pass
		 * "<slot> current value" and nothing else.
		 */
		ariaLabel: string;
		/**
		 * Called with a finished value — a blur, an Enter, or a step
		 * button — and never per keystroke. Typing "12" would otherwise
		 * broadcast a one-point total on the way past.
		 */
		oncommit: (next: number | null) => void;
	} = $props();

	// What the box shows. Held locally rather than read straight off the
	// token so a step button can show its result on the next frame
	// instead of after the socket has echoed it back.
	//
	// Typed loosely because `bind:value` on a number input hands back a
	// number (and null for an empty box), not the string that was typed.
	// Everything here goes through `parse`, which is why that takes both:
	// assuming a string cost an afternoon once, because the failure is a
	// TypeError inside a click handler, which shows up as a button that
	// silently does nothing rather than as an error anyone sees.
	let draft = $state<string | number | null>('');
	let amount = $state<string | number | null>('');
	let focused = $state(false);

	// The last number this box put on the wire, so the change event that
	// follows a step button doesn't repeat what the button already said.
	// Cleared on focus because it only has to hold for one interaction.
	let lastSent: number | null | undefined = undefined;

	let container: HTMLDivElement | undefined = $state();
	let referenceEl: HTMLInputElement | null = $state(null);
	let floatingEl: HTMLDivElement | undefined = $state();

	// While the box has focus its contents belong to whoever is typing;
	// re-seeding from an echo mid-edit would overwrite them. Unfocused it
	// follows the token, which is how someone else's change to the same
	// slot turns up here.
	$effect(() => {
		if (focused) return;
		draft = value === null ? '' : String(value);
	});

	$effect(() => {
		if (!focused || !referenceEl || !floatingEl) return;

		const cleanup = autoUpdate(referenceEl, floatingEl, async () => {
			if (!referenceEl || !floatingEl) return;
			const { x, y } = await computePosition(referenceEl, floatingEl, {
				placement: 'top',
				// flip() matters more here than it looks: this panel lives in
				// a sidebar on a desktop but in a bottom sheet on a phone,
				// where "above the input" can be off the top of the screen.
				middleware: [offset(8), flip(), shift({ padding: 8 })]
			});
			Object.assign(floatingEl.style, { left: `${x}px`, top: `${y}px` });
		});

		return cleanup;
	});

	function parse(raw: string | number | null | undefined): number | null {
		if (raw === null || raw === undefined) return null;
		if (typeof raw === 'number') return Number.isFinite(raw) ? raw : null;
		const trimmed = raw.trim();
		if (trimmed === '') return null;
		const parsed = Number(trimmed);
		return Number.isFinite(parsed) ? parsed : null;
	}

	function send(next: number | null) {
		if (next === lastSent) return;
		lastSent = next;
		oncommit(next);
	}

	// Committed on `change` rather than `input` — the change event is the
	// blur or the Enter, which is when someone has finished saying what
	// they meant.
	function handleChange(event: Event & { currentTarget: HTMLInputElement }) {
		const text = event.currentTarget.value;
		// Anything unparseable is dropped rather than stored as NaN; the
		// box re-seeds from the token, so it snaps back to what the room
		// actually holds.
		if (text.trim() !== '' && parse(text) === null) return;
		send(parse(text));
	}

	function handleKeydown(event: KeyboardEvent & { currentTarget: HTMLInputElement }) {
		if (event.key === 'Enter') event.currentTarget.blur();
	}

	function handleFocusIn() {
		focused = true;
		lastSent = undefined;
	}

	function handleFocusOut(event: FocusEvent) {
		const next = event.relatedTarget as Node | null;
		// Focus moving to the step buttons or the by-how-much box is still
		// this control being used, so the panel stays up.
		if (container && next && container.contains(next)) return;
		focused = false;
		amount = '';
	}

	// A click is already a finished intent, the way a blur or an Enter is,
	// so a step commits on the spot. Waiting for a blur would mean the
	// first of "−7, then −3, then −5" never landed.
	function step(direction: 1 | -1) {
		const next = (parse(draft) ?? 0) + direction * (parse(amount) ?? 1);
		draft = next;
		send(next);
	}

	const by = $derived(parse(amount) ?? 1);
</script>

<div class="relative" bind:this={container} onfocusin={handleFocusIn} onfocusout={handleFocusOut}>
	<!-- for/id would be the usual pairing, but three of these share a
	     panel and the ids would have to be threaded through the caller.
	     aria-label carries the same name and is what the specs match on. -->
	<span class="mb-0.5 block truncate text-center text-xs text-muted-foreground">{label}</span>
	<!-- md:text-lg is repeated deliberately: the shared input drops to
	     text-sm from md up, which would undo the whole point of this on a
	     desktop, where the reader is furthest from the screen. -->
	<Input
		bind:ref={referenceEl}
		type="number"
		inputmode="numeric"
		aria-label={ariaLabel}
		placeholder="—"
		bind:value={draft}
		onchange={handleChange}
		onkeydown={handleKeydown}
		class="h-11 [appearance:textfield] px-1 text-center font-mono text-lg md:text-lg [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
	/>

	{#if focused}
		<div
			bind:this={floatingEl}
			class="absolute z-50 flex w-max items-center gap-1 rounded-md border bg-popover p-1 shadow-md"
		>
			<!-- preventDefault on mousedown keeps focus in the box, which
			     both holds this panel open for a second click and stops a
			     blur from firing a change event mid-step. -->
			<!-- The name stays put while the tooltip carries the amount: a
			     button that renames itself to "Decrease HP by 5" as someone
			     types into the box beside it is read out again on every
			     keystroke, and gives tests a moving target. -->
			<Button
				variant="outline"
				size="icon-sm"
				title="Decrease by {by}"
				aria-label="Decrease {label}"
				onmousedown={(e) => e.preventDefault()}
				onclick={() => step(-1)}
			>
				<MinusIcon />
			</Button>
			<Input
				type="number"
				inputmode="numeric"
				aria-label="Adjust {label} by"
				placeholder="1"
				step="1"
				bind:value={amount}
				class="h-8 w-16 [appearance:textfield] px-1 text-center font-mono text-sm md:text-sm [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
			/>
			<Button
				variant="outline"
				size="icon-sm"
				title="Increase by {by}"
				aria-label="Increase {label}"
				onmousedown={(e) => e.preventDefault()}
				onclick={() => step(1)}
			>
				<PlusIcon />
			</Button>
		</div>
	{/if}
</div>
