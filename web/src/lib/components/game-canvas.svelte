<script lang="ts">
	import { onDestroy, onMount, untrack } from 'svelte';
	import { mode } from 'mode-watcher';
	import Konva from 'konva';
	import { assetUrl } from '$lib/api';
	import { DRAWING_STROKE_WIDTH, isFilled, pickDrawing, strokeWidthOf } from '$lib/drawing-hit';
	import { identityHex } from '$lib/identity-color';
	import { fillFor } from '$lib/drawing-fill';
	import {
		DEFAULT_LINE_WIDTH_FEET,
		circleRadius,
		quantiseTemplateEnd,
		snapPoint,
		templateLabel,
		templatePolygon,
		type SnapMode,
		type TemplateKind
	} from '$lib/aoe';
	import { cellAt, cellCentre, measureLabel, type Cell } from '$lib/measure';
	import { snapTokenCell, tokenDragPreview } from '$lib/token-drag';
	import type { Tool } from '$lib/tool-family';
	import { isPanButton, panStep } from '$lib/pan';
	import { pinchStep, touchCentre, touchDistance, type Point } from '$lib/pinch';
	import { PING_PULSES, PING_PULSE_INTERVAL_MS, PING_PULSE_SECONDS } from '$lib/ping';
	import { fogRuns } from '$lib/fog';
	import { DEFAULT_FOG_OPACITY } from '$lib/fog-opacity';
	import { DEFAULT_STROKE_COLOR } from '$lib/stroke-colors';
	import { setTrackers, trackerText } from '$lib/room.svelte';
	import type {
		Drawing,
		DrawingPoint,
		FogCell,
		Measurement,
		Ping,
		RoomClient,
		Token
	} from '$lib/room.svelte';

	// The tool union lives in $lib/tool-family alongside the rules for
	// grouping it into the toolbar's five families — the canvas only cares
	// which tool is active, never how the toolbar arranges them. Import
	// the type from there rather than from this component.

	// The area templates and the shape each drags out. They share the
	// measuring tool's whole gesture — one per participant, thrown away
	// when the drag ends — so the only thing that varies is this.
	const TEMPLATE_TOOLS: Partial<Record<Tool, TemplateKind>> = {
		'template-circle': 'circle',
		'template-cone': 'cone',
		'template-line': 'line',
		'template-cube': 'cube'
	};

	let {
		room,
		activeTool = 'none',
		strokeColor = DEFAULT_STROKE_COLOR,
		strokeWidth = DRAWING_STROKE_WIDTH,
		shapeFilled = false,
		snapMode = 'intersections',
		lineWidthFeet = DEFAULT_LINE_WIDTH_FEET,
		fogOpacity = DEFAULT_FOG_OPACITY,
		selectedTokenId = $bindable(null)
	}: {
		room: RoomClient;
		activeTool?: Tool;
		strokeColor?: string;
		/**
		 * How wide a new drawing's stroke is, in world pixels — so it keeps
		 * its weight relative to the map rather than to the screen, the same
		 * as the drawing itself. Picked on the draw strip.
		 */
		strokeWidth?: number;
		/**
		 * Whether a new rect or ellipse is shaded inside. Only those two
		 * kinds can be, so the toolbar only offers it for those two and the
		 * server refuses it for the rest.
		 */
		shapeFilled?: boolean;
		/** Where template points may land. Purely a local input aid. */
		snapMode?: SnapMode;
		lineWidthFeet?: number;
		/** The GM's own preference for how dark fog looks on their screen. */
		fogOpacity?: number;
		/**
		 * The token this client has selected, or null. Bound rather than
		 * owned here because the room page draws the details section that
		 * goes with it — and because nothing about a selection goes on the
		 * wire, so two clients can have different tokens selected at once.
		 */
		selectedTokenId?: string | null;
	} = $props();

	const MIN_SCALE = 0.2;
	const MAX_SCALE = 4;
	const ZOOM_STEP = 1.05;
	// Minimum world-space distance between consecutive freehand points —
	// keeps strokes from accumulating an unbounded number of points
	// while the pointer is held down.
	const MIN_FREEHAND_SPACING = 3;
	// How far from a stroke the eraser still grabs it, in *screen*
	// pixels — converted to world units at the current zoom, so the
	// eraser has the same reach whether you're zoomed right in or out,
	// and a thin stroke is no harder to hit than a thick one. This is
	// the radius drawn around the cursor.
	const ERASER_PICK_RADIUS = 12;
	// Halo drawn around the stroke the eraser is about to remove, in
	// screen pixels either side of it.
	const ERASE_HIGHLIGHT_PADDING = 6;
	const ERASE_HIGHLIGHT_COLOR = '#f59e0b';
	// The stage's own furniture, and the only part of the canvas that
	// follows the app's colour scheme. Everything else painted here —
	// strokes, pings, measurements, the eraser's halo — is map content and
	// stays put in both schemes; the dark-map drawing palette is a
	// separate problem with its own backlog item.
	//
	// These two need it because they sit against the container's
	// `bg-muted`, which does flip. A grid ruled in 13%-opacity black
	// vanishes on a dark background, and the light slab shown where a
	// scene has no map is a lit rectangle in the middle of a dark room.
	//
	// Written as explicit pairs rather than read off the CSS custom
	// properties: Konva paints into a canvas, which takes colour strings
	// and has never heard of `var()`.
	const MAP_PLACEHOLDER = { light: '#e4e4e7', dark: '#3f3f46' };
	const GRID_LINE = { light: '#00000022', dark: '#ffffff26' };
	// A map that failed to load is a different thing from a scene with no
	// map at all, and the two have to stay apart in both schemes. This
	// used to be the dark placeholder's exact value, which was fine while
	// the placeholder was always light and would have made the two
	// identical the moment it wasn't. Mid-grey reads as neither.
	const MAP_LOAD_FAILED = '#71717a';
	// Measurement lines are one colour for everyone rather than per
	// participant: they're transient, and at most one per person is on
	// the map at a time with their name on it.
	const MEASURE_COLOR = '#0ea5e9';
	// What a ping is when nobody has an identity colour — every ping's
	// colour until seats could carry one, and still the fallback for a
	// seat that predates them.
	const PING_COLOR = '#f59e0b';
	// Area templates are filled as well as outlined, faintly enough to
	// read the map and the tokens through — the point is to see what a
	// shape covers, not to hide it. A paper cutout you can see through.
	const TEMPLATE_FILL = 'rgba(14, 165, 233, 0.18)';
	// Sized in screen pixels and converted at the current zoom, so a
	// measurement reads the same whether you're zoomed in or out.
	const MEASURE_LINE_WIDTH = 2;
	const MEASURE_END_RADIUS = 4;
	const MEASURE_FONT_SIZE = 13;
	const MEASURE_LABEL_PADDING = 4;
	// How far above the end of the line the label floats, so the pointer
	// isn't sitting on top of the number it's producing.
	const MEASURE_LABEL_OFFSET = 18;
	// The selection ring: two concentric dashed circles sharing one dash
	// pattern, a thicker black one underneath and a thinner pale-yellow
	// one on top, so each is the other's backing. Pale yellow alone
	// disappears into a lit tavern floor and black alone disappears into a
	// night-time dungeon; together they read on both. Order matters — this
	// is the order they're added in, so the black goes down first. The two
	// widths are close on purpose: the black is meant to read as an
	// outline around a yellow dash, so leaving it much wider than the
	// yellow turns the whole ring black at a glance.
	const SELECTION_RING_LAYERS = [
		{ color: '#000000', width: 5 },
		{ color: '#fde68a', width: 3 }
	];
	// Ring geometry in screen pixels, converted at the current zoom: how
	// far outside the token the ring sits, and the dash/gap that rotating
	// it makes legible.
	const SELECTION_RING_PADDING = 6;
	// The dash is a *target* period and the share of it that is ink, not
	// two fixed lengths, because a fixed dash almost never divides evenly
	// into a circumference. At a 70px grid the ring is r=41, so its
	// circumference is 257.6 and a 12px period fits 21.47 times: the last
	// dash landed 0.6px from the first instead of 7px, leaving one pair of
	// dashes visibly touching — and the group rotates, so that seam
	// orbited the token. Fitting a whole number of periods removes it at
	// every radius and every zoom.
	const SELECTION_RING_DASH_PERIOD = 12;
	const SELECTION_RING_DASH_INK = 5 / 12;
	// One full turn, slow on purpose. It has to say "this is the one" from
	// the corner of the eye without competing with anything actually
	// happening on the map.
	const SELECTION_RING_PERIOD_MS = 14000;
	// How long a token takes to slide to a new square. Fixed rather than
	// scaled by distance: a move of twenty squares travelling twenty times
	// slower reads as a different kind of event, and what this is for is
	// letting the eye follow *which* token moved and roughly where from —
	// which a fifth of a second does at any distance. Short enough that
	// nobody is waiting on it, long enough to be followed.
	const TOKEN_MOVE_SECONDS = 0.22;
	// How solid the ghost left behind at a token's starting square is
	// while it's being dragged. Faint enough to read as a memory of where
	// the token was rather than as a second token, but not so faint that
	// it disappears into a busy map — which is the whole thing it's there
	// to be measured against.
	const TOKEN_GHOST_OPACITY = 0.35;
	// The hover card that shows a token's trackers and conditions. Screen
	// pixels like everything else that has to read the same at any zoom.
	const HOVER_FONT_SIZE = 13;
	const HOVER_LABEL_PADDING = 6;
	// Clear of the token's top edge, so the card doesn't cover the art of
	// the thing it's describing.
	const HOVER_LABEL_OFFSET = 6;
	const HOVER_FILL = 'rgba(24, 24, 27, 0.92)';

	let container: HTMLDivElement;
	let stage: Konva.Stage | undefined;
	let mapLayer: Konva.Layer;
	let gridLayer: Konva.Layer;
	let fogLayer: Konva.Layer;
	let drawingLayer: Konva.Layer;
	let tokenLayer: Konva.Layer;
	let pingLayer: Konva.Layer;
	let measureLayer: Konva.Layer;
	let previewLayer: Konva.Layer;
	let selectionLayer: Konva.Layer;
	let hoverLayer: Konva.Layer;
	let resizeObserver: ResizeObserver | undefined;

	// Tracks the active scene so a switch to a different scene resets
	// the camera, rather than carrying over an unrelated pan/zoom.
	let lastSceneId: string | null = null;

	// Which half of MAP_PLACEHOLDER/GRID_LINE the render functions read.
	//
	// Deliberately a plain variable rather than `$state` or a read of
	// `mode.current` where it's used. The render functions run inside
	// several different effects, and a reactive read in one of them would
	// give every one of those effects a dependency on the theme — the same
	// shape as the bug in resetView() below, where a read on the way past
	// made clicking a token rebuild the map. One effect owns this and
	// re-renders explicitly; nothing else has to know.
	//
	// Seeded from the current scheme rather than defaulting to light, so
	// a browser that is already dark renders once. Defaulting meant the
	// first paint was light and the effect below immediately re-rendered
	// over it — two renderMap calls, two image loads of the same URL in
	// flight at once, and whichever finished last winning.
	let stageScheme: 'light' | 'dark' = mode.current === 'dark' ? 'dark' : 'light';

	// Imperative Konva bookkeeping, not template state — doesn't need to
	// be a SvelteMap since nothing reactive reads it.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const imageCache = new Map<string, HTMLImageElement>();

	function loadImage(src: string): Promise<HTMLImageElement> {
		const cached = imageCache.get(src);
		if (cached) return Promise.resolve(cached);
		return new Promise((resolve, reject) => {
			const img = new Image();
			img.onload = () => {
				imageCache.set(src, img);
				resolve(img);
			};
			img.onerror = reject;
			img.src = src;
		});
	}

	// Deterministic color per token so untitled/imageless tokens are at
	// least visually distinct from each other.
	function colorFor(id: string): string {
		let hash = 0;
		for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
		return `hsl(${hash % 360}, 65%, 55%)`;
	}

	onMount(() => {
		// Konva ships with `dragButtons: [0, 1]`, so out of the box the
		// *middle* button drags anything draggable — the stage, and any
		// token under the pointer — alongside the left one. That was never
		// asked for, and it collides head-on with the middle-button pan
		// below: both would run on the same press and the map would travel
		// at twice the speed of the hand. Narrowing it to the left button
		// leaves Konva's own drag doing exactly what the rest of this file
		// assumes it does, and leaves right and middle to the pan handlers,
		// which work whatever tool is active.
		//
		// Global rather than per-node — Konva reads it off the singleton at
		// drag time — which is why it is set here, once, before any node
		// exists to have captured the old value.
		Konva.dragButtons = [0];

		stage = new Konva.Stage({
			container,
			width: container.clientWidth,
			height: container.clientHeight,
			draggable: true
		});
		mapLayer = new Konva.Layer();
		gridLayer = new Konva.Layer({ listening: false });
		fogLayer = new Konva.Layer();
		drawingLayer = new Konva.Layer({ listening: false });
		tokenLayer = new Konva.Layer();
		pingLayer = new Konva.Layer({ listening: false });
		measureLayer = new Konva.Layer({ listening: false });
		previewLayer = new Konva.Layer({ listening: false });
		// The selection ring gets a layer of its own because it is animated:
		// a Konva.Animation redraws its whole layer every frame for as long
		// as it runs, and putting a 60fps redraw on the token layer is the
		// same mistake that produced the token-drag and erasing lag bugs.
		// Appended last, so the existing layer indices the e2e specs read
		// pixels from don't move.
		selectionLayer = new Konva.Layer({ listening: false });
		// Topmost, and appended last for the same reason the selection layer
		// was: the e2e specs index layers by number. It has a layer of its
		// own rather than living on the token layer because renderTokens
		// destroys that layer wholesale on any change to room.tokens — a
		// card built there would blink out every time anyone moved anything.
		hoverLayer = new Konva.Layer({ listening: false });
		stage.add(
			mapLayer,
			gridLayer,
			fogLayer,
			drawingLayer,
			tokenLayer,
			pingLayer,
			measureLayer,
			previewLayer,
			selectionLayer,
			hoverLayer
		);

		stage.on('wheel', handleWheel);
		// Namespaced and bound once here rather than in
		// attachToolHandlers, which tears down every `.tool` handler each
		// time the tool changes. A pinch has to work whatever is selected,
		// including nothing.
		stage.on('touchmove.pinch', handlePinchMove);
		stage.on('touchend.pinch touchcancel.pinch', handlePinchEnd);
		// Right- and middle-button panning. Bound once here rather than in
		// attachToolHandlers for the same reason the pinch handlers are:
		// that function tears down every `.tool` handler on each tool
		// change, and dragging the map with the right button has to work
		// whatever is selected — that is the entire point of it.
		//
		// **Plain DOM listeners rather than stage.on(...), and that is a
		// fix rather than a preference.** Konva stops dispatching
		// mousemove to stage listeners entirely while any node is being
		// dragged — `Stage._pointermove` returns early on
		// `Konva.isDragging()` unless `hitOnDragEnabled`, which is off by
		// default and turning it on would put hit-testing back on every
		// frame of every drag. Bound the Konva way, the pan therefore did
		// nothing from the moment a token was picked up: the press
		// registered and not one move followed. That is precisely when
		// reaching for the map is most useful — the square you want is off
		// the screen, which is *why* you are still holding the token. The
		// DOM knows nothing about Konva's drag state, so this route keeps
		// working.
		//
		// Capture phase, because Konva binds its own listeners on
		// `stage.content`, a child of this container. Capture runs
		// outer-to-inner, so these land before all of it — which is the
		// ordering a pan needs and used to get from being registered first:
		// a tool's mousemove reads getRelativePointerPosition() and has to
		// see the translation this frame's pan has already applied.
		container.addEventListener('mousedown', handlePanStart, true);
		container.addEventListener('mousemove', handlePanMove, true);
		container.addEventListener('mouseup', handlePanEnd, true);
		// No mouseup arrives once the button is released outside the canvas,
		// the same gap mouseleave.tool covers for a held tool gesture — and
		// a pan left running would then follow the pointer back in on a
		// button nobody is holding any more. On the container itself, so it
		// needs no capture flag: this element is the event's target.
		container.addEventListener('mouseleave', () => handlePanEnd());
		// The browser's own menu would otherwise land at the end of every
		// right-drag, on top of the map that had just finished moving. It is
		// suppressed over the whole map rather than only after a drag that
		// actually travelled: "sometimes there's a menu" is harder to
		// explain than "there isn't one", and a right-click here means pan
		// now. If a right-click menu on a token ever arrives it replaces
		// this, rather than sharing the button with it.
		//
		// A plain DOM listener on our own container rather than
		// `stage.on('contextmenu')`. Konva routes that event through
		// `getIntersection` before firing it, so a Konva handler only runs
		// once hit-testing across the listening layers has agreed on a
		// target — a dependency this has no use for. Suppressing a browser
		// default is a property of a region of the page, not of whatever
		// shape happens to be under the pointer, so it belongs on the
		// element. No removal needed: the listener goes with the node.
		container.addEventListener('contextmenu', (e) => e.preventDefault());
		stage.on('dragmove', () => renderGrid());
		// A pointer that leaves the canvas entirely never gets a mouseleave
		// from the token it was over — the same gap mouseleave.tool covers
		// for held gestures.
		stage.on('mouseleave', () => (hoveredTokenId = null));

		// applyViewChange rather than renderGrid alone: the stage now fills
		// the window, so it resizes for reasons that aren't a window drag —
		// the mobile sheet opening, the contextual strip appearing under
		// the toolbar. Those move the viewport over the world exactly as a
		// pan does, and everything authored in screen pixels has to be
		// re-rendered afterwards or it keeps the size it was given for the
		// old box. Same contract a zoom follows; see canvas.md.
		resizeObserver = new ResizeObserver(() => {
			if (!stage) return;
			stage.width(container.clientWidth);
			stage.height(container.clientHeight);
			applyViewChange();
		});
		resizeObserver.observe(container);

		// Deliberately no render() here. The $effect below runs on mount
		// as well, and because effects run in creation order — this
		// onMount is declared above it — it runs after the stage exists.
		// Calling render() here too ran it twice in the same flush, and
		// renderMap clears synchronously but adds after an await, so both
		// clears landed before either add: the map layer kept two
		// identical copies of the bitmap for the rest of the session, and
		// redrew both on every pan, zoom and fog change. See
		// planning/backlog/duplicate-map-image.md.
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		for (const marker of pingMarkers.values()) destroyPingMarker(marker);
		pingMarkers.clear();
		// Before the stage goes: a running Konva.Animation would otherwise
		// keep asking a destroyed layer to redraw itself every frame, and a
		// token still sliding would go on setting attributes on nodes that
		// no longer exist.
		clearSelectionRing();
		for (const id of [...moveTweens.keys()]) stopTokenMove(id);
		stage?.destroy();
	});

	// Standard Konva "zoom to pointer" recipe: keep the world point under
	// the cursor fixed on screen while scaling around it.
	function handleWheel(e: Konva.KonvaEventObject<WheelEvent>) {
		e.evt.preventDefault();
		if (!stage) return;

		const oldScale = stage.scaleX();
		const pointer = stage.getPointerPosition();
		if (!pointer) return;

		const mousePointTo = {
			x: (pointer.x - stage.x()) / oldScale,
			y: (pointer.y - stage.y()) / oldScale
		};
		const direction = e.evt.deltaY > 0 ? -1 : 1;
		const newScale = Math.min(
			MAX_SCALE,
			Math.max(MIN_SCALE, direction > 0 ? oldScale * ZOOM_STEP : oldScale / ZOOM_STEP)
		);

		stage.scale({ x: newScale, y: newScale });
		stage.position({
			x: pointer.x - mousePointTo.x * newScale,
			y: pointer.y - mousePointTo.y * newScale
		});
		applyViewChange();
	}

	// Where a right- or middle-button pan began: the pointer and the
	// stage's translation as they were at the press, both in screen
	// pixels. Null when no pan is in flight, which is also the flag the
	// move handler tests — one piece of state rather than a separate
	// boolean that could disagree with it.
	let panPointerStart: Point | null = null;
	let panStageStart: Point | null = null;

	// Dragging the map with the right or middle button, which works
	// whatever tool is selected. The left button is spoken for — a tool
	// owns it while one is active, and Konva's own stage drag has it when
	// none is — so panning with a ruler or a pen in hand needs a button
	// nothing else wants. This is also the only reason the right button
	// exists on this map: nothing else uses it, by the decision in
	// planning/backlog/no-draw-on-right-click.md.
	//
	// **It deliberately runs during a left-button gesture too**, which is
	// what lets a ruler or a rectangle be dragged past the edge of the
	// screen: hold the right button as well, shove the map along, let go,
	// and carry on pulling. The gesture underneath survives untouched
	// without anything here defending it, and the reason is worth keeping
	// — a pan moves the stage by exactly the distance the pointer moved,
	// so the *world* point under the cursor doesn't change while both
	// buttons are down. The tool's own mousemove keeps firing and keeps
	// arriving at the same answer: the far end stays anchored where it was
	// and the map slides underneath it. Retracting the gesture (the way a
	// second finger does for a pinch) would throw away work the pointer is
	// still in the middle of; a pinch has no choice, a spare button does.
	// These run before Konva's own dispatch (capture phase, see onMount),
	// so `setPointersPositions` has not been called for this event yet and
	// `getPointerPosition()` would answer with the previous one. Calling it
	// here is the public way to sample the pointer early; Konva calls it
	// again on the way past, with the same event and the same answer.
	function panPointer(e: MouseEvent): Point | null {
		if (!stage) return null;
		stage.setPointersPositions(e);
		return stage.getPointerPosition();
	}

	function handlePanStart(e: MouseEvent) {
		if (!stage || !isPanButton(e.button)) return;

		const pointer = panPointer(e);
		if (!pointer) return;

		// Stops the middle button's autoscroll and the text selection a
		// drag over the page would otherwise start. Harmless for the right
		// button, whose own default — the context menu — is a separate
		// event, suppressed where it is bound.
		e.preventDefault();

		panPointerStart = pointer;
		panStageStart = stage.position();
		// The only cursor this component sets. A tool's cursor is drawn on
		// the preview layer as a ring; this one has no reach to show and
		// every reason to say the map itself is moving.
		stage.container().style.cursor = 'grabbing';
	}

	function handlePanMove(e: MouseEvent) {
		if (!stage || !panPointerStart || !panStageStart) return;
		// getPointerPosition, never getRelativePointerPosition: the
		// relative one is the pointer put through the inverse of the stage
		// transform, so feeding it back into that same transform makes each
		// frame's delta depend on the translation the last frame set, and
		// the map accelerates away under the hand. See $lib/pan.
		const pointer = panPointer(e);
		if (!pointer) return;

		stage.position(panStep({ origin: panStageStart, from: panPointerStart, to: pointer }));
		// Deliberately not applyViewChange(). A pan moves the viewport over
		// the world without touching the scale, so everything authored in
		// screen pixels — measurement labels, the selection ring, the
		// eraser's halo — still means what it did and rides along on its
		// own layer. Only the grid has to be rebuilt, because it is
		// generated for the visible region rather than the whole world.
		// This is the same trade Konva's own stage drag makes, above.
		stage.batchDraw();
		renderGrid();
	}

	// Ends only when a *panning* button comes up. A left release has to
	// pass straight through: it belongs to the tool gesture that may be
	// running underneath, and ending the pan on it would drop the map
	// half-way through a shove that is still being made.
	//
	// Called unguarded from mouseleave as well, where there is no button
	// to inspect and no mouseup coming.
	function handlePanEnd(e?: MouseEvent) {
		if (e && !isPanButton(e.button)) return;
		if (!panPointerStart) return;
		panPointerStart = null;
		panStageStart = null;
		if (stage) stage.container().style.cursor = '';
	}

	// Where the pinch was last sampled, or null when fewer than two
	// fingers are down. Both are cleared on touchend: lifting one finger
	// of two leaves the other mid-gesture, and a stale distance would
	// make the next touch jump the scale by whatever it happened to hold.
	let pinchCentre: Point | null = null;
	let pinchDistance = 0;

	// Touch positions arrive in client coordinates, and the stage's own
	// pointer helpers only ever track one of them — so the two fingers
	// have to be read off the raw event and offset by the container's box
	// by hand.
	function touchPoint(touch: Touch, box: DOMRect): Point {
		return { x: touch.clientX - box.left, y: touch.clientY - box.top };
	}

	function handlePinchMove(e: Konva.KonvaEventObject<TouchEvent>) {
		const touches = e.evt.touches;
		if (!stage || touches.length < 2) return;

		// Only once two fingers are down, so a one-finger pan still scrolls
		// the page's own gestures normally on the way in.
		e.evt.preventDefault();

		const box = stage.container().getBoundingClientRect();
		const a = touchPoint(touches[0], box);
		const b = touchPoint(touches[1], box);
		const centre = touchCentre(a, b);
		const distance = touchDistance(a, b);

		if (!pinchCentre) {
			// The second finger has just landed. Konva is most likely
			// dragging the stage from the first one; left running, the map
			// pans and scales at once and slides out from under the hands.
			stage.stopDrag();
			// A tool owns the pointer, and a pinch is two of them — the
			// second touch reads as more of the same stroke otherwise. The
			// gesture is abandoned rather than committed: a half-drawn line
			// that appears because someone reached to zoom isn't something
			// they asked for, and undo is a poor answer on a tablet.
			retractInFlightGesture();
			pinchCentre = centre;
			pinchDistance = distance;
			return;
		}

		const next = pinchStep({
			scale: stage.scaleX(),
			position: stage.position(),
			from: pinchCentre,
			to: centre,
			ratio: pinchDistance > 0 ? distance / pinchDistance : 1,
			minScale: MIN_SCALE,
			maxScale: MAX_SCALE
		});

		stage.scale({ x: next.scale, y: next.scale });
		stage.position(next.position);
		pinchCentre = centre;
		pinchDistance = distance;
		applyViewChange();
	}

	function handlePinchEnd(e: Konva.KonvaEventObject<TouchEvent>) {
		if (e.evt.touches.length >= 2) return;
		pinchCentre = null;
		pinchDistance = 0;
	}

	// Everything that has to happen after the stage's scale or position
	// changes, for any reason.
	//
	// These are re-renders rather than redraws on purpose. Measurement
	// lines and labels, the selection ring's stroke and dashes, the hover
	// card and the eraser's halo are all authored in screen pixels and
	// converted through screenToWorld() at render time, so a change of
	// scale changes what they should be in world units. A handler that
	// calls batchDraw() and stops leaves every one of them the wrong
	// size, which reads as a rendering bug rather than as a missing call
	// — see canvas.md, which writes the contract down.
	function applyViewChange() {
		stage?.batchDraw();
		renderGrid();
		renderMeasurements();
		renderSelection();
		renderHoverCard();
		refreshCursorOverlay();
		// The ghost is a token and lives in world units, so it looks after
		// itself — but the line's width and dash and the label's type are
		// all screen pixels, and the wheel still zooms while a token is
		// being held. A right-button pan deliberately isn't on this path
		// (it calls renderGrid alone) and doesn't need to be: it changes
		// the translation only, so nothing here has changed size.
		updateTokenDragPreview();
	}

	// Resets the camera to its identity transform (pan at the origin,
	// no zoom) — the same view a scene starts in, since the map's
	// origin (0,0) is always its top-left corner. Exposed for the
	// "Reset view" button in the room page via bind:this.
	export function resetView() {
		if (!stage) return;
		stage.scale({ x: 1, y: 1 });
		stage.position({ x: 0, y: 0 });
		// Resetting from 2x back to 1x is a scale change like any other, so
		// everything sized in screen pixels has to be re-rendered — this
		// used to redraw the grid alone, which left a selection ring
		// carrying the stroke width it had been given at the old zoom.
		//
		// `untrack` is load-bearing rather than tidy. This runs inside
		// render()'s effect, before its first await and therefore inside
		// the window Svelte is collecting dependencies in, and
		// applyViewChange reads `selectedTokenId` on the way through
		// renderSelection. Without it, the whole map effect gains a
		// dependency on the selection: clicking a token rebuilds the map,
		// which re-enters this, and selection stopped working entirely
		// across 26 specs. The reads in there belong to their own effects,
		// which already track them.
		untrack(applyViewChange);
	}

	// The grid cell currently at the center of the viewport, in the same
	// cell-coordinate units tokens are placed in. Exposed so new tokens
	// can spawn near what the GM is actually looking at instead of
	// always at the map's origin.
	export function viewCenterCell(): { x: number; y: number } {
		const gridSize = room.scene?.gridSize;
		if (!stage || !gridSize) return { x: 0, y: 0 };

		const scale = stage.scaleX();
		const centerWorldX = (-stage.x() + stage.width() / 2) / scale;
		const centerWorldY = (-stage.y() + stage.height() / 2) / scale;
		return {
			x: Math.round(centerWorldX / gridSize),
			y: Math.round(centerWorldY / gridSize)
		};
	}

	// render() is async and awaits partway through (loading the map
	// image), so Svelte's $effect dependency tracking — which only sees
	// reads that happen synchronously before the first await — would
	// otherwise miss room.tokens/fogCells/you entirely. track() forces
	// them to be read up front, inside the effect's synchronous window.
	function track(...values: unknown[]) {
		return values.length;
	}

	// Deliberately not tracking room.drawings, room.tokens or
	// room.fogChunks: any of the three would rebuild the map image, every
	// grid line and every fog cell for a change that only touched a
	// single stroke, a single token's position, or one painted cell —
	// and drawing, erasing, dragging tokens and painting fog are the most
	// frequent things that happen to a scene. They get their own effects
	// below.
	$effect(() => {
		track(room.scene, room.you);
		render();
	});

	// The one place the theme is read reactively. `mode.current` is
	// mode-watcher's resolved scheme, so this fires both for someone
	// choosing Dark and for the OS flipping under a browser following it.
	//
	// A full render() for a colour change is more than it needs, and
	// costs nothing: a theme flips a handful of times in an evening, and
	// the map image is already in imageCache. Doing less would mean
	// keeping a second list of which render functions read the scheme.
	$effect(() => {
		const next = mode.current === 'dark' ? 'dark' : 'light';
		untrack(() => {
			if (next === stageScheme) return;
			stageScheme = next;
			if (stage) render();
		});
	});

	$effect(() => {
		track(room.drawings);
		if (stage) renderDrawings();
	});

	// Its own effect for the same reason drawings and tokens have theirs
	// (see the note above): fogOpacity fires on every tick of the GM
	// dragging their opacity slider, and pairing that with the full
	// rebuild would redraw the map, grid and every token for no reason.
	$effect(() => {
		track(room.fogChunks, room.scene, room.you, fogOpacity);
		const scene = room.scene;
		if (stage && scene) {
			renderFog(scene.gridSize, fogOpacity);
		}
	});

	// activeTool is tracked here, not only by the handler effect below,
	// because tokens are draggable in 'none' mode alone — switching to
	// any tool has to re-render them to take that away. It is tracked
	// *here* rather than alongside render() because token draggability is
	// the only thing in the whole render path that reads it, so pairing
	// it with the full rebuild made every tool switch redraw the map too.
	//
	// ownerOnlyMovement is tracked for exactly the same reason and is the
	// case that would be missed: a GM flipping the lock mid-session has to
	// take hold of everyone else's tokens *now*, not at their next reload.
	$effect(() => {
		track(room.tokens, room.scene, activeTool, room.ownerOnlyMovement, room.you);
		const gridSize = room.scene?.gridSize;
		if (stage && gridSize) renderTokens(gridSize);
	});

	$effect(() => {
		track(activeTool, room.scene, room.you);
		attachToolHandlers();
	});

	$effect(() => {
		track(room.pings);
		renderPings();
	});

	$effect(() => {
		track(room.measurements, room.scene);
		renderMeasurements();
	});

	// room.tokens is tracked because the ring has to follow the token it
	// marks — someone else moving it, or it leaving the scene entirely,
	// both arrive that way.
	$effect(() => {
		track(room.tokens, room.scene, selectedTokenId);
		if (stage) renderSelection();
	});

	// Selecting a token raises it, wherever the selection came from — the
	// stage's own click handler, or a linked entry in the initiative
	// tracker, which sets the same bound id from outside this component.
	// One effect for both, rather than a call at each site, because the
	// tracker's click never reaches the canvas at all.
	//
	// Deliberately its own effect rather than a line in the one above:
	// that one also tracks room.tokens, so anyone changing any token would
	// re-raise the selected one — over a token dragged since, which was
	// the more recent interaction.
	$effect(() => {
		if (stage && selectedTokenId) raiseToken(selectedTokenId);
	});

	// room.tokens is tracked so the card follows the numbers it's showing:
	// someone else changing a hovered token's hit points has to reach the
	// card, and a token that leaves the scene has to take its card with it.
	$effect(() => {
		track(room.tokens, room.scene, hoveredTokenId);
		if (stage) renderHoverCard();
	});

	// The eraser's halo points at a specific drawing, so it has to be
	// re-resolved whenever the set of drawings changes — otherwise it
	// hangs over the empty space where a stroke used to be (including
	// one someone else just erased) until the pointer next moves.
	//
	// strokeWidth is tracked for the other half of the overlay: the
	// freehand ring is a picture of how wide the line will be, so picking
	// a new width has to resize it under a pointer that hasn't moved.
	$effect(() => {
		track(room.drawings, room.you, activeTool, strokeWidth);
		refreshCursorOverlay();
	});

	// --- shape geometry shared between committed drawings and the live
	// rubber-band preview while a shape is being drawn ---

	function lineGeometry(a: DrawingPoint, b: DrawingPoint) {
		return { points: [a.x, a.y, b.x, b.y] };
	}

	function rectGeometry(a: DrawingPoint, b: DrawingPoint) {
		return {
			x: Math.min(a.x, b.x),
			y: Math.min(a.y, b.y),
			width: Math.abs(b.x - a.x),
			height: Math.abs(b.y - a.y)
		};
	}

	// Ellipses are dragged out corner to corner, inscribed in the box the
	// drag defines — the same gesture as a rect, and the same two stored
	// points. Konva wants that as a centre plus two radii.
	function ellipseGeometry(a: DrawingPoint, b: DrawingPoint) {
		return {
			x: (a.x + b.x) / 2,
			y: (a.y + b.y) / 2,
			radiusX: Math.abs(b.x - a.x) / 2,
			radiusY: Math.abs(b.y - a.y) / 2
		};
	}

	// --- pointer-driven tools: fog rect, freehand/line/rect/circle
	// drawing, and ping. Exactly one owns the stage's pointer at a time. ---

	let drawStart: DrawingPoint | null = null;
	let freehandPoints: DrawingPoint[] = [];
	let previewShape: Konva.Shape | null = null;

	function clearPreview() {
		previewShape?.destroy();
		previewShape = null;
		drawStart = null;
		freehandPoints = [];
		previewLayer?.batchDraw();
	}

	// Where the current measurement was dragged from, or null when none
	// is in progress. The measurement itself lives in RoomClient rather
	// than here, since it's shared with the rest of the room while the
	// drag lasts.
	let measureStart: DrawingPoint | null = null;

	function stopMeasuring() {
		if (!measureStart) return;
		measureStart = null;
		room.endMeasure();
	}

	// --- cursor overlay: a ring showing the tool's reach, plus (for the
	// eraser) a halo on the stroke a click would remove. Both live on
	// previewLayer, which doesn't listen for events and is above
	// everything else. ---

	let cursorRing: Konva.Circle | null = null;
	let eraseHighlight: Konva.Shape | null = null;
	let highlightedDrawingId: string | null = null;

	// Whether the eraser is mid-sweep, and where it last erased — the
	// start of the next segment to clear.
	let erasing = false;
	let lastErasePoint: DrawingPoint | null = null;

	function stopErasing() {
		erasing = false;
		lastErasePoint = null;
	}

	function clearCursorOverlay() {
		cursorRing?.destroy();
		cursorRing = null;
		eraseHighlight?.destroy();
		eraseHighlight = null;
		highlightedDrawingId = null;
		previewLayer?.batchDraw();
	}

	// World-space distance covered by n screen pixels at the current
	// zoom — how the eraser keeps a constant on-screen reach, and how
	// overlay outlines keep a constant on-screen thickness.
	function screenToWorld(pixels: number): number {
		return pixels / (stage?.scaleX() ?? 1);
	}

	// radius is in world units, so the ring shows the tool's actual
	// footprint on the map rather than a fixed blob on the screen.
	function updateCursorRing(radius: number) {
		const pos = stage?.getRelativePointerPosition();
		if (!pos) return;

		if (!cursorRing) {
			cursorRing = new Konva.Circle({
				fill: 'rgba(120, 120, 120, 0.18)',
				stroke: '#52525b',
				listening: false
			});
			previewLayer.add(cursorRing);
		}
		cursorRing.position(pos);
		cursorRing.radius(radius);
		cursorRing.strokeWidth(screenToWorld(1));
	}

	// The drawing the eraser would take at point: the nearest one within
	// reach, considering only drawings this participant is allowed to
	// erase — so someone else's stroke lying closer doesn't mask your
	// own, and touching it does nothing rather than erroring.
	function eraserTargetAt(point: DrawingPoint): Drawing | null {
		return pickDrawing(room.drawings.filter(canErase), point, screenToWorld(ERASER_PICK_RADIUS));
	}

	function eraserTargetAtPointer(): Drawing | null {
		const pos = stage?.getRelativePointerPosition();
		return pos ? eraserTargetAt(pos) : null;
	}

	function eraseAt(point: DrawingPoint) {
		const target = eraserTargetAt(point);
		if (target) room.deleteDrawing(target.id);
	}

	// Erasing along the path the pointer travelled, not just where it
	// landed: pointer events arrive far apart when the mouse is moving
	// quickly, and testing only the endpoints would sweep straight over
	// strokes in between. Stepping by no more than the pick radius makes
	// the tested circles overlap, so nothing within reach can fall
	// through a gap.
	function eraseAlong(from: DrawingPoint, to: DrawingPoint) {
		const step = screenToWorld(ERASER_PICK_RADIUS);
		const steps = Math.max(1, Math.ceil(Math.hypot(to.x - from.x, to.y - from.y) / step));
		for (let i = 1; i <= steps; i++) {
			const t = i / steps;
			eraseAt({ x: from.x + (to.x - from.x) * t, y: from.y + (to.y - from.y) * t });
		}
	}

	function updateEraserCursor() {
		updateCursorRing(screenToWorld(ERASER_PICK_RADIUS));

		const target = eraserTargetAtPointer();
		if (target?.id !== highlightedDrawingId) {
			eraseHighlight?.destroy();
			eraseHighlight = null;
			highlightedDrawingId = target?.id ?? null;

			if (target) {
				eraseHighlight = shapeForDrawing(target);
				eraseHighlight.stroke(ERASE_HIGHLIGHT_COLOR);
				eraseHighlight.opacity(0.55);
				previewLayer.add(eraseHighlight);
			}
		}
		// Set every time, not just on a new target: the halo is sized in
		// screen pixels, so it has to be recomputed when the zoom changes
		// under a stationary pointer.
		if (target && eraseHighlight) {
			eraseHighlight.strokeWidth(
				strokeWidthOf(target) + screenToWorld(ERASE_HIGHLIGHT_PADDING * 2)
			);
		}
		previewLayer.batchDraw();
	}

	// Zooming changes what a screen-pixel reach means in world units, so
	// the overlay has to be recomputed even though the pointer hasn't
	// moved.
	function refreshCursorOverlay() {
		if (activeTool === 'eraser') {
			updateEraserCursor();
		} else if (activeTool === 'freehand') {
			updateCursorRing(strokeWidth / 2);
			previewLayer.batchDraw();
		}
	}

	// Konva reports every mouse button through the same mousedown/mouseup
	// events, so without this a right-drag draws, erases, reveals fog,
	// pings and measures exactly as a left one does. Every tool that opens
	// a gesture checks this.
	//
	// Touch events carry no `button` at all, so they have to pass rather
	// than be rejected for failing to be the primary button — testing
	// `button !== 0` alone would silently break every tool on a tablet.
	function isPrimaryPointer(e: Konva.KonvaEventObject<MouseEvent | TouchEvent>): boolean {
		return !(e.evt instanceof MouseEvent) || e.evt.button === 0;
	}

	// Abandons whatever gesture is part-way through, leaving nothing
	// half-finished on this map or anyone else's. Called when the tool
	// changes and when a second finger turns a drag into a pinch — the
	// two need identical cleanup, and having had it written out twice is
	// how a retraction gets added to one and missed in the other.
	//
	// The measurement is the one that matters beyond this browser: it
	// lives in RoomClient and is broadcast, so without the endMeasure()
	// inside stopMeasuring() it stays frozen on every other map with no
	// end event ever coming.
	function retractInFlightGesture() {
		stopErasing();
		stopMeasuring();
		clearPreview();
		clearCursorOverlay();
		// A token drag isn't a tool gesture — it only happens with no tool
		// active — so a tool change can't be mid-drag. A pinch can: the
		// second finger arrives while the first is still holding a token,
		// and Konva's own drag is stopped without ever firing the dragend
		// that would otherwise clear this.
		clearTokenDragPreview();
	}

	function attachToolHandlers() {
		if (!stage) return;
		stage.off(
			'mousedown.tool touchstart.tool mousemove.tool touchmove.tool mouseup.tool touchend.tool mouseleave.tool'
		);
		retractInFlightGesture();

		const scene = room.scene;
		const isActive = activeTool !== 'none' && !!scene;
		// Tools and panning both start on left-drag, so only one can own
		// the gesture at a time.
		stage.draggable(!isActive);

		// Selecting is its own namespace rather than part of the block
		// above, because it isn't a tool: it only binds when no tool owns
		// the pointer, since with one active a click means erase, ping, or
		// the first half of a drag. An existing selection survives a tool
		// switch — it just can't be changed until the tool is put down.
		stage.off('click.select tap.select');
		if (!isActive) {
			stage.on('click.select tap.select', (e) => {
				if (!isPrimaryPointer(e)) return;
				// Konva suppresses `click` after a real drag, so dragging a
				// token deliberately doesn't select it — only a click that
				// stayed put does.
				const group = e.target.findAncestor('.token', true) as Konva.Group | undefined;
				// Anything that isn't a token — bare grid, the map image, a
				// drawing — clears the selection.
				selectedTokenId = (group?.getAttr('tokenId') as string | undefined) ?? null;
			});
		}

		if (!isActive || !scene) return;

		const gridSize = scene.gridSize;
		const sceneId = scene.id;

		// Revealing and hiding both rubber-band a rectangle from a fixed
		// start point, the same shape line/rect/ellipse drag below — and
		// commit every cell inside it in one command on release. An L-shaped
		// room is two drags, not a reason for a freeform sweep: a rectangle
		// covers what that shape needs without asking for a steady hand
		// along the way, and a plain click (no drag at all) still resolves
		// to the one cell under the pointer, for a single-square touch-up.
		if (activeTool === 'fog-reveal' || activeTool === 'fog-hide') {
			if (room.you?.role !== 'gm') return; // fog stays GM-only
			const hiding = activeTool === 'fog-hide';
			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				drawStart = pos;
				previewShape = buildFogPreviewShape(gridSize, pos, pos, hiding);
				previewLayer.add(previewShape);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				if (!drawStart || !previewShape) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				updateFogPreviewShape(previewShape as Konva.Rect, gridSize, drawStart, pos);
				previewLayer.batchDraw();
			});
			stage.on('mouseup.tool touchend.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				if (drawStart) {
					const pos = stage!.getRelativePointerPosition() ?? drawStart;
					const cells = fogCellsInRange(fogCellRange(gridSize, drawStart, pos));
					// Every cell in the rectangle is sent, including ones already
					// in the target state — both commands are idempotent server
					// side, which is what lets this stay a plain bounding box.
					if (hiding) room.hideFog(sceneId, cells);
					else room.revealFog(sceneId, cells);
				}
				clearPreview();
			});
			return;
		}

		if (activeTool === 'eraser') {
			// Held down, the eraser keeps taking whatever it's dragged
			// across, so clearing a scribbled-over area is one gesture
			// rather than a click per stroke.
			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				erasing = true;
				lastErasePoint = pos;
				eraseAt(pos);
				// Whatever was highlighted is either gone or no longer
				// under the pointer, so re-resolve from where we are now.
				updateEraserCursor();
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				const pos = stage!.getRelativePointerPosition();
				if (erasing && pos) {
					eraseAlong(lastErasePoint ?? pos, pos);
					lastErasePoint = pos;
				}
				updateEraserCursor();
			});
			stage.on('mouseup.tool touchend.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				stopErasing();
			});
			// Unguarded, unlike the mouseup above: leaving the canvas ends the
			// sweep whatever the buttons are doing, since no mouseup is coming
			// once the pointer is released outside.
			stage.on('mouseleave.tool', () => {
				stopErasing();
				clearCursorOverlay();
			});
			return;
		}

		// The distance line and all four area templates are one gesture:
		// press to set the point of origin, drag to size it, release to
		// take it off everyone's map. Only the shape sent differs, and
		// only templates snap — the distance line already reports whole
		// squares from the cells its ends fall in.
		const templateKind = TEMPLATE_TOOLS[activeTool];
		if (activeTool === 'measure' || templateKind) {
			const kind = templateKind ?? 'distance';
			// Both read *here* rather than inside the handlers below. This
			// function runs inside the $effect that rebinds tool handlers, so
			// only what it reads synchronously is tracked — a value read
			// later, when a pointer event fires, would be captured once and
			// never refreshed, leaving the snap control doing nothing until
			// the tool was reselected.
			const snap = snapMode;
			const width = templateKind === 'line' ? lineWidthFeet : undefined;
			// The two ends are treated differently, and only for templates.
			// The origin obeys the snap setting; the far end is left where
			// the pointer put it and then pulled to the nearest whole area
			// size, so the drag sets direction and the rules set length.
			// Snapping the far end too would only coarsen the direction, and
			// the quantise would move it off the grid regardless.
			const placeOrigin = (point: DrawingPoint) =>
				templateKind ? snapPoint(point, gridSize, snap) : point;
			const placeEnd = (start: DrawingPoint, point: DrawingPoint) =>
				templateKind ? quantiseTemplateEnd(templateKind, start, point, gridSize) : point;

			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				measureStart = placeOrigin(pos);
				room.updateMeasure(sceneId, measureStart, measureStart, kind, width);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				const pos = stage!.getRelativePointerPosition();
				if (!measureStart || !pos) return;
				room.updateMeasure(sceneId, measureStart, placeEnd(measureStart, pos), kind, width);
			});
			stage.on('mouseup.tool touchend.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				stopMeasuring();
			});
			// Split from the mouseup above, and deliberately unguarded:
			// leaving the canvas ends the measurement rather than leaving it
			// frozen at the edge on everyone else's map, since no mouseup is
			// coming once the pointer is released outside.
			stage.on('mouseleave.tool', stopMeasuring);
			return;
		}

		if (activeTool === 'ping') {
			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				room.sendPing(sceneId, pos.x, pos.y);
			});
			return;
		}

		if (activeTool === 'freehand') {
			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				freehandPoints = [pos];
				previewShape = new Konva.Line({
					points: [pos.x, pos.y],
					stroke: strokeColor,
					strokeWidth,
					lineCap: 'round',
					lineJoin: 'round',
					dash: previewDash(strokeWidth),
					listening: false
				});
				previewLayer.add(previewShape);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				// The ring tracks the pointer whether or not a stroke is in
				// progress: it's showing how wide the line will be.
				updateCursorRing(strokeWidth / 2);
				previewLayer.batchDraw();

				if (!previewShape) return;
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				const last = freehandPoints[freehandPoints.length - 1];
				if (Math.hypot(pos.x - last.x, pos.y - last.y) < MIN_FREEHAND_SPACING) return;
				freehandPoints.push(pos);
				(previewShape as Konva.Line).points(freehandPoints.flatMap((p) => [p.x, p.y]));
				previewLayer.batchDraw();
			});
			stage.on('mouseup.tool touchend.tool', (e) => {
				// Guarded as well as mousedown, so releasing the right button
				// part-way through a left-button stroke doesn't commit it early.
				if (!isPrimaryPointer(e)) return;
				if (freehandPoints.length >= 2) {
					room.createDrawing(sceneId, 'freehand', freehandPoints, strokeColor, { strokeWidth });
				}
				clearPreview();
			});
			stage.on('mouseleave.tool touchend.tool', () => clearCursorOverlay());
			return;
		}

		// line, rect, ellipse: rubber-band from a fixed start point to
		// wherever the pointer currently is.
		//
		// Named explicitly rather than taken as whatever is left over: this
		// used to be a fall-through, so a tool added above without its own
		// branch silently became a rubber-band drawing tool instead of
		// doing nothing. The type checker catches it now.
		if (activeTool !== 'line' && activeTool !== 'rect' && activeTool !== 'ellipse') return;
		const kind = activeTool;
		stage.on('mousedown.tool touchstart.tool', (e) => {
			if (!isPrimaryPointer(e)) return;
			const pos = stage!.getRelativePointerPosition();
			if (!pos) return;
			drawStart = pos;
			previewShape = buildPreviewShape(kind, pos, pos);
			previewLayer.add(previewShape);
		});
		stage.on('mousemove.tool touchmove.tool', () => {
			if (!drawStart || !previewShape) return;
			const pos = stage!.getRelativePointerPosition();
			if (!pos) return;
			updatePreviewShape(previewShape, kind, drawStart, pos);
			previewLayer.batchDraw();
		});
		stage.on('mouseup.tool touchend.tool', (e) => {
			// Same reason as freehand: a right-button release must not commit
			// the shape a left-button drag is still rubber-banding.
			if (!isPrimaryPointer(e)) return;
			if (drawStart) {
				const pos = stage!.getRelativePointerPosition() ?? drawStart;
				if (pos.x !== drawStart.x || pos.y !== drawStart.y) {
					room.createDrawing(sceneId, kind, [drawStart, pos], strokeColor, {
						filled: shapeFilled,
						strokeWidth
					});
				}
			}
			clearPreview();
		});
	}

	// Dashes proportional to the stroke, not a fixed [6, 4]: at 16 world
	// pixels wide that pattern is a row of blobs rather than a dashed
	// outline, because a dash shorter than the line is thick reads as a
	// square. At the default width this is what it always was.
	function previewDash(width: number): number[] {
		return [width * 2, width * 1.5];
	}

	function buildPreviewShape(
		kind: 'line' | 'rect' | 'ellipse',
		a: DrawingPoint,
		b: DrawingPoint
	): Konva.Shape {
		// Drawn at the width it will land at, for the same reason the fill
		// below is drawn at all. This used to be a flat 2, which was
		// indistinguishable from the default width and a lie about any
		// other.
		const strokeProps = {
			stroke: strokeColor,
			strokeWidth,
			dash: previewDash(strokeWidth),
			listening: false
		};
		// The preview carries the fill as well, so what is being dragged
		// out looks like what will land. The dashed outline is what still
		// says "not committed yet".
		const fillProps = shapeFilled && kind !== 'line' ? { fill: fillFor(strokeColor) } : {};
		switch (kind) {
			case 'line':
				return new Konva.Line({ ...lineGeometry(a, b), ...strokeProps });
			case 'rect':
				return new Konva.Rect({ ...rectGeometry(a, b), ...strokeProps, ...fillProps });
			case 'ellipse':
				return new Konva.Ellipse({ ...ellipseGeometry(a, b), ...strokeProps, ...fillProps });
		}
	}

	function updatePreviewShape(
		shape: Konva.Shape,
		kind: 'line' | 'rect' | 'ellipse',
		a: DrawingPoint,
		b: DrawingPoint
	) {
		switch (kind) {
			case 'line':
				shape.setAttrs(lineGeometry(a, b));
				break;
			case 'rect':
				shape.setAttrs(rectGeometry(a, b));
				break;
			case 'ellipse':
				shape.setAttrs(ellipseGeometry(a, b));
				break;
		}
	}

	// The two fog tools tint their drag rectangle in different colours so a
	// GM mid-drag can tell which direction they're painting in. Neither is
	// what the cells will end up looking like — that arrives with the
	// broadcast, which re-renders the fog layer from scratch.
	const FOG_REVEAL_PREVIEW = 'yellow';
	const FOG_HIDE_PREVIEW = '#dc2626';

	// The grid cells a drag from a to b covers, inclusive at both ends —
	// floor at each corner independently rather than measuring a span, so
	// a==b (a plain click) always resolves to exactly the one cell under
	// the pointer instead of an empty range.
	function fogCellRange(gridSize: number, a: DrawingPoint, b: DrawingPoint) {
		return {
			x0: Math.floor(Math.min(a.x, b.x) / gridSize),
			y0: Math.floor(Math.min(a.y, b.y) / gridSize),
			x1: Math.floor(Math.max(a.x, b.x) / gridSize),
			y1: Math.floor(Math.max(a.y, b.y) / gridSize)
		};
	}

	function fogCellsInRange(range: ReturnType<typeof fogCellRange>): FogCell[] {
		const cells: FogCell[] = [];
		for (let y = range.y0; y <= range.y1; y++) {
			for (let x = range.x0; x <= range.x1; x++) {
				cells.push({ x, y });
			}
		}
		return cells;
	}

	function buildFogPreviewShape(
		gridSize: number,
		a: DrawingPoint,
		b: DrawingPoint,
		hiding: boolean
	): Konva.Rect {
		const shape = new Konva.Rect({
			fill: hiding ? FOG_HIDE_PREVIEW : FOG_REVEAL_PREVIEW,
			opacity: 0.35,
			listening: false
		});
		updateFogPreviewShape(shape, gridSize, a, b);
		return shape;
	}

	// Snapped to whole cells on every move rather than the raw pixel
	// rectangle, so the preview always shows exactly the set
	// fogCellsInRange will send — not a rectangle that disagrees with it
	// at the edges.
	function updateFogPreviewShape(
		shape: Konva.Rect,
		gridSize: number,
		a: DrawingPoint,
		b: DrawingPoint
	) {
		const range = fogCellRange(gridSize, a, b);
		shape.setAttrs({
			x: range.x0 * gridSize,
			y: range.y0 * gridSize,
			width: (range.x1 - range.x0 + 1) * gridSize,
			height: (range.y1 - range.y0 + 1) * gridSize
		});
	}

	async function render() {
		if (!stage) return;
		const scene = room.scene;

		const sceneId = scene?.id ?? null;
		if (sceneId !== lastSceneId) {
			lastSceneId = sceneId;
			resetView();
			// Nothing on the new scene has a previous position on this one.
			// Without this every token would slide in from wherever some
			// unrelated token happened to be standing on the old map.
			for (const id of [...moveTweens.keys()]) stopTokenMove(id);
			renderedPositions.clear();
		}

		if (!scene) {
			mapLayer.destroyChildren();
			gridLayer.destroyChildren();
			fogLayer.destroyChildren();
			drawingLayer.destroyChildren();
			tokenLayer.destroyChildren();
			mapLayer.draw();
			gridLayer.draw();
			fogLayer.draw();
			drawingLayer.draw();
			tokenLayer.draw();
			return;
		}

		const width = scene.width || 0;
		const height = scene.height || 0;

		await renderMap(scene.mapAssetId, width, height);
		renderGrid();
		renderFog(scene.gridSize, fogOpacity);
		renderDrawings();
		renderTokens(scene.gridSize);
	}

	// The map image/background only covers the scene's defined bounds —
	// that's "the map". Panning beyond it shows bare infinite grid.
	async function renderMap(mapAssetId: string | null, width: number, height: number) {
		mapLayer.destroyChildren();

		if (width > 0 && height > 0) {
			if (mapAssetId) {
				try {
					const img = await loadImage(assetUrl(mapAssetId));
					mapLayer.add(new Konva.Image({ image: img, width, height }));
				} catch {
					mapLayer.add(new Konva.Rect({ width, height, fill: MAP_LOAD_FAILED }));
				}
			} else {
				mapLayer.add(new Konva.Rect({ width, height, fill: MAP_PLACEHOLDER[stageScheme] }));
			}
		}

		mapLayer.batchDraw();
	}

	// Draws grid lines across whatever part of the infinite plane is
	// currently visible, recomputed on every pan/zoom/resize — rather
	// than once over a fixed scene size — so the grid never runs out.
	function renderGrid() {
		gridLayer.destroyChildren();

		const gridSize = room.scene?.gridSize;
		if (!stage || !gridSize) {
			gridLayer.batchDraw();
			return;
		}

		const scale = stage.scaleX();
		const viewLeft = -stage.x() / scale;
		const viewTop = -stage.y() / scale;
		const viewRight = viewLeft + stage.width() / scale;
		const viewBottom = viewTop + stage.height() / scale;

		const startX = Math.floor(viewLeft / gridSize) * gridSize;
		const startY = Math.floor(viewTop / gridSize) * gridSize;
		// Constant on-screen thickness regardless of zoom level.
		const strokeWidth = 1 / scale;
		const stroke = GRID_LINE[stageScheme];

		for (let x = startX; x <= viewRight; x += gridSize) {
			gridLayer.add(
				new Konva.Line({
					points: [x, viewTop, x, viewBottom],
					stroke,
					strokeWidth
				})
			);
		}
		for (let y = startY; y <= viewBottom; y += gridSize) {
			gridLayer.add(
				new Konva.Line({
					points: [viewLeft, y, viewRight, y],
					stroke,
					strokeWidth
				})
			);
		}

		gridLayer.batchDraw();
	}

	// Fog is drawn directly: the hidden cells are filled in, rather than
	// the whole scene being covered and the revealed cells punched back
	// out of it with destination-out. That inversion follows the storage
	// one — what's stored is now what's hidden, so what's drawn is what's
	// hidden — and it takes two problems with it. A scene with no fog
	// draws nothing at all instead of a full-size rect plus a punch-out
	// per cell, and the GM's opacity is applied once to real geometry
	// rather than being multiplied by a second translucent rect (which is
	// what used to leave revealed cells at ~0.23 instead of 0).
	//
	// The scene's width and height don't come into it any more: fog is
	// wherever its chunks say it is, including outside the map's bounds.
	function renderFog(gridSize: number, opacity: number) {
		fogLayer.destroyChildren();

		const runs = fogRuns(room.fogChunks);
		if (runs.length === 0) {
			fogLayer.batchDraw();
			return;
		}

		// Every run goes into one shape as one compound path, filled once,
		// rather than a Konva.Rect each. Abutting translucent rectangles
		// blend twice along the edge they share and leave a hairline grid
		// over the fog at any opacity below 1; a single path has no
		// interior edges to blend. It is also one node instead of hundreds.
		fogLayer.add(
			new Konva.Shape({
				sceneFunc: (context, shape) => {
					context.beginPath();
					for (const run of runs) {
						context.rect(run.x * gridSize, run.y * gridSize, run.length * gridSize, gridSize);
					}
					context.fillStrokeShape(shape);
				},
				fill: 'black',
				// A Player's fog is opaque and always has been. The GM sees
				// through their own at whatever strength they've set, which is
				// a per-browser preference rather than room state — see
				// $lib/fog-opacity.
				opacity: room.you?.role === 'gm' ? opacity : 1,
				listening: false
			})
		);

		fogLayer.batchDraw();
	}

	// Drawings are visible to everyone regardless of role — they're a
	// shared communication tool, not game-hidden state — so they render
	// above fog rather than being subject to it. The layer is inert:
	// the eraser finds its target geometrically (see $lib/drawing-hit),
	// not by asking Konva what was clicked.
	function renderDrawings() {
		drawingLayer.destroyChildren();
		for (const d of room.drawings) {
			drawingLayer.add(shapeForDrawing(d));
		}
		drawingLayer.batchDraw();
	}

	// A GM can erase anything on the map; everyone else only what they
	// drew themselves. A drawing with no recorded author belongs to
	// nobody, so it isn't a Player's to erase. The server enforces this
	// too — checking here just means clicking someone else's work does
	// nothing, instead of coming back as an error toast.
	function canErase(d: Drawing): boolean {
		if (!room.you) return false;
		if (room.you.role === 'gm') return true;
		return d.createdByParticipantId === room.you.participantId;
	}

	function shapeForDrawing(d: Drawing): Konva.Shape {
		const strokeProps = {
			stroke: d.color,
			strokeWidth: strokeWidthOf(d),
			listening: false
		};
		// The fill is translucent while the stroke stays solid, so the
		// shape keeps a crisp edge — hence a colour rather than Konva's
		// `opacity`, which would fade the outline with it.
		const fillProps = isFilled(d) ? { fill: fillFor(d.color) } : {};
		switch (d.kind) {
			case 'freehand':
				return new Konva.Line({
					points: d.points.flatMap((p) => [p.x, p.y]),
					lineCap: 'round',
					lineJoin: 'round',
					...strokeProps
				});
			case 'line':
				return new Konva.Line({ ...lineGeometry(d.points[0], d.points[1]), ...strokeProps });
			case 'rect':
				return new Konva.Rect({
					...rectGeometry(d.points[0], d.points[1]),
					...strokeProps,
					...fillProps
				});
			case 'ellipse':
				return new Konva.Ellipse({
					...ellipseGeometry(d.points[0], d.points[1]),
					...strokeProps,
					...fillProps
				});
		}
	}

	// Ping markers are ephemeral: RoomClient removes each ping from
	// room.pings on its own after PING_LIFETIME_MS, which is what
	// actually clears its shapes here (via the "no longer present"
	// branch below) — the tweens are just the look, not the mechanism
	// that ends the ping. That lifetime is derived from the pulse timing
	// in $lib/ping so a marker can't be dropped mid-sequence.
	type PingMarker = { rings: Konva.Circle[]; timers: ReturnType<typeof setTimeout>[] };

	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const pingMarkers = new Map<string, PingMarker>();

	function renderPings() {
		if (!pingLayer) return;

		const currentIds = new Set(room.pings.map((p) => p.id));
		for (const [id, marker] of pingMarkers) {
			if (!currentIds.has(id)) {
				destroyPingMarker(marker);
				pingMarkers.delete(id);
			}
		}

		for (const ping of room.pings) {
			if (pingMarkers.has(ping.id)) continue;
			pingMarkers.set(ping.id, startPingPulses(ping));
		}

		pingLayer.batchDraw();
	}

	// One click pulses a few times rather than once: a single flash is
	// easy to miss if someone happened to be looking at the chat panel,
	// and repeating it costs nothing on the wire — the server broadcasts
	// one ping and every client expands it into the same sequence.
	function startPingPulses(ping: Ping): PingMarker {
		const rings: Konva.Circle[] = [];
		const timers: ReturnType<typeof setTimeout>[] = [];
		// The pinger's own colour, falling back to the amber every ping
		// used to be. That fallback is not decoration: a seat from before
		// colours has none, and a ping nobody can see is worse than one
		// that doesn't say whose it is.
		const stroke = identityHex(room.colorOf(ping.participantId)) ?? PING_COLOR;

		for (let i = 0; i < PING_PULSES; i++) {
			const ring = new Konva.Circle({
				x: ping.x,
				y: ping.y,
				radius: 6,
				stroke,
				strokeWidth: 3,
				// Later pulses stay invisible until their turn, rather than
				// sitting on the map as a static dot in the meantime.
				opacity: 0,
				listening: false
			});
			pingLayer.add(ring);
			rings.push(ring);

			const pulse = () => {
				ring.opacity(1);
				new Konva.Tween({
					node: ring,
					duration: PING_PULSE_SECONDS,
					radius: 40,
					opacity: 0,
					easing: Konva.Easings.EaseOut
				}).play();
			};

			if (i === 0) {
				pulse();
			} else {
				timers.push(setTimeout(pulse, i * PING_PULSE_INTERVAL_MS));
			}
		}

		return { rings, timers };
	}

	// Pending pulses have to be cancelled along with the shapes they
	// animate — a timer that fires after its ring is destroyed would be
	// reaching into a node that no longer exists.
	function destroyPingMarker(marker: PingMarker) {
		for (const timer of marker.timers) clearTimeout(timer);
		for (const ring of marker.rings) ring.destroy();
	}

	// Measurements are rebuilt wholesale on every change. There is at most
	// one per participant and each is a handful of shapes, so this stays
	// far cheaper than diffing — unlike drawings, which accumulate.
	function renderMeasurements() {
		if (!measureLayer) return;
		measureLayer.destroyChildren();

		const gridSize = room.scene?.gridSize;
		if (gridSize) {
			for (const measurement of room.measurements) {
				if (measurement.sceneId !== room.scene?.id) continue;
				drawMeasurement(measurement, gridSize);
			}
		}

		measureLayer.batchDraw();
	}

	function drawMeasurement(measurement: Measurement, gridSize: number) {
		if (measurement.kind !== 'distance') {
			drawTemplate(measurement, gridSize);
			return;
		}
		drawDistance(measurement, gridSize);
	}

	// An area template is drawn as its true shape and nothing else — no
	// squares are highlighted, deliberately. Tables disagree about which
	// squares an area catches, so highlighting would be picking a side
	// invisibly; this is the paper cutout laid on the map, and the
	// players read it. See the header comment in $lib/aoe.
	function drawTemplate(measurement: Measurement, gridSize: number) {
		const { kind, from, to } = measurement;
		if (kind === 'distance') return;

		const outline = {
			stroke: MEASURE_COLOR,
			strokeWidth: screenToWorld(MEASURE_LINE_WIDTH),
			fill: TEMPLATE_FILL,
			listening: false
		};

		if (kind === 'circle') {
			const radius = circleRadius(from, to);
			if (radius <= 0) return;
			measureLayer.add(new Konva.Circle({ x: from.x, y: from.y, radius, ...outline }));
		} else {
			const polygon = templatePolygon(kind, from, to, gridSize, measurement.widthFeet);
			if (polygon.length === 0) return;
			measureLayer.add(
				new Konva.Line({
					points: polygon.flatMap((p) => [p.x, p.y]),
					closed: true,
					...outline
				})
			);
		}

		// The point of origin, marked because it's the one part of a
		// template that isn't obvious from the outline — a cube's origin
		// is a corner, a cone's is the apex, a circle's is the centre.
		measureLayer.add(
			new Konva.Circle({
				x: from.x,
				y: from.y,
				radius: screenToWorld(MEASURE_END_RADIUS),
				fill: MEASURE_COLOR,
				listening: false
			})
		);

		const size = templateLabel(kind, from, to, gridSize, measurement.widthFeet);
		addMeasureLabel(labelFor(measurement, size), to);
	}

	// Someone else's measurement says whose it is; your own doesn't need
	// to.
	function labelFor(measurement: Measurement, text: string): string {
		return measurement.participantId === room.you?.participantId
			? text
			: `${measurement.participantName}: ${text}`;
	}

	// Drawn cell centre to cell centre rather than pointer to pointer:
	// the distance is counted in whole squares, so a line that stopped
	// wherever the pointer happened to be would disagree with its own
	// label about which square it had reached.
	function drawDistance(measurement: Measurement, gridSize: number) {
		const from = cellCentre(cellAt(measurement.from, gridSize), gridSize);
		const to = cellCentre(cellAt(measurement.to, gridSize), gridSize);

		measureLayer.add(
			new Konva.Line({
				points: [from.x, from.y, to.x, to.y],
				stroke: MEASURE_COLOR,
				strokeWidth: screenToWorld(MEASURE_LINE_WIDTH),
				dash: [screenToWorld(8), screenToWorld(6)],
				listening: false
			})
		);
		for (const end of [from, to]) {
			measureLayer.add(
				new Konva.Circle({
					x: end.x,
					y: end.y,
					radius: screenToWorld(MEASURE_END_RADIUS),
					fill: MEASURE_COLOR,
					listening: false
				})
			);
		}

		addMeasureLabel(
			labelFor(measurement, measureLabel(measurement.from, measurement.to, gridSize)),
			to
		);
	}

	function addMeasureLabel(text: string, at: DrawingPoint) {
		measureLayer.add(buildMeasureLabel(text, at));
	}

	// The label itself, unattached. Split from addMeasureLabel because the
	// token drag preview shows the same badge on a different layer, and
	// two hand-built copies of it is how the ruler and the drag preview
	// start disagreeing about what a distance looks like.
	function buildMeasureLabel(text: string, at: DrawingPoint): Konva.Label {
		const group = new Konva.Label({
			x: at.x,
			y: at.y - screenToWorld(MEASURE_LABEL_OFFSET),
			listening: false
		});
		group.add(
			new Konva.Tag({
				fill: MEASURE_COLOR,
				cornerRadius: screenToWorld(MEASURE_LABEL_PADDING),
				// Anchored by its bottom-centre so the label sits above the
				// end of the line and stays centred on it as it grows.
				pointerDirection: 'down',
				pointerWidth: screenToWorld(6),
				pointerHeight: screenToWorld(4)
			})
		);
		group.add(
			new Konva.Text({
				text,
				fontSize: screenToWorld(MEASURE_FONT_SIZE),
				padding: screenToWorld(MEASURE_LABEL_PADDING),
				fill: 'white'
			})
		);
		return group;
	}

	// --- the hover card: a token's trackers and conditions, shown while
	// the pointer is over it. Local like the selection, and for the same
	// reason — what one person's pointer is doing isn't the room's
	// business. ---

	// Which token the pointer is over, or null. $state because the card is
	// rendered from an effect: unlike the selection ring's bookkeeping,
	// this one *is* the reactive truth.
	let hoveredTokenId = $state<string | null>(null);

	// What the card says, or null when there is nothing worth saying. A
	// token nobody has put a number on gets no card at all — every token on
	// the map popping an empty box as the pointer crossed it would make the
	// map unusable during a fight, which is exactly when this is for.
	function hoverCardText(token: Token): string | null {
		const lines: string[] = [];
		const trackers = setTrackers(token);
		if (trackers.length) lines.push(trackers.map(trackerText).join('   '));
		const conditions = token.conditions ?? [];
		if (conditions.length) lines.push(conditions.join(', '));
		return lines.length ? lines.join('\n') : null;
	}

	function renderHoverCard() {
		if (!hoverLayer) return;
		hoverLayer.destroyChildren();

		const gridSize = room.scene?.gridSize;
		const token = hoveredTokenId ? room.tokens.find((t) => t.id === hoveredTokenId) : undefined;
		// A hovered id with no token behind it — deleted, hidden, or the
		// scene changed under it — reads as nothing hovered. The id is left
		// alone rather than cleared, so this doesn't write back into the
		// state it renders from; the pointer's next move settles it.
		if (!token || !gridSize) {
			hoverLayer.batchDraw();
			return;
		}

		const text = hoverCardText(token);
		if (!text) {
			hoverLayer.batchDraw();
			return;
		}

		const label = new Konva.Label({
			// Bottom-centre of the card at the top-centre of the token, so it
			// stays put as the token's size changes.
			x: (token.x + token.width / 2) * gridSize,
			y: token.y * gridSize - screenToWorld(HOVER_LABEL_OFFSET),
			listening: false
		});
		label.add(
			new Konva.Tag({
				fill: HOVER_FILL,
				cornerRadius: screenToWorld(HOVER_LABEL_PADDING),
				pointerDirection: 'down',
				pointerWidth: screenToWorld(6),
				pointerHeight: screenToWorld(4)
			})
		);
		label.add(
			new Konva.Text({
				text,
				fontSize: screenToWorld(HOVER_FONT_SIZE),
				padding: screenToWorld(HOVER_LABEL_PADDING),
				align: 'center',
				lineHeight: 1.3,
				fill: 'white'
			})
		);
		hoverLayer.add(label);
		hoverLayer.batchDraw();
	}

	// --- the selection ring. Purely local: which token this client has
	// selected is never sent anywhere, so two people can be looking at
	// different tokens at the same time. ---

	type SelectionRing = { group: Konva.Group; circles: Konva.Circle[]; spin: Konva.Animation };

	// Konva bookkeeping for the ring currently on the map, and which token
	// it belongs to. Deliberately not $state: `selectedTokenId` is the
	// reactive truth, and these only exist so the ring can be kept alive
	// across re-renders instead of rebuilt.
	let selectionRing: SelectionRing | null = null;
	let selectionRingTokenId: string | null = null;

	function clearSelectionRing() {
		selectionRing?.spin.stop();
		selectionRing?.group.destroy();
		selectionRing = null;
		selectionRingTokenId = null;
		selectionLayer?.batchDraw();
	}

	function buildSelectionRing(tokenId: string) {
		const group = new Konva.Group({ listening: false });
		const circles = SELECTION_RING_LAYERS.map((spec) => {
			const circle = new Konva.Circle({ stroke: spec.color });
			group.add(circle);
			return circle;
		});
		selectionLayer.add(group);

		const spin = new Konva.Animation((frame) => {
			if (frame) group.rotation((frame.time / SELECTION_RING_PERIOD_MS) * 360);
		}, selectionLayer);
		spin.start();

		selectionRing = { group, circles, spin };
		selectionRingTokenId = tokenId;
	}

	// The ring is a sibling of the token rather than a child of it, so it
	// has to be told where the token went. Takes an id and checks it here
	// rather than at the call site: the token handlers that call this are
	// built in an effect that doesn't track the selection, so a comparison
	// made in one of their closures would be against a stale id.
	function moveSelectionRing(tokenId: string, x: number, y: number, w: number, h: number) {
		if (tokenId !== selectionRingTokenId || !selectionRing) return;
		selectionRing.group.position({ x: x + w / 2, y: y + h / 2 });
		selectionLayer.batchDraw();
	}

	function renderSelection() {
		if (!selectionLayer) return;

		const gridSize = room.scene?.gridSize;
		const token = selectedTokenId ? room.tokens.find((t) => t.id === selectedTokenId) : undefined;
		// A selected id with no token behind it — the scene changed under
		// it, or someone removed it — reads as nothing selected. The id is
		// left alone rather than cleared, so it isn't this function's job to
		// write back into the state it renders from.
		if (!token || !gridSize) {
			clearSelectionRing();
			return;
		}

		// Rebuilt only when the selection moves to a *different* token.
		// Re-creating it every time would restart the animation, and this
		// runs whenever anyone moves any token — so the ring would snap back
		// to its start angle every time someone else dragged something.
		if (selectionRingTokenId !== token.id) {
			clearSelectionRing();
			buildSelectionRing(token.id);
		}
		if (!selectionRing) return;

		const w = token.width * gridSize;
		const h = token.height * gridSize;
		// Screen pixels converted at the current zoom, so the ring keeps its
		// weight, its standoff and its dash spacing however far in you are.
		const radius = Math.min(w, h) / 2 + screenToWorld(SELECTION_RING_PADDING);
		// Round the period so a whole number of them closes the ring. The
		// cost is that the period flexes by a few percent, and that zooming
		// occasionally adds or drops one dash as the rounding crosses over —
		// both far less noticeable than a permanent seam.
		const circumference = 2 * Math.PI * radius;
		const periods = Math.max(
			1,
			Math.round(circumference / screenToWorld(SELECTION_RING_DASH_PERIOD))
		);
		const period = circumference / periods;
		const dash = [period * SELECTION_RING_DASH_INK, period * (1 - SELECTION_RING_DASH_INK)];
		selectionRing.circles.forEach((circle, i) => {
			circle.radius(radius);
			circle.dash(dash);
			circle.strokeWidth(screenToWorld(SELECTION_RING_LAYERS[i].width));
		});

		moveSelectionRing(token.id, token.x * gridSize, token.y * gridSize, w, h);
	}

	// --- sliding a token to its new square ---
	//
	// renderTokens rebuilds every group from scratch on any change to
	// room.tokens, so there is never a node still around to animate. The
	// way round that is to remember where each token was last *drawn* and
	// build the new group there, then tween it to where it now belongs —
	// which leaves the wholesale rebuild exactly as it was.
	//
	// This is not the selection ring's situation. That runs a
	// Konva.Animation for as long as something is selected, which is why
	// it earned a layer of its own; these tweens last a fifth of a second
	// and stop.

	// Last resting position drawn per token, in world units. Imperative
	// bookkeeping — nothing reactive reads it.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const renderedPositions = new Map<string, DrawingPoint>();
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const moveTweens = new Map<string, Konva.Tween>();

	function stopTokenMove(tokenId: string) {
		const tween = moveTweens.get(tokenId);
		if (!tween) return;
		// destroy() rather than stop(): the group it animates is about to
		// be destroyed by the re-render, and a tween left alive would go on
		// setting attributes on a detached node — and go on dragging the
		// selection ring around after it.
		tween.destroy();
		moveTweens.delete(tokenId);
	}

	function startTokenMove(
		group: Konva.Group,
		token: Token,
		to: DrawingPoint,
		w: number,
		h: number
	) {
		const tween = new Konva.Tween({
			node: group,
			duration: TOKEN_MOVE_SECONDS,
			x: to.x,
			y: to.y,
			easing: Konva.Easings.EaseInOut,
			// The ring is a sibling on another layer, so it has to be
			// carried along by hand or it arrives before the token does.
			onUpdate: () => moveSelectionRing(token.id, group.x(), group.y(), w, h),
			onFinish: () => {
				moveTweens.delete(token.id);
				moveSelectionRing(token.id, to.x, to.y, w, h);
			}
		});
		moveTweens.set(token.id, tween);
		tween.play();
	}

	// Someone who has asked for less motion gets the old instant jump.
	// matchMedia is read at render rather than cached so a change of the
	// system setting takes effect on the next move.
	function motionAllowed(): boolean {
		return !window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
	}

	// --- the drag preview: a ghost of the token at the square it was
	// picked up from, a line to the square it would land on, and how far
	// that is. Local to this browser like the selection ring and never
	// broadcast — "can I reach it before I let go" is the dragger's own
	// question, and one line per person mid-drag would be a busy map.
	//
	// It lives on previewLayer with the rubber-band shapes and the
	// eraser's halo, which is what that layer is for: transient overlays
	// belonging to a gesture in progress. That also keeps it above the
	// token layer, so the line isn't drawn under the token it's measuring.
	// ---

	// The drag in progress, or null. Holds the group so a zoom or a pan
	// mid-drag can redraw the preview at the new scale without waiting for
	// the pointer to move again, and the origin cell so the distance stays
	// measured from where the token actually started rather than from
	// wherever it has been dragged to since.
	let tokenDrag: { group: Konva.Group; token: Token; origin: Cell } | null = null;
	let dragGhost: Konva.Group | null = null;
	let dragLine: Konva.Line | null = null;
	let dragLabel: Konva.Label | null = null;

	function startTokenDragPreview(group: Konva.Group, token: Token, gridSize: number) {
		// A gesture that survived a stray second-button release comes back
		// through here, because re-arming the drag fires a second dragstart
		// (see the dragend handler). Rebuilding would blink the overlay for
		// a frame in the middle of a drag that never actually stopped —
		// and the ghost belongs to where the token was picked up, which
		// this second dragstart is no longer a witness to.
		if (tokenDrag?.token.id === token.id) return;

		clearTokenDragPreview();
		tokenDrag = { group, token, origin: { x: token.x, y: token.y } };

		// A clone rather than a second copy built from the token: its art
		// is an image, or a placeholder circle with initials, or a
		// placeholder that an image is still loading behind — and
		// duplicating that branch here is how the ghost starts disagreeing
		// with the token it's a ghost of.
		//
		// Cloning brings the drag and hover handlers with it, hence
		// `listening: false` as well as `draggable: false`: previewLayer
		// doesn't listen either, but a ghost that could be picked up is a
		// bad enough bug to refuse twice. The name goes because `.token` is
		// how a click finds a real one.
		dragGhost = group.clone({
			opacity: TOKEN_GHOST_OPACITY,
			draggable: false,
			listening: false,
			name: '',
			// Placed from the token's stored cell rather than from the
			// group's current position: by the time anything asks, the group
			// has moved.
			x: token.x * gridSize,
			y: token.y * gridSize
		});
		previewLayer.add(dragGhost);

		dragLine = new Konva.Line({ stroke: MEASURE_COLOR, listening: false });
		previewLayer.add(dragLine);

		updateTokenDragPreview();
	}

	function updateTokenDragPreview() {
		if (!tokenDrag) return;
		const gridSize = room.scene?.gridSize;
		if (!gridSize) return;

		const { group, token, origin } = tokenDrag;
		const preview = tokenDragPreview(origin, token, { x: group.x(), y: group.y() }, gridSize);

		// No line until it has left the square it started on: from a point
		// to itself it's a dot sitting under the token, and the label
		// already says 0 ft. Width and dash are re-set every update rather
		// than at creation because they're authored in screen pixels, and
		// the map can be zoomed or panned mid-drag.
		dragLine?.points(
			preview.moved ? [preview.from.x, preview.from.y, preview.to.x, preview.to.y] : []
		);
		dragLine?.strokeWidth(screenToWorld(MEASURE_LINE_WIDTH));
		dragLine?.dash([screenToWorld(8), screenToWorld(6)]);

		// Rebuilt rather than edited in place, the same way renderMeasurements
		// rebuilds its labels: the text, the position and three screen-pixel
		// sizes all change together, and reaching into a Konva.Label to set
		// them one at a time is more code for the same picture.
		dragLabel?.destroy();
		dragLabel = buildMeasureLabel(preview.label, preview.labelAt);
		previewLayer.add(dragLabel);

		previewLayer.batchDraw();
	}

	function clearTokenDragPreview() {
		if (!tokenDrag) return;
		tokenDrag = null;
		dragGhost?.destroy();
		dragGhost = null;
		dragLine?.destroy();
		dragLine = null;
		dragLabel?.destroy();
		dragLabel = null;
		previewLayer?.batchDraw();
	}

	// --- recency stacking. Which tokens have been touched on this screen,
	// oldest first, so the last one interacted with is drawn on top. ---
	//
	// Purely local and never sent anywhere: two people at the same table
	// have been handling different tokens, and each of them wants their
	// own on top. It isn't persisted either — a reload is a fresh screen
	// with nothing touched yet, which is creation order again.
	//
	// A plain array rather than $state: nothing reactive reads it, the
	// Konva layer is the thing it changes, and making it reactive would
	// give every effect that reads it a dependency on which token was last
	// poked.
	let raisedTokenIds: string[] = [];

	// Finds a token's group on the layer, or null while the layer is
	// between rebuilds. Matches on the same `.token` name and `tokenId`
	// attr the click handler walks up to.
	function tokenGroup(tokenId: string): Konva.Group | null {
		for (const node of tokenLayer.find<Konva.Group>('.token')) {
			if (node.getAttr('tokenId') === tokenId) return node;
		}
		return null;
	}

	// Brings a token to the top of the stack, and remembers that it is
	// there. Both halves matter: `moveToTop` alone is undone by the next
	// rebuild of the layer (someone else moving any token is enough), and
	// the list alone wouldn't move anything until that rebuild came.
	//
	// Called for a click and for the start of a drag, which are the two
	// pointer gestures a token has — Konva suppresses the click after a
	// real drag, so neither covers the other.
	function raiseToken(tokenId: string) {
		const at = raisedTokenIds.indexOf(tokenId);
		if (at !== -1) raisedTokenIds.splice(at, 1);
		raisedTokenIds.push(tokenId);

		const group = tokenGroup(tokenId);
		if (!group) return; // not rendered yet; the next rebuild lays it out
		group.moveToTop();
		tokenLayer.batchDraw();
	}

	function renderTokens(gridSize: number) {
		// Where each group has actually got to, for any slide still in
		// flight. A re-render mid-slide — someone else moving a different
		// token — would otherwise restart it from the resting position it
		// had already left, snapping the token backwards.
		// Local to this render and thrown away with it, so plain collections
		// rather than the reactive ones — nothing reads either of them
		// outside this function.
		// eslint-disable-next-line svelte/prefer-svelte-reactivity
		const midSlide = new Map<string, DrawingPoint>();
		// Selected by the same `.token` name the click handler walks up to,
		// which is also what makes these typed as groups rather than the
		// bare-node union getChildren() gives.
		for (const node of tokenLayer.find<Konva.Group>('.token')) {
			const id = node.getAttr('tokenId');
			if (typeof id === 'string' && moveTweens.has(id)) {
				midSlide.set(id, { x: node.x(), y: node.y() });
			}
		}
		for (const id of [...moveTweens.keys()]) stopTokenMove(id);

		tokenLayer.destroyChildren();

		// eslint-disable-next-line svelte/prefer-svelte-reactivity
		const present = new Set<string>();
		const animate = motionAllowed();

		for (const token of room.tokens) {
			present.add(token.id);
			const to = { x: token.x * gridSize, y: token.y * gridSize };
			const from = midSlide.get(token.id) ?? renderedPositions.get(token.id) ?? to;
			// A token that has only been renamed, or that is new to the
			// scene, has nothing to slide from.
			const moved = Math.abs(from.x - to.x) > 0.01 || Math.abs(from.y - to.y) > 0.01;
			const slide = animate && moved;
			renderedPositions.set(token.id, to);

			// The room's movement lock is taken away here as well as refused
			// by the hub. Without it a Player drags a locked token around,
			// the drop is refused, and the group sits where the pointer left
			// it until something forces a rebuild — a token that looks moved
			// to them and hasn't moved for anyone else.
			const movable = room.canMoveToken(token);

			const group = new Konva.Group({
				x: slide ? from.x : to.x,
				y: slide ? from.y : to.y,
				draggable: activeTool === 'none' && movable,
				// Named and tagged so a click on whatever is inside — the
				// image, or the placeholder circle and its initials — can be
				// walked back up to the token it landed on.
				name: 'token'
			});
			group.setAttr('tokenId', token.id);

			const w = token.width * gridSize;
			const h = token.height * gridSize;

			if (token.imageAssetId) {
				const src = assetUrl(token.imageAssetId);
				const cached = imageCache.get(src);
				if (cached) {
					addTokenImage(group, cached, w, h);
				} else {
					addTokenPlaceholder(group, token, w, h);
					loadImage(src).then((img) => {
						group.destroyChildren();
						addTokenImage(group, img, w, h);
						tokenLayer.batchDraw();
					});
				}
			} else {
				addTokenPlaceholder(group, token, w, h);
			}

			if (token.visibility === 'hidden') {
				group.opacity(0.55);
			}

			// Bound per group and re-bound on every rebuild, like the drag
			// handlers beside them. A rebuild while the pointer is resting on
			// a token fires no fresh mouseenter, which is why hoveredTokenId
			// lives outside this function and survives it — the card is
			// re-rendered from the new token either way.
			group.on('mouseenter', () => (hoveredTokenId = token.id));
			group.on('mouseleave', () => {
				if (hoveredTokenId === token.id) hoveredTokenId = null;
			});
			// A card floating over a token being dragged tracks a stale
			// position — the group moves under the pointer but the token's
			// stored square doesn't change until the drop.
			group.on('dragstart', () => {
				hoveredTokenId = null;
				// Raised as the drag starts rather than when it lands: a token
				// dragged out from under two others should be visible for the
				// whole journey, not only once it stops.
				raiseToken(token.id);
				startTokenDragPreview(group, token, gridSize);
			});

			// A locked token swallows the press rather than merely ignoring
			// it. Konva starts the *stage* drag from whatever pointerdown
			// bubbles up to it, so a token that simply isn't draggable hands
			// the gesture to the map and pans the whole scene — which reads
			// as the app misbehaving rather than as "this one isn't yours".
			// `click` is a separate event and still bubbles, so a locked
			// token can be selected and inspected as before.
			//
			// Guarded to the primary pointer, though: the stage drag it is
			// defending against is a left-button gesture, while a right- or
			// middle-button press on this token is someone panning the map
			// and has to reach the stage like it would anywhere else.
			// Swallowing everything here made locked tokens into holes in the
			// map that a pan couldn't start from.
			if (!movable) {
				group.on('mousedown.lock touchstart.lock', (e) => {
					if (!isPrimaryPointer(e)) return;
					e.cancelBubble = true;
				});
			}

			group.on('dragmove', () => {
				moveSelectionRing(token.id, group.x(), group.y(), w, h);
				updateTokenDragPreview();
			});

			group.on('dragend', (e) => {
				// Konva ends a drag on *any* mouseup anywhere on the page:
				// DD._endDragBefore is bound on window and never looks at which
				// button came up. So pressing and releasing the right button to
				// shove the map mid-drag used to end the token's drag silently
				// — the token stopped following the cursor and the eventual
				// left release committed it wherever it had frozen, several
				// squares short of where it was dropped. Nothing said so.
				//
				// The left button still being held says this release wasn't
				// ours. `buttons` is the live mask of what is down *now*
				// (bit 0 = left), which is exactly the question; `button`, the
				// one that changed, would only say "right" and can't tell a
				// right-release-mid-drag from a right-release-after-drop.
				//
				// Re-arming from in here works because of where Konva fires
				// this: _endDragAfter fires 'dragend' and only *then* drops the
				// drag element for anything no longer dragging. startDrag()
				// finds that element still present, flips it back to 'dragging'
				// before the check, and keeps its original offset — so the
				// token doesn't jump, and the gesture carries on as though
				// nothing happened.
				if (e.evt instanceof MouseEvent && (e.evt.buttons & 1) !== 0) {
					group.startDrag();
					return;
				}
				// Cleared before the snap rather than after, so nothing is left
				// pointing at the pre-snap position for a frame.
				clearTokenDragPreview();
				// The same rounding the preview promised — one function, so the
				// square the line ended on is the square that gets sent.
				const { x: cellX, y: cellY } = snapTokenCell({ x: group.x(), y: group.y() }, gridSize);
				group.x(cellX * gridSize);
				group.y(cellY * gridSize);
				// Recorded now rather than waiting for the echo. Whoever did
				// the dragging has already watched the token travel under
				// their own pointer; without this the broadcast would come
				// back, see the token still remembered at the square it left,
				// and slide it the whole way a second time.
				renderedPositions.set(token.id, { x: group.x(), y: group.y() });
				// Again after the snap, and not left to the broadcast: a drop
				// back onto the cell it started from is a no-op in RoomClient
				// (see token.moved), so no state change arrives to re-render
				// the ring — it would stay wherever the pointer let go.
				moveSelectionRing(token.id, group.x(), group.y(), w, h);
				room.moveToken(token.id, cellX, cellY);
			});

			tokenLayer.add(group);
			// Started after the group is on the layer, so the first frame
			// the tween draws has somewhere to land.
			if (slide) startTokenMove(group, token, to, w, h);
		}

		// A token that has left the scene shouldn't leave its last position
		// behind to be slid from if the same id ever comes back — which an
		// undone deletion does, under exactly the same id.
		for (const id of [...renderedPositions.keys()]) {
			if (!present.has(id)) renderedPositions.delete(id);
		}

		// The rebuild above put every group back in `room.tokens` order, so
		// the recency order has to be laid on again — oldest touched first,
		// each one lifted over the last, which leaves the most recent on
		// top. Doing it here rather than sorting the loop above keeps this a
		// handful of `moveToTop` calls on the tokens anyone has actually
		// touched, and leaves everything else in the order it was created.
		raisedTokenIds = raisedTokenIds.filter((id) => present.has(id));
		for (const id of raisedTokenIds) tokenGroup(id)?.moveToTop();

		tokenLayer.batchDraw();

		// The rebuild above destroyed every group, and Konva's idea of what
		// the pointer is over went with them: the old group can't fire
		// mouseleave because it no longer exists, and the new one fires no
		// mouseenter because the pointer never moved. Left alone,
		// hoveredTokenId keeps whatever it had — so a card could outlive the
		// pointer resting on its token, and moving away would never clear it.
		// Re-deriving it from where the pointer actually is fixes both
		// directions at once.
		//
		// This only shows up when the pointer stays *inside* the stage: the
		// stage's own mouseleave used to paper over it, which is why it went
		// unnoticed until the map filled the window and there was much less
		// outside to move to.
		syncHoverToPointer();
	}

	// The token under the pointer right now, or null. Reads the layer
	// rather than the tokens array, so it agrees with what Konva would
	// dispatch a mouse event to — including a token hidden behind another.
	function syncHoverToPointer() {
		const pointer = stage?.getPointerPosition();
		if (!pointer) return;
		const hit = tokenLayer.getIntersection(pointer);
		const group = hit?.findAncestor('.token', true) as Konva.Group | undefined;
		hoveredTokenId = (group?.getAttr('tokenId') as string | undefined) ?? null;
	}

	function addTokenImage(group: Konva.Group, img: HTMLImageElement, w: number, h: number) {
		group.add(new Konva.Image({ image: img, width: w, height: h, cornerRadius: w / 2 }));
	}

	function addTokenPlaceholder(group: Konva.Group, token: Token, w: number, h: number) {
		group.add(
			new Konva.Circle({
				x: w / 2,
				y: h / 2,
				radius: Math.min(w, h) / 2,
				fill: colorFor(token.id)
			})
		);
		group.add(
			new Konva.Text({
				text: token.name.slice(0, 2).toUpperCase(),
				width: w,
				height: h,
				align: 'center',
				verticalAlign: 'middle',
				fill: 'white',
				fontStyle: 'bold'
			})
		);
	}
</script>

<!-- Fills whatever box the room page gives it, which since the
     full-bleed layout is the whole window. No border or rounding: there
     is no card around the map any more, and a 1px edge on a stage that
     reaches every side of the screen only ever reads as a rendering
     artefact. -->
<div bind:this={container} class="h-full w-full overflow-hidden bg-muted"></div>
