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
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';

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

	// 1 until the image has loaded and been measured, so the overlay is
	// never drawn against a zero width.
	const scale = $derived(naturalWidth > 0 && displayWidth > 0 ? displayWidth / naturalWidth : 1);

	function measure() {
		if (!img) return;
		naturalWidth = img.naturalWidth;
		displayWidth = img.clientWidth;
	}

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
	<div class="relative max-w-xs shrink-0 overflow-hidden rounded-md border">
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

	<div class="flex min-w-0 flex-1 flex-col gap-2">
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
			the lines keep matching across the whole map. The image is padded to match when it's added —
			nothing is cropped.
		</p>
	</div>
</div>
