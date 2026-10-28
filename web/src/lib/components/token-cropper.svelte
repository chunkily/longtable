<script lang="ts">
	// Choosing what a piece of token art shows, before it's uploaded.
	//
	// The square is cut out on the server during the re-encode every upload
	// already goes through, so the stored art is square and nothing that
	// draws a token needs to know a crop happened — the same bargain the
	// grid aligner makes for a map's offset. Like the aligner, this works on
	// a local object URL the server has never seen.
	//
	// The crop is in *natural* image pixels and the preview is scaled to fit,
	// so everything on screen goes through `scale` and the arithmetic lives
	// in crop-frame.ts, tested. Mixing the two units up is the bug this kind
	// of component is easiest to ship (grid-zoom.ts has the longer version).
	import { Label } from '$lib/components/ui/label';
	import { Slider } from '$lib/components/ui/slider';
	import {
		clampCrop,
		initialCrop,
		maxZoom,
		moveCrop,
		needsCrop,
		resizeCrop,
		sizeForZoom,
		zoomForSize,
		type CropSquare
	} from '$lib/crop-frame';

	let {
		src,
		crop = $bindable(null),
		// Written here and read only by the parent through bind:ready, which
		// the rule can't see.
		// eslint-disable-next-line no-useless-assignment
		ready = $bindable(false),
		idPrefix
	}: {
		/** Object URL for the file being staged. */
		src: string;
		/** The square to keep, in image pixels — null for art that's already square. */
		crop?: CropSquare | null;
		/**
		 * Whether the picture has loaded and the crop (or its absence) is
		 * decided. Add to library waits on it, so there's no path past the
		 * cropper for art that needs one.
		 */
		ready?: boolean;
		idPrefix: string;
	} = $props();

	let img = $state<HTMLImageElement | null>(null);
	let naturalWidth = $state(0);
	let naturalHeight = $state(0);
	let displayWidth = $state(0);

	const scale = $derived(naturalWidth > 0 ? displayWidth / naturalWidth : 0);
	const zoom = $derived(crop ? zoomForSize(crop.size, naturalWidth, naturalHeight) : 1);

	function measure() {
		if (!img) return;
		naturalWidth = img.naturalWidth;
		naturalHeight = img.naturalHeight;
		// Framed on arrival, so the default is already a choice someone can
		// accept with one click — square art has nothing to choose.
		crop = needsCrop(naturalWidth, naturalHeight) ? initialCrop(naturalWidth, naturalHeight) : null;
		ready = true;
	}

	// Dragging is measured from where it started rather than step by step, so
	// rounding to whole pixels on every move can't make the square drift
	// away from the pointer over a long drag.
	let drag: { pointerX: number; pointerY: number; from: CropSquare } | null = null;

	function startDrag(event: PointerEvent) {
		if (!crop) return;
		// Captured so a drag that leaves the button — which is most drags,
		// since the button moves with the pointer a frame behind — keeps
		// arriving here.
		if (event.currentTarget instanceof HTMLElement) {
			event.currentTarget.setPointerCapture(event.pointerId);
		}
		drag = { pointerX: event.clientX, pointerY: event.clientY, from: crop };
	}

	function continueDrag(event: PointerEvent) {
		if (!drag || scale <= 0) return;
		crop = moveCrop(
			drag.from,
			(event.clientX - drag.pointerX) / scale,
			(event.clientY - drag.pointerY) / scale,
			naturalWidth,
			naturalHeight
		);
	}

	function endDrag() {
		drag = null;
	}

	// The arrow keys move it too: a twentieth of the frame per press, or a
	// single pixel with Shift for the last nudge.
	function handleKey(event: KeyboardEvent) {
		if (!crop) return;
		const step = event.shiftKey ? 1 : Math.max(1, Math.round(crop.size / 20));
		const delta: Record<string, [number, number]> = {
			ArrowLeft: [-step, 0],
			ArrowRight: [step, 0],
			ArrowUp: [0, -step],
			ArrowDown: [0, step]
		};
		const d = delta[event.key];
		if (!d) return;
		event.preventDefault();
		crop = moveCrop(crop, d[0], d[1], naturalWidth, naturalHeight);
	}

	function setZoom(value: number) {
		if (!crop) return;
		crop = resizeCrop(
			crop,
			sizeForZoom(value, naturalWidth, naturalHeight),
			naturalWidth,
			naturalHeight
		);
	}

	// A crop handed in from outside — none today, but cheap to honour — is
	// held to the same bounds as one made here.
	$effect(() => {
		if (crop && naturalWidth > 0) {
			const clamped = clampCrop(crop, naturalWidth, naturalHeight);
			if (clamped.x !== crop.x || clamped.y !== crop.y || clamped.size !== crop.size) {
				crop = clamped;
			}
		}
	});
</script>

<div class="flex flex-col gap-2">
	{#if crop}
		<div class="flex flex-col gap-1">
			<span class="text-sm font-medium">Frame the token</span>
			<span class="text-xs text-muted-foreground">
				Drag the circle to choose what the token shows.
			</span>
		</div>
	{/if}
	<!-- Hidden until it's known whether there's anything to frame, so square
	     art never flashes a cropper it doesn't need. Still loaded either way:
	     the image's own size is how that's found out. -->
	<div
		class={[
			'relative self-start overflow-hidden rounded border',
			!crop && 'pointer-events-none absolute h-0 w-0 opacity-0'
		]}
	>
		<img
			bind:this={img}
			{src}
			alt=""
			draggable="false"
			onload={measure}
			bind:clientWidth={displayWidth}
			class="block max-h-80 max-w-full select-none"
		/>
		{#if crop && scale > 0}
			<!-- The token's own circle, with everything it won't show dimmed —
			     the corners of the square included, since a token is round.
			     A button so it takes focus, has a name and answers the arrow
			     keys without any role-patching. -->
			<button
				type="button"
				aria-label="Move the crop"
				class="absolute cursor-move touch-none rounded-full ring-2 ring-white outline-none focus-visible:ring-4 focus-visible:ring-primary"
				style="left: {crop.x * scale}px; top: {crop.y * scale}px; width: {crop.size *
					scale}px; height: {crop.size * scale}px; box-shadow: 0 0 0 9999px rgb(0 0 0 / 0.55);"
				onpointerdown={startDrag}
				onpointermove={continueDrag}
				onpointerup={endDrag}
				onpointercancel={endDrag}
				onkeydown={handleKey}
			></button>
		{/if}
	</div>
	{#if crop && maxZoom(naturalWidth, naturalHeight) > 1}
		<div class="flex items-center gap-3">
			<Label for="{idPrefix}-zoom" class="text-sm">Zoom</Label>
			<Slider
				id="{idPrefix}-zoom"
				type="single"
				value={zoom}
				onValueChange={setZoom}
				min={1}
				max={maxZoom(naturalWidth, naturalHeight)}
				step={0.01}
				class="w-40"
				aria-label="Zoom"
			/>
		</div>
	{/if}
</div>
