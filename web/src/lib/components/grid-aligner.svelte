<script lang="ts">
	// Lining a map's own squares up with the grid, before it's uploaded.
	//
	// The overlay is drawn over a local object URL, so this is all happening
	// on a file the server has never seen — which is what lets the offset be
	// baked into the pixels during the one re-encode on the way in, rather
	// than stored beside the image and re-applied by every renderer forever.
	//
	// Everything here is in *natural* image pixels; the preview is scaled to
	// fit, so display maths goes through `scale` and nothing else. Mixing the
	// two up is the bug this file is easiest to introduce.
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { frameWidth, previewScale, type Zoom } from '$lib/grid-zoom';

	let {
		src,
		gridSize = $bindable(70),
		originX = $bindable(0),
		originY = $bindable(0),
		idPrefix
	}: {
		/** Object URL for the file being staged. */
		src: string;
		/** Measured pixels per square. */
		gridSize?: number;
		/** Where the art's first grid line sits, in image pixels. */
		originX?: number;
		originY?: number;
		idPrefix: string;
	} = $props();

	let img = $state<HTMLImageElement | null>(null);
	let naturalWidth = $state(0);
	let displayWidth = $state(0);

	// Fit is the old behaviour and stays the default: it's the only view
	// that answers "do the lines keep matching across the whole map?", which
	// is what the square size is set against. The fixed steps are for the
	// other half of the job — judging a few pixels of offset, which fit mode
	// physically cannot show on a map dozens of squares across, where the
	// lines compress into a solid mesh. 100% is native pixel scale, which is
	// as close as the art itself can be read.
	const ZOOM_STEPS: { value: Zoom; label: string }[] = [
		{ value: 'fit', label: 'Fit' },
		{ value: 1, label: '100%' },
		{ value: 2, label: '200%' }
	];
	let zoom = $state<Zoom>('fit');

	// Both in grid-zoom.ts, with the reasoning and the tests: this is
	// display maths, and the component's own doc comment above says why
	// that is the thing to keep at arm's length.
	const frame = $derived(frameWidth(zoom, naturalWidth));
	const scale = $derived(previewScale(zoom, naturalWidth, displayWidth));

	function measure() {
		if (img) naturalWidth = img.naturalWidth;
	}

	// Watching the image covers every reason its width changes with one
	// mechanism: the file loading, a zoom step, the window resizing, the
	// `sm:` breakpoint turning this row from a column back into two, and a
	// scrollbar arriving. Observing the container instead misses that last
	// one, because a scrollbar changes what fits inside the box without
	// changing the box.
	$effect(() => {
		const el = img;
		if (!el) return;

		const observer = new ResizeObserver(() => (displayWidth = el.getBoundingClientRect().width));
		observer.observe(el);
		displayWidth = el.getBoundingClientRect().width;
		return () => observer.disconnect();
	});

	// Dragging the overlay is the gesture that makes this legible — nudging
	// two number fields and looking back at the picture is much harder than
	// pushing the lines onto the art directly. Pointer events (not mouse)
	// so it works under a finger, and capture so a drag that leaves the
	// image still tracks.
	let dragging = $state(false);
	let dragStart = { pointerX: 0, pointerY: 0, originX: 0, originY: 0 };

	function startDrag(event: PointerEvent) {
		dragging = true;
		dragStart = { pointerX: event.clientX, pointerY: event.clientY, originX, originY };
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
	}

	function drag(event: PointerEvent) {
		if (!dragging) return;
		originX = dragStart.originX + (event.clientX - dragStart.pointerX) / scale;
		originY = dragStart.originY + (event.clientY - dragStart.pointerY) / scale;
	}

	function endDrag(event: PointerEvent) {
		dragging = false;
		(event.currentTarget as HTMLElement).releasePointerCapture(event.pointerId);
		// Snapped back into the first square once the drag ends: the pad
		// sent to the server is modulo the square size anyway, so leaving
		// "212" in the field would only look like it meant something.
		originX = wrap(originX);
		originY = wrap(originY);
	}

	function wrap(value: number): number {
		if (!(gridSize > 0)) return 0;
		return ((Math.round(value) % gridSize) + gridSize) % gridSize;
	}

	// Kept in the script rather than inline on the element: it's long
	// enough that the formatter wraps it, and a newline inside a style:
	// directive's value lands as a raw newline in a generated string
	// literal, which fails svelte-check with a hundred errors that all
	// point somewhere else.
	const GRID_LINES =
		'linear-gradient(to right, rgba(56,189,248,0.9) 1px, transparent 1px), linear-gradient(to bottom, rgba(56,189,248,0.9) 1px, transparent 1px)';
