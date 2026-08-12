<script lang="ts">
	// The contextual strip: whatever the active tool family needs, and
	// nothing belonging to any other family. Hand and ping render nothing
	// at all — an empty bar still costs a row and still covers the map.
	//
	// Placed by the room page rather than by the toolbar, because where it
	// goes depends on the viewport: floating under the tool row on a
	// desktop, docked into the bottom sheet on a phone. Draw's strip is
	// borderline at 375px and measure's doesn't fit, and a horizontally
	// scrolling bar floating over a pannable canvas is a gesture conflict.
	import { Button } from '$lib/components/ui/button';
	import { Slider } from '$lib/components/ui/slider';
	import type { RoomClient } from '$lib/room.svelte';
	import { LINE_WIDTH_CHOICES_FEET, type SnapMode } from '$lib/aoe';
	import { MAX_FOG_OPACITY, MIN_FOG_OPACITY } from '$lib/fog-opacity';
	import { familyOf, type Tool } from '$lib/tool-family';
	import Pen from '@lucide/svelte/icons/pen';
	import Slash from '@lucide/svelte/icons/slash';
	import RectangleHorizontal from '@lucide/svelte/icons/rectangle-horizontal';
	import Circle from '@lucide/svelte/icons/circle';
	import Eraser from '@lucide/svelte/icons/eraser';
	import Ruler from '@lucide/svelte/icons/ruler';
	import CircleDot from '@lucide/svelte/icons/circle-dot';
	import Cone from '@lucide/svelte/icons/cone';
	import Minus from '@lucide/svelte/icons/minus';
	import Square from '@lucide/svelte/icons/square';
	import Eye from '@lucide/svelte/icons/eye';
	import EyeOff from '@lucide/svelte/icons/eye-off';

	let {
		room,
		sceneId,
		activeTool = $bindable('none'),
		strokeColor = $bindable('#000000'),
		snapMode = $bindable('intersections'),
		lineWidthFeet = $bindable(),
		fogOpacity = $bindable(0.5),
		isGM
	}: {
		room: RoomClient;
		sceneId: string;
		activeTool?: Tool;
		strokeColor?: string;
		snapMode?: SnapMode;
		lineWidthFeet?: number;
		fogOpacity?: number;
		isGM: boolean;
	} = $props();

	const family = $derived(familyOf(activeTool));

	const DRAW_VARIANTS: { value: Tool; label: string; title: string; icon: typeof Pen }[] = [
		{ value: 'freehand', label: 'Freehand', title: 'Freehand drawing', icon: Pen },
		{ value: 'line', label: 'Line', title: 'Straight line drawing', icon: Slash },
		{ value: 'rect', label: 'Rectangle', title: 'Rectangle drawing', icon: RectangleHorizontal },
		{ value: 'ellipse', label: 'Ellipse', title: 'Ellipse drawing', icon: Circle }
	];

	// Six shapes in the rules, but Sphere, Cylinder and Emanation are all
	// a circle seen from above, so they share one tool.
	const MEASURE_VARIANTS: { value: Tool; label: string; title: string; icon: typeof Pen }[] = [
		// "Distance" rather than "Measure": the family button on the tool
		// row is already called Measure, and two controls with the same
		// accessible name in the same view is ambiguous for anything
		// selecting by name — a screen reader and a test runner both.
		{
			value: 'measure',
			label: 'Distance',
			title: 'Drag to measure a distance in feet',
			icon: Ruler
		},
		{
			value: 'template-circle',
			label: 'Circle template',
			title: 'Sphere, cylinder or emanation — drag from the centre',
			icon: CircleDot
		},
		{
			value: 'template-cone',
			label: 'Cone template',
			title: 'Cone — drag from the point of origin',
			icon: Cone
		},
		{
			value: 'template-line',
			label: 'Line template',
			title: 'Line — drag its length; set its width here',
			icon: Minus
		},
		{
			value: 'template-cube',
			label: 'Cube template',
			title: 'Cube — drag one corner to the opposite corner',
			icon: Square
		}
	];

	const SNAP_MODES: { value: SnapMode; label: string }[] = [
		{ value: 'intersections', label: 'Corners' },
		{ value: 'centres', label: 'Centres' },
		{ value: 'free', label: 'Free' }
	];

	const STROKE_COLORS = [
		{ label: 'Black', value: '#000000' },
		{ label: 'Red', value: '#cc0000' },
		{ label: 'Green', value: '#008000' },
		{ label: 'Blue', value: '#0033cc' }
	];
</script>

