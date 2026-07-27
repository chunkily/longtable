<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import Konva from 'konva';
	import { assetUrl } from '$lib/api';
	import type { RoomClient, Token } from '$lib/room.svelte';

	let {
		room,
		fogToolActive = false
	}: {
		room: RoomClient;
		fogToolActive?: boolean;
	} = $props();

	const DEFAULT_SIZE = 800;

	let container: HTMLDivElement;
	let stage: Konva.Stage | undefined;
	let mapLayer: Konva.Layer;
	let fogLayer: Konva.Layer;
	let tokenLayer: Konva.Layer;

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
		stage = new Konva.Stage({ container, width: DEFAULT_SIZE, height: DEFAULT_SIZE });
		mapLayer = new Konva.Layer();
		fogLayer = new Konva.Layer();
		tokenLayer = new Konva.Layer();
		stage.add(mapLayer, fogLayer, tokenLayer);
		render();
	});

	onDestroy(() => {
		stage?.destroy();
	});

	// render() is async and awaits partway through (loading the map
	// image), so Svelte's $effect dependency tracking — which only sees
	// reads that happen synchronously before the first await — would
	// otherwise miss room.tokens/fogCells/you entirely. track() forces
	// them to be read up front, inside the effect's synchronous window.
	function track(...values: unknown[]) {
		return values.length;
	}

	$effect(() => {
		track(room.scene, room.tokens, room.fogCells, room.you);
		render();
	});

	$effect(() => {
		track(fogToolActive, room.scene, room.you);
		attachFogPaintHandlers();
	});

	// --- fog-of-war paint tool (GM only) ---

	let painting = false;
	// Transient drag-stroke bookkeeping, cleared every stroke — not
	// template state, so a plain Map (not SvelteMap) is correct.
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	let pendingCells = new Map<string, { x: number; y: number }>();

	function attachFogPaintHandlers() {
		if (!stage) return;
		stage.off('mousedown.fog touchstart.fog mousemove.fog touchmove.fog mouseup.fog touchend.fog');
		painting = false;
		pendingCells.clear();

		const scene = room.scene;
		if (!fogToolActive || room.you?.role !== 'gm' || !scene) return;
		const gridSize = scene.gridSize;
		const sceneId = scene.id;

		stage.on('mousedown.fog touchstart.fog', () => {
			painting = true;
			paintAtPointer(gridSize);
		});
		stage.on('mousemove.fog touchmove.fog', () => {
			if (painting) paintAtPointer(gridSize);
		});
		stage.on('mouseup.fog touchend.fog', () => {
			if (!painting) return;
			painting = false;
			if (pendingCells.size > 0) {
				room.revealFog(sceneId, Array.from(pendingCells.values()));
			}
			pendingCells.clear();
		});
	}

	function paintAtPointer(gridSize: number) {
		const pos = stage?.getPointerPosition();
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

		if (!scene) {
			stage.width(DEFAULT_SIZE);
			stage.height(DEFAULT_SIZE);
			mapLayer.destroyChildren();
			fogLayer.destroyChildren();
			tokenLayer.destroyChildren();
			mapLayer.draw();
			fogLayer.draw();
			tokenLayer.draw();
			return;
		}

		const width = scene.width || DEFAULT_SIZE;
		const height = scene.height || DEFAULT_SIZE;
		stage.width(width);
		stage.height(height);

		await renderMap(scene.mapAssetId, width, height, scene.gridSize);
		renderFog(scene.gridSize, width, height);
		renderTokens(scene.gridSize);
	}

	async function renderMap(
		mapAssetId: string | null,
		width: number,
		height: number,
		gridSize: number
	) {
		mapLayer.destroyChildren();

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

		for (let x = 0; x <= width; x += gridSize) {
			mapLayer.add(
				new Konva.Line({ points: [x, 0, x, height], stroke: '#00000022', strokeWidth: 1 })
			);
		}
		for (let y = 0; y <= height; y += gridSize) {
			mapLayer.add(
				new Konva.Line({ points: [0, y, width, y], stroke: '#00000022', strokeWidth: 1 })
			);
		}

		mapLayer.batchDraw();
	}

	function renderFog(gridSize: number, width: number, height: number) {
		fogLayer.destroyChildren();

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

	function renderTokens(gridSize: number) {
		tokenLayer.destroyChildren();

		for (const token of room.tokens) {
			const group = new Konva.Group({
				x: token.x * gridSize,
				y: token.y * gridSize,
				draggable: !fogToolActive
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

<div bind:this={container} class="inline-block overflow-auto rounded-md border"></div>