</script>

<div class="flex flex-col gap-3 sm:flex-row">
	<div class="flex min-w-0 flex-1 flex-col gap-2">
		<!-- Scrolls rather than shrinks. The preview used to be capped at
		     max-w-xs, which put a map dozens of squares across into a box a
		     few hundred pixels wide and turned its grid into a solid smear —
		     there was no offset anybody could judge against that. -->
		<div class="relative max-h-96 overflow-auto rounded-md border">
			<!-- Sized to the image so the overlays, which are inset-0 against
			     this, cover the art rather than the scroll box. -->
			<div class="relative" style:width={frame}>
				<img
					bind:this={img}
					{src}
					alt="The map being aligned"
					class="block w-full touch-none select-none"
					draggable="false"
					onload={measure}
					onpointerdown={startDrag}
					onpointermove={drag}
					onpointerup={endDrag}
					onpointercancel={endDrag}
				/>
				<!-- Repeating-gradient lines rather than an SVG or a canvas: the
				     browser redraws them on every drag frame for free, and the grid
				     has no state worth owning. Pointer events pass through to the
				     image underneath, which is what handles the drag. -->
				<div
					aria-hidden="true"
					class="pointer-events-none absolute inset-0"
					style:background-image={GRID_LINES}
					style:background-size="{gridSize * scale}px {gridSize * scale}px"
					style:background-position="{wrap(originX) * scale}px {wrap(originY) * scale}px"
				></div>
				<!-- One square picked out, so there's something specific to line up
				     rather than a field of identical lines. -->
				<div
					aria-hidden="true"
					class="pointer-events-none absolute border-2 border-sky-400/90 bg-sky-400/10"
					style:left="{wrap(originX) * scale}px"
					style:top="{wrap(originY) * scale}px"
					style:width="{gridSize * scale}px"
					style:height="{gridSize * scale}px"
				></div>
			</div>
		</div>

		<div class="flex items-center gap-1">
			<span class="mr-1 text-xs text-muted-foreground">Zoom</span>
			{#each ZOOM_STEPS as step (step.label)}
				<Button
					type="button"
					variant={zoom === step.value ? 'secondary' : 'outline'}
					size="sm"
					aria-pressed={zoom === step.value}
					onclick={() => (zoom = step.value)}
				>
					{step.label}
				</Button>
			{/each}
		</div>
	</div>

	<div class="flex min-w-0 flex-col gap-2 sm:w-56 sm:shrink-0">
		<div class="flex flex-col gap-1">
			<Label for="{idPrefix}-grid-size">Square size (px)</Label>
			<Input id="{idPrefix}-grid-size" type="number" min="8" max="1024" bind:value={gridSize} />
		</div>
		<div class="grid grid-cols-2 gap-2">
			<div class="flex flex-col gap-1">
				<Label for="{idPrefix}-origin-x">Offset across</Label>
				<Input
					id="{idPrefix}-origin-x"
					type="number"
					min="0"
					max={gridSize}
					value={wrap(originX)}
					oninput={(e) => (originX = Number(e.currentTarget.value))}
				/>
			</div>
			<div class="flex flex-col gap-1">
				<Label for="{idPrefix}-origin-y">Offset down</Label>
				<Input
					id="{idPrefix}-origin-y"
					type="number"
					min="0"
					max={gridSize}
					value={wrap(originY)}
					oninput={(e) => (originY = Number(e.currentTarget.value))}
				/>
			</div>
		</div>
		<p class="text-xs text-muted-foreground">
			Drag the picture to move the grid onto the map's own squares, then set the square size until
			the lines keep matching across the whole map. Zoom in to check the fit on a large map. The
			image is padded to match when it's added — nothing is cropped.
		</p>
	</div>
</div>
