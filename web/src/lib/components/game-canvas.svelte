<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import Konva from 'konva';
	import { assetUrl } from '$lib/api';
	import { DRAWING_STROKE_WIDTH, pickDrawing, strokeWidthOf } from '$lib/drawing-hit';
	import type { Drawing, DrawingKind, DrawingPoint, RoomClient, Token } from '$lib/room.svelte';

	// 'none' is plain pan/token-drag mode. Every other tool takes over
	// the stage's pointer handling exclusively — only one can be active
	// at a time, since they all interpret a left-drag differently.
	export type Tool = 'none' | 'fog' | DrawingKind | 'ping' | 'eraser';

	let {
		room,
		activeTool = 'none',
		strokeColor = '#000000'
	}: {
		room: RoomClient;
		activeTool?: Tool;
		strokeColor?: string;
	} = $props();

	const MIN_SCALE = 0.2;
	const MAX_SCALE = 4;
	const ZOOM_STEP = 1.05;
	// Minimum world-space distance between consecutive freehand points —
	// keeps strokes from accumulating an unbounded number of points
	// while the pointer is held down.
	const MIN_FREEHAND_SPACING = 3;
	const PING_TWEEN_SECONDS = 1.4;
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

	let container: HTMLDivElement;
	let stage: Konva.Stage | undefined;
	let mapLayer: Konva.Layer;
	let gridLayer: Konva.Layer;
	let fogLayer: Konva.Layer;
	let drawingLayer: Konva.Layer;
	let tokenLayer: Konva.Layer;
	let pingLayer: Konva.Layer;
	let previewLayer: Konva.Layer;
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
		previewLayer = new Konva.Layer({ listening: false });
		stage.add(mapLayer, gridLayer, fogLayer, drawingLayer, tokenLayer, pingLayer, previewLayer);

		stage.on('wheel', handleWheel);
		stage.on('dragmove', () => renderGrid());

		resizeObserver = new ResizeObserver(() => {
			if (!stage) return;
			stage.width(container.clientWidth);
			stage.height(container.clientHeight);
			renderGrid();
		});
		resizeObserver.observe(container);

		render();
	});

	onDestroy(() => {
		resizeObserver?.disconnect();
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

	// activeTool is tracked here too, not just by the handler effect
	// below: tokens are only draggable in 'none' mode, so switching to
	// any tool has to re-render them to take that away.
	$effect(() => {
		track(room.scene, room.tokens, room.fogCells, room.drawings, room.you, activeTool);
		render();
	});

	$effect(() => {
		track(activeTool, room.scene, room.you);
		attachToolHandlers();
	});

	$effect(() => {
		track(room.pings);
		renderPings();
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

	// --- cursor overlay: a ring showing the tool's reach, plus (for the
	// eraser) a halo on the stroke a click would remove. Both live on
	// previewLayer, which doesn't listen for events and is above
	// everything else. ---

	let cursorRing: Konva.Circle | null = null;
	let eraseHighlight: Konva.Shape | null = null;
	let highlightedDrawingId: string | null = null;

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

	// The drawing an eraser click would remove right now: the nearest
	// one within reach, considering only drawings this participant is
	// allowed to erase — so someone else's stroke lying closer doesn't
	// mask your own, and clicking it does nothing rather than erroring.
	function eraserTargetAtPointer(): Drawing | null {
		const pos = stage?.getRelativePointerPosition();
		if (!pos) return null;
		return pickDrawing(room.drawings.filter(canErase), pos, screenToWorld(ERASER_PICK_RADIUS));
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

	function attachToolHandlers() {
		if (!stage) return;
		stage.off(
			'mousedown.tool touchstart.tool mousemove.tool touchmove.tool mouseup.tool touchend.tool mouseleave.tool'
		);
		painting = false;
		pendingCells.clear();
		clearPreview();
		clearCursorOverlay();

		const scene = room.scene;
		const isActive = activeTool !== 'none' && !!scene;
		// Tools and panning both start on left-drag, so only one can own
		// the gesture at a time.
		stage.draggable(!isActive);
		if (!isActive || !scene) return;

		const gridSize = scene.gridSize;
		const sceneId = scene.id;

		if (activeTool === 'fog') {
			if (room.you?.role !== 'gm') return; // fog stays GM-only
			stage.on('mousedown.tool touchstart.tool', () => {
				painting = true;
				paintAtPointer(gridSize);
			});
			stage.on('mousemove.tool touchmove.tool', () => {
				if (painting) paintAtPointer(gridSize);
			});
			stage.on('mouseup.tool touchend.tool', () => {
				if (!painting) return;
				painting = false;
				if (pendingCells.size > 0) {
					room.revealFog(sceneId, Array.from(pendingCells.values()));
				}
				pendingCells.clear();
			});
			return;
		}

		if (activeTool === 'eraser') {
			stage.on('mousedown.tool touchstart.tool', () => {
				const target = eraserTargetAtPointer();
				if (target) room.deleteDrawing(target.id);
				// Whatever was highlighted is either gone or no longer
				// under the pointer, so re-resolve from where we are now.
				updateEraserCursor();
			});
			stage.on('mousemove.tool touchmove.tool', () => updateEraserCursor());
			stage.on('mouseleave.tool touchend.tool', () => clearCursorOverlay());
			return;
		}

		if (activeTool === 'ping') {
			stage.on('mousedown.tool touchstart.tool', () => {
				const pos = stage!.getRelativePointerPosition();
				if (!pos) return;
				room.sendPing(sceneId, pos.x, pos.y);
			});
			return;
		}

		if (activeTool === 'freehand') {
			stage.on('mousedown.tool touchstart.tool', () => {
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
			stage.on('mouseup.tool touchend.tool', () => {
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
		const kind = activeTool;
		stage.on('mousedown.tool touchstart.tool', () => {
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
		stage.on('mouseup.tool touchend.tool', () => {
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

	function paintAtPointer(gridSize: number) {
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
				fill: 'yellow',
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
	// room.pings on its own after a short lifetime, which is what
	// actually clears its shape here (via the "no longer present"
	// branch below) — the tween is just the fade-out look, not the
	// mechanism that ends the ping.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	const pingShapes = new Map<string, Konva.Circle>();

	function renderPings() {
		if (!pingLayer) return;

		const currentIds = new Set(room.pings.map((p) => p.id));
		for (const [id, shape] of pingShapes) {
			if (!currentIds.has(id)) {
				shape.destroy();
				pingShapes.delete(id);
			}
		}

		for (const ping of room.pings) {
			if (pingShapes.has(ping.id)) continue;
			const circle = new Konva.Circle({
				x: ping.x,
				y: ping.y,
				radius: 6,
				stroke: '#f59e0b',
				strokeWidth: 3,
				listening: false
			});
			pingLayer.add(circle);
			pingShapes.set(ping.id, circle);
			new Konva.Tween({
				node: circle,
				duration: PING_TWEEN_SECONDS,
				radius: 40,
				opacity: 0,
				easing: Konva.Easings.EaseOut
			}).play();
		}

		pingLayer.batchDraw();
	}

	function renderTokens(gridSize: number) {
		tokenLayer.destroyChildren();

		for (const token of room.tokens) {
			const group = new Konva.Group({
				x: token.x * gridSize,
				y: token.y * gridSize,
				draggable: activeTool === 'none'
			});

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

			group.on('dragend', () => {
				const cellX = Math.round(group.x() / gridSize);
				const cellY = Math.round(group.y() / gridSize);
				group.x(cellX * gridSize);
				group.y(cellY * gridSize);
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
