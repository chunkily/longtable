<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import Konva from 'konva';
	import { assetUrl } from '$lib/api';
	import { DRAWING_STROKE_WIDTH, pickDrawing, strokeWidthOf } from '$lib/drawing-hit';
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
	import { cellAt, cellCentre, measureLabel } from '$lib/measure';
	import { PING_PULSES, PING_PULSE_INTERVAL_MS, PING_PULSE_SECONDS } from '$lib/ping';
	import type {
		Drawing,
		DrawingKind,
		DrawingPoint,
		Measurement,
		Ping,
		RoomClient,
		Token
	} from '$lib/room.svelte';

	// 'none' is plain pan/token-drag mode. Every other tool takes over
	// the stage's pointer handling exclusively — only one can be active
	// at a time, since they all interpret a left-drag differently.
	export type Tool =
		| 'none'
		| 'fog-reveal'
		| 'fog-hide'
		| DrawingKind
		| 'ping'
		| 'eraser'
		| 'measure'
		| 'template-circle'
		| 'template-cone'
		| 'template-line'
		| 'template-cube';

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
		strokeColor = '#000000',
		snapMode = 'intersections',
		lineWidthFeet = DEFAULT_LINE_WIDTH_FEET,
		selectedTokenId = $bindable(null)
	}: {
		room: RoomClient;
		activeTool?: Tool;
		strokeColor?: string;
		/** Where template points may land. Purely a local input aid. */
		snapMode?: SnapMode;
		lineWidthFeet?: number;
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
	// Measurement lines are one colour for everyone rather than per
	// participant: they're transient, and at most one per person is on
	// the map at a time with their name on it.
	const MEASURE_COLOR = '#0ea5e9';
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
	let resizeObserver: ResizeObserver | undefined;

	// Tracks the active scene so a switch to a different scene resets
	// the camera, rather than carrying over an unrelated pan/zoom.
	let lastSceneId: string | null = null;

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
		stage.add(
			mapLayer,
			gridLayer,
			fogLayer,
			drawingLayer,
			tokenLayer,
			pingLayer,
			measureLayer,
			previewLayer,
			selectionLayer
		);

		stage.on('wheel', handleWheel);
		stage.on('dragmove', () => renderGrid());

		resizeObserver = new ResizeObserver(() => {
			if (!stage) return;
			stage.width(container.clientWidth);
			stage.height(container.clientHeight);
			renderGrid();
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
		// planning/backlog/done/duplicate-map-image.md.
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
		for (const marker of pingMarkers.values()) destroyPingMarker(marker);
		pingMarkers.clear();
		// Before the stage goes: a running Konva.Animation would otherwise
		// keep asking a destroyed layer to redraw itself every frame.
		clearSelectionRing();
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
		stage.batchDraw();
		renderGrid();
		// Measurement lines and labels are sized in screen pixels, so a
		// zoom changes what they should be in world units. The selection
		// ring's stroke, dash and standoff are the same.
		renderMeasurements();
		renderSelection();
		refreshCursorOverlay();
	}

	// Resets the camera to its identity transform (pan at the origin,
	// no zoom) — the same view a scene starts in, since the map's
	// origin (0,0) is always its top-left corner. Exposed for the
	// "Reset view" button in the room page via bind:this.
	export function resetView() {
		if (!stage) return;
		stage.scale({ x: 1, y: 1 });
		stage.position({ x: 0, y: 0 });
		stage.batchDraw();
		renderGrid();
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

	// Deliberately not tracking room.drawings or room.tokens: either one
	// would rebuild the map image, every grid line and every fog cell for
	// a change that only touched a single stroke or a single token's
	// position — and drawing, erasing and dragging tokens are the most
	// frequent things that happen to a scene. They get their own effects
	// below.
	$effect(() => {
		track(room.scene, room.fogCells, room.you);
		render();
	});

	$effect(() => {
		track(room.drawings);
		if (stage) renderDrawings();
	});

	// activeTool is tracked here, not only by the handler effect below,
	// because tokens are draggable in 'none' mode alone — switching to
	// any tool has to re-render them to take that away. It is tracked
	// *here* rather than alongside render() because token draggability is
	// the only thing in the whole render path that reads it, so pairing
	// it with the full rebuild made every tool switch redraw the map too.
	$effect(() => {
		track(room.tokens, room.scene, activeTool);
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

	// The eraser's halo points at a specific drawing, so it has to be
	// re-resolved whenever the set of drawings changes — otherwise it
	// hangs over the empty space where a stroke used to be (including
	// one someone else just erased) until the pointer next moves.
	$effect(() => {
		track(room.drawings, room.you, activeTool);
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

	// --- pointer-driven tools: fog paint, freehand/line/rect/circle
	// drawing, and ping. Exactly one owns the stage's pointer at a time. ---

	let painting = false;
	// Transient drag-stroke bookkeeping, cleared every stroke — not
	// template state, so a plain Map (not SvelteMap) is correct.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	let pendingCells = new Map<string, { x: number; y: number }>();

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
			updateCursorRing(DRAWING_STROKE_WIDTH / 2);
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

	function attachToolHandlers() {
		if (!stage) return;
		stage.off(
			'mousedown.tool touchstart.tool mousemove.tool touchmove.tool mouseup.tool touchend.tool mouseleave.tool'
		);
		painting = false;
		pendingCells.clear();
		stopErasing();
		// Switching tools mid-drag has to retract the measurement, or it
		// stays on every other map with no end event ever coming.
		stopMeasuring();
		clearPreview();
		clearCursorOverlay();

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

		// Revealing and hiding are the same gesture over the same cells in
		// opposite directions, so they share a handler and differ only in
		// which command the sweep ends with.
		if (activeTool === 'fog-reveal' || activeTool === 'fog-hide') {
			if (room.you?.role !== 'gm') return; // fog stays GM-only
			const hiding = activeTool === 'fog-hide';
			stage.on('mousedown.tool touchstart.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				painting = true;
				paintAtPointer(gridSize, hiding);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				if (painting) paintAtPointer(gridSize, hiding);
			});
			stage.on('mouseup.tool touchend.tool', (e) => {
				if (!isPrimaryPointer(e)) return;
				if (!painting) return;
				painting = false;
				if (pendingCells.size > 0) {
					const cells = Array.from(pendingCells.values());
					// A sweep sends every cell it crossed, including ones already
					// in the target state — both commands are idempotent server
					// side, which is what lets the gesture stay this simple.
					if (hiding) room.hideFog(sceneId, cells);
					else room.revealFog(sceneId, cells);
				}
				pendingCells.clear();
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
					strokeWidth: DRAWING_STROKE_WIDTH,
					lineCap: 'round',
					lineJoin: 'round',
					dash: [6, 4],
					listening: false
				});
				previewLayer.add(previewShape);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				// The ring tracks the pointer whether or not a stroke is in
				// progress: it's showing how wide the line will be.
				updateCursorRing(DRAWING_STROKE_WIDTH / 2);
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
					room.createDrawing(sceneId, 'freehand', freehandPoints, strokeColor);
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
					room.createDrawing(sceneId, kind, [drawStart, pos], strokeColor);
				}
			}
			clearPreview();
		});
	}

	function buildPreviewShape(
		kind: 'line' | 'rect' | 'ellipse',
		a: DrawingPoint,
		b: DrawingPoint
	): Konva.Shape {
		const strokeProps = { stroke: strokeColor, strokeWidth: 2, dash: [6, 4], listening: false };
		switch (kind) {
			case 'line':
				return new Konva.Line({ ...lineGeometry(a, b), ...strokeProps });
			case 'rect':
				return new Konva.Rect({ ...rectGeometry(a, b), ...strokeProps });
			case 'ellipse':
				return new Konva.Ellipse({ ...ellipseGeometry(a, b), ...strokeProps });
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

	// The two fog tools mark their swept cells in different colours so a
	// GM mid-drag can tell which direction they're painting in. Neither
	// is what the cell will end up looking like — that arrives with the
	// broadcast, which re-renders the layer from scratch.
	const FOG_REVEAL_PREVIEW = 'yellow';
	const FOG_HIDE_PREVIEW = '#dc2626';

	function paintAtPointer(gridSize: number, hiding: boolean) {
		// Pointer position adjusted for the stage's own pan/zoom, so
		// painting still lands on the right cell after the camera moves.
		const pos = stage?.getRelativePointerPosition();
		if (!pos) return;
		const cell = { x: Math.floor(pos.x / gridSize), y: Math.floor(pos.y / gridSize) };
		const key = `${cell.x},${cell.y}`;
		if (pendingCells.has(key)) return;
		pendingCells.set(key, cell);

		fogLayer.add(
			new Konva.Rect({
				x: cell.x * gridSize,
				y: cell.y * gridSize,
				width: gridSize,
				height: gridSize,
				fill: hiding ? FOG_HIDE_PREVIEW : FOG_REVEAL_PREVIEW,
				opacity: 0.35,
				listening: false
			})
		);
		fogLayer.batchDraw();
	}

	async function render() {
		if (!stage) return;
		const scene = room.scene;

		const sceneId = scene?.id ?? null;
		if (sceneId !== lastSceneId) {
			lastSceneId = sceneId;
			resetView();
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
		renderFog(scene.gridSize, width, height);
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
					mapLayer.add(new Konva.Rect({ width, height, fill: '#3f3f46' }));
				}
			} else {
				mapLayer.add(new Konva.Rect({ width, height, fill: '#e4e4e7' }));
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

		for (let x = startX; x <= viewRight; x += gridSize) {
			gridLayer.add(
				new Konva.Line({
					points: [x, viewTop, x, viewBottom],
					stroke: '#00000022',
					strokeWidth
				})
			);
		}
		for (let y = startY; y <= viewBottom; y += gridSize) {
			gridLayer.add(
				new Konva.Line({
					points: [viewLeft, y, viewRight, y],
					stroke: '#00000022',
					strokeWidth
				})
			);
		}

		gridLayer.batchDraw();
	}

	function renderFog(gridSize: number, width: number, height: number) {
		fogLayer.destroyChildren();
		if (width <= 0 || height <= 0) {
			fogLayer.batchDraw();
			return;
		}

		const isGM = room.you?.role === 'gm';
		const cover = new Konva.Rect({
			width,
			height,
			fill: 'black',
			opacity: isGM ? 0.35 : 1,
			listening: false
		});
		fogLayer.add(cover);

		if (!isGM) {
			// players never see through fog at all, so revealed cells are
			// punched out of the cover entirely
			for (const cell of room.fogCells) {
				fogLayer.add(
					new Konva.Rect({
						x: cell.x * gridSize,
						y: cell.y * gridSize,
						width: gridSize,
						height: gridSize,
						fill: 'black',
						globalCompositeOperation: 'destination-out',
						listening: false
					})
				);
			}
		} else {
			// GM sees everything; revealed cells just get a lighter tint so
			// they can tell what's currently visible to players
			for (const cell of room.fogCells) {
				fogLayer.add(
					new Konva.Rect({
						x: cell.x * gridSize,
						y: cell.y * gridSize,
						width: gridSize,
						height: gridSize,
						fill: 'black',
						opacity: 0.35,
						globalCompositeOperation: 'destination-out',
						listening: false
					})
				);
			}
		}

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
				return new Konva.Rect({ ...rectGeometry(d.points[0], d.points[1]), ...strokeProps });
			case 'ellipse':
				return new Konva.Ellipse({ ...ellipseGeometry(d.points[0], d.points[1]), ...strokeProps });
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

		for (let i = 0; i < PING_PULSES; i++) {
			const ring = new Konva.Circle({
				x: ping.x,
				y: ping.y,
				radius: 6,
				stroke: '#f59e0b',
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
		measureLayer.add(group);
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

	function renderTokens(gridSize: number) {
		tokenLayer.destroyChildren();

		for (const token of room.tokens) {
			const group = new Konva.Group({
				x: token.x * gridSize,
				y: token.y * gridSize,
				draggable: activeTool === 'none',
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

			group.on('dragmove', () => moveSelectionRing(token.id, group.x(), group.y(), w, h));

			group.on('dragend', () => {
				const cellX = Math.round(group.x() / gridSize);
				const cellY = Math.round(group.y() / gridSize);
				group.x(cellX * gridSize);
				group.y(cellY * gridSize);
				// Again after the snap, and not left to the broadcast: a drop
				// back onto the cell it started from is a no-op in RoomClient
				// (see token.moved), so no state change arrives to re-render
				// the ring — it would stay wherever the pointer let go.
				moveSelectionRing(token.id, group.x(), group.y(), w, h);
				room.moveToken(token.id, cellX, cellY);
			});

			tokenLayer.add(group);
		}

		tokenLayer.batchDraw();
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

<div
	bind:this={container}
	class="h-[70vh] min-h-[480px] w-full overflow-hidden rounded-md border bg-muted"
></div>