{#snippet variant(v: { value: Tool; label: string; title: string; icon: typeof Pen })}
	<Button
		variant={activeTool === v.value ? 'default' : 'outline'}
		size="sm"
		aria-label={v.label}
		aria-pressed={activeTool === v.value}
		title={v.title}
		onclick={() => (activeTool = v.value)}
	>
		<v.icon class="h-4 w-4" />
	</Button>
{/snippet}

{#if family === 'draw'}
	<div
		aria-label="Draw options"
		class="flex flex-wrap items-center gap-2 rounded-md border bg-background/95 p-1 shadow-sm"
	>
		{#each DRAW_VARIANTS as v (v.value)}
			{@render variant(v)}
		{/each}
		<!-- The eraser is a draw-family variant rather than a tool of its
		     own: same gesture, same objects. It reaches different drawings
		     per role — a GM erases anyone's, a Player only their own. -->
		<Button
			variant={activeTool === 'eraser' ? 'default' : 'outline'}
			size="sm"
			aria-label="Erase"
			aria-pressed={activeTool === 'eraser'}
			title={isGM ? 'Click a drawing to erase it' : 'Click one of your own drawings to erase it'}
			onclick={() => (activeTool = 'eraser')}
		>
			<Eraser class="h-4 w-4" />
		</Button>
		<!-- The selected swatch is ringed rather than bordered: an outline
		     is drawn outside the element, so it never covers the colour it
		     is marking and can't sit between the swatch and the pointer.
		     Light blue reads against every colour in the palette, black
		     included, and against the strip. -->
		<div class="flex items-center gap-2 border-l pl-2">
			{#each STROKE_COLORS as opt (opt.value)}
				<button
					type="button"
					aria-label={opt.label}
					aria-pressed={strokeColor === opt.value}
					title={opt.label}
					class={[
						'h-6 w-6 rounded-full',
						strokeColor === opt.value && 'outline-2 outline-offset-2 outline-sky-400'
					]}
					style="background-color: {opt.value}"
					onclick={() => (strokeColor = opt.value)}
				></button>
			{/each}
		</div>
	</div>
{:else if family === 'measure'}
	<div
		aria-label="Measure options"
		class="flex flex-wrap items-center gap-2 rounded-md border bg-background/95 p-1 shadow-sm"
	>
		{#each MEASURE_VARIANTS as v (v.value)}
			{@render variant(v)}
		{/each}
		<!-- Snap and line width apply to the templates, not to the plain
		     ruler, so they appear once a template is chosen rather than
		     sitting inert beside the measuring tool. -->
		{#if activeTool !== 'measure'}
			<div class="flex items-center gap-1 border-l pl-2">
				<span class="text-xs text-muted-foreground">Snap to</span>
				{#each SNAP_MODES as mode (mode.value)}
					<Button
						variant={snapMode === mode.value ? 'default' : 'outline'}
						size="sm"
						aria-pressed={snapMode === mode.value}
						onclick={() => (snapMode = mode.value)}
					>
						{mode.label}
					</Button>
				{/each}
			</div>
		{/if}
		{#if activeTool === 'template-line'}
			<div class="flex items-center gap-1 border-l pl-2">
				<span class="text-xs text-muted-foreground">Width</span>
				{#each LINE_WIDTH_CHOICES_FEET as feet (feet)}
					<Button
						variant={lineWidthFeet === feet ? 'default' : 'outline'}
						size="sm"
						aria-label="{feet} foot wide line"
						aria-pressed={lineWidthFeet === feet}
						onclick={() => (lineWidthFeet = feet)}
					>
						{feet} ft
					</Button>
				{/each}
			</div>
		{/if}
	</div>
{:else if family === 'fog'}
	<div
		aria-label="Fog options"
		class="flex flex-wrap items-center gap-2 rounded-md border bg-background/95 p-1 shadow-sm"
	>
		<Button
			variant={activeTool === 'fog-reveal' ? 'default' : 'outline'}
			size="sm"
			aria-label="Reveal fog"
			aria-pressed={activeTool === 'fog-reveal'}
			title="Reveal"
			onclick={() => (activeTool = 'fog-reveal')}
		>
			<Eye class="h-4 w-4" />
		</Button>
		<Button
			variant={activeTool === 'fog-hide' ? 'default' : 'outline'}
			size="sm"
			aria-label="Hide fog"
			aria-pressed={activeTool === 'fog-hide'}
			title="Hide"
			onclick={() => (activeTool = 'fog-hide')}
		>
			<EyeOff class="h-4 w-4" />
		</Button>
		<!-- These two keep their text labels while the tools above became
		     icons. They wipe a whole scene's fog, there is no undo for fog,
		     and as two adjacent unlabelled icons meaning "uncover
		     everything" and "cover everything" they are exactly the pair
		     that gets mis-hit. The text costs ~120px in a strip with room
		     for it.

		     Both are one-shot buttons rather than tools: neither has a
		     gesture to make, so making them modes would mean arming
		     something that fires on the next click anywhere on the map. -->
		<div class="flex items-center gap-2 border-l pl-2">
			<Button variant="outline" size="sm" onclick={() => room.revealAllFog(sceneId)}>
				Reveal all
			</Button>
			<!-- Deliberately not behind a confirmation. The story asks for a
			     single action, and the cost is that a misclick drops a
			     session's worth of revealed fog with no undo — worth fixing
			     the first time someone actually hits it. -->
			<Button variant="outline" size="sm" onclick={() => room.resetFog(sceneId)}>Hide all</Button>
		</div>
		<!-- The GM's own screen only — a Player's cover is always opaque, so
		     this has nothing to control there. Persisted per browser rather
		     than sent anywhere; see $lib/fog-opacity. -->
		<div class="flex items-center gap-2 border-l pl-2">
			<span class="text-xs text-muted-foreground">Fog opacity</span>
			<Slider
				type="single"
				bind:value={fogOpacity}
				min={MIN_FOG_OPACITY}
				max={MAX_FOG_OPACITY}
				step={0.05}
				class="w-24"
				aria-label="Fog opacity"
			/>
		</div>
	</div>
{/if}
