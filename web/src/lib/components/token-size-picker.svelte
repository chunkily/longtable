<script lang="ts" module>
	/**
	 * The creature sizes a token can be, and the square each occupies.
	 *
	 * Tiny, Small and Medium all stand in one square, so they are one
	 * option here rather than three. The story lists six names, but a
	 * token only stores width/height — three options that all mean 1×1
	 * couldn't be told apart when reading a token back into this picker,
	 * so editing a "Tiny" token would silently show it as "Medium". One
	 * option carrying all three names says the same thing and round-trips.
	 */
	export const TOKEN_SIZES = [
		{ label: 'Tiny / Small / Medium', squares: 1 },
		{ label: 'Large', squares: 2 },
		{ label: 'Huge', squares: 3 },
		{ label: 'Gargantuan', squares: 4 }
	];

	/** The option a stored width belongs to, defaulting to one square. */
	export function sizeForSquares(squares: number) {
		return TOKEN_SIZES.find((s) => s.squares === squares) ?? TOKEN_SIZES[0];
	}
</script>

<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';

	let {
		squares = $bindable(1),
		idPrefix = 'token'
	}: {
		/** Grid squares on a side. Bindable so a dialog can read it on submit. */
		squares?: number;
		idPrefix?: string;
	} = $props();
</script>

<div class="flex flex-col gap-2">
	<Label for="{idPrefix}-size">Size</Label>
	<div id="{idPrefix}-size" class="flex flex-wrap gap-2">
		{#each TOKEN_SIZES as size (size.squares)}
			<Button
				type="button"
				variant={squares === size.squares ? 'default' : 'outline'}
				size="sm"
				aria-pressed={squares === size.squares}
				aria-label="{size.label} ({size.squares}×{size.squares} squares)"
				onclick={() => (squares = size.squares)}
			>
				{size.label}
				<span class="text-xs opacity-70">{size.squares}×{size.squares}</span>
			</Button>
		{/each}
	</div>
</div>
