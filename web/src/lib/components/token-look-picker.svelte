<script lang="ts">
	// What a token looks like: its art, or an icon on a coloured disc for the
	// table that didn't bring any. Shared by New token, the edit dialog and
	// a party member's form, so the three can't drift.
	//
	// Native controls only — a search box, buttons, radio swatches — because
	// all three hosts are dialogs, and a popover inside a dialog comes out
	// unpositioned and under the overlay (seats-dialog.svelte has the detail).
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import AssetPicker from '$lib/components/asset-picker.svelte';
	import { IDENTITY_COLORS } from '$lib/identity-color';
	import { searchTokenIcons } from '$lib/token-icons';

	let {
		imageAssetId = $bindable(null),
		icon = $bindable(''),
		color = $bindable(''),
		roomSlug,
		sessionToken,
		idPrefix
	}: {
		imageAssetId?: string | null;
		/** A TOKEN_ICONS key, or '' for none. */
		icon?: string;
		/** An IDENTITY_COLORS key, or '' for the colour the token's id picks. */
		color?: string;
		roomSlug: string;
		sessionToken: string;
		idPrefix: string;
	} = $props();

	// Which half is showing. Starts on whatever the token already has, so
	// editing an icon token opens on its icon.
	let mode = $state<'art' | 'icon'>(icon && !imageAssetId ? 'icon' : 'art');
	let query = $state('');
	const matches = $derived(searchTokenIcons(query));

	// Art wins wherever a token has both, so choosing to show an icon means
	// letting go of the art — otherwise the switch would appear to do
	// nothing. Going the other way keeps the icon: it's invisible under the
	// art, and comes back if the art is cleared.
	function showIcon() {
		mode = 'icon';
		imageAssetId = null;
	}
</script>

<div class="flex flex-col gap-2">
	<Label>Look</Label>
	<div class="flex gap-2">
		<Button
			type="button"
			variant={mode === 'art' ? 'default' : 'outline'}
			aria-pressed={mode === 'art'}
			onclick={() => (mode = 'art')}
			class="flex-1"
		>
			Art
		</Button>
		<Button
			type="button"
			variant={mode === 'icon' ? 'default' : 'outline'}
			aria-pressed={mode === 'icon'}
			onclick={showIcon}
			class="flex-1"
		>
			Icon
		</Button>
	</div>

	{#if mode === 'art'}
		<AssetPicker
			{roomSlug}
			{sessionToken}
			{idPrefix}
			kind="token"
			lockKind
			bind:selectedId={imageAssetId}
			emptyHint="Nothing in the library yet — add art on the assets page, or choose an icon."
		/>
	{:else}
		<Input
			id="{idPrefix}-icon-search"
			bind:value={query}
			placeholder="Search icons, like undead or wolf"
			aria-label="Search icons"
			autocomplete="off"
		/>
		{#if matches.length === 0}
			<p class="text-sm text-muted-foreground">No icons match that.</p>
		{:else}
			<!-- Fixed height and scrolling, so the dialog doesn't jump about as
			     the search narrows. -->
			<div class="grid max-h-36 grid-cols-8 gap-1 overflow-y-auto rounded border p-1">
				{#each matches as match (match.key)}
					<button
						type="button"
						class={[
							'flex aspect-square items-center justify-center rounded border',
							icon === match.key
								? 'border-primary bg-primary text-primary-foreground'
								: 'border-transparent hover:bg-muted'
						]}
						aria-label={match.label}
						aria-pressed={icon === match.key}
						title={match.label}
						onclick={() => (icon = icon === match.key ? '' : match.key)}
					>
						<match.icon class="h-4 w-4" />
					</button>
				{/each}
			</div>
		{/if}

		<fieldset class="flex flex-col gap-1">
			<legend class="mb-1 text-sm font-medium">Colour</legend>
			<div class="flex flex-wrap gap-1.5">
				<!-- First, because it's what every token had before colours
				     could be chosen: the colour its own id picks. -->
				<!-- Each radio is invisible and laid over its whole swatch, rather
				     than shrunk to a screen-reader-only pixel: a click anywhere on
				     the swatch is then a click on the radio itself, for a pointer,
				     a keyboard and a test driver alike. -->
				<label
					class="relative flex h-7 cursor-pointer items-center rounded-full border px-2 text-xs has-[:checked]:ring-2 has-[:checked]:ring-primary has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
				>
					<input
						type="radio"
						class="absolute inset-0 cursor-pointer appearance-none rounded-full opacity-0"
						name="{idPrefix}-color"
						value=""
						bind:group={color}
					/>
					Any
				</label>
				{#each IDENTITY_COLORS as c (c.key)}
					<label
						class="relative h-7 w-7 cursor-pointer rounded-full border has-[:checked]:ring-2 has-[:checked]:ring-primary has-[:checked]:ring-offset-2 has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-ring"
						style="background-color: {c.hex};"
						title={c.label}
					>
						<input
							type="radio"
							class="absolute inset-0 cursor-pointer appearance-none rounded-full opacity-0"
							name="{idPrefix}-color"
							value={c.key}
							aria-label={c.label}
							bind:group={color}
						/>
					</label>
				{/each}
			</div>
		</fieldset>
	{/if}
</div>
