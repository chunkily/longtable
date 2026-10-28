// An icon as an image Konva can draw.
//
// Konva paints into a canvas, which can't hold a Svelte component, and
// the Lucide package exposes its icons only as components — the path data
// is private to each. So the component is mounted once into a detached
// element, its SVG read back as text, and that text loaded as an image.
// Cached per icon and ink: a room of goblins draws the skull once.
import { mount, unmount } from 'svelte';
import { tokenIcon } from './token-icons';

/**
 * Drawn large and scaled down by Konva, so the stroke stays crisp on a
 * Gargantuan token at full zoom.
 */
const ICON_PIXELS = 128;

/** The SVG text for an icon in `ink`, or null for a key that isn't one. */
export function iconSvg(key: string, ink: string): string | null {
	const icon = tokenIcon(key);
	if (!icon) return null;
	const host = document.createElement('div');
	const instance = mount(icon.icon, {
		target: host,
		props: { size: ICON_PIXELS, color: ink, strokeWidth: 2.25 }
	});
	let svg = host.innerHTML;
	unmount(instance);
	// An image loaded from a data URL is parsed as a standalone document,
	// which needs the namespace an inline <svg> in HTML gets for free.
	if (!svg.includes('xmlns=')) svg = svg.replace('<svg', '<svg xmlns="http://www.w3.org/2000/svg"');
	// Svelte leaves comment markers around what it renders; they mean
	// nothing to an image decoder, but nor do they need to go along.
	return svg.replace(/<!--.*?-->/g, '').trim();
}

// Plain Maps — this is a .ts module and nothing reactive reads them.
// `cache` holds the loads, `ready` the images
// that have finished — which is what lets a re-render draw an icon it has
// drawn before in the same frame, rather than blank for one tick while an
// already-settled promise resolves.
const cache = new Map<string, Promise<HTMLImageElement>>();
const ready = new Map<string, HTMLImageElement>();

/** The icon's image if it has already loaded, else undefined. */
export function loadedIconImage(key: string, ink: string): HTMLImageElement | undefined {
	return ready.get(`${key}|${ink}`);
}

/** The icon as a loaded image, or null if the key isn't an icon. */
export function iconImage(key: string, ink: string): Promise<HTMLImageElement> | null {
	const cacheKey = `${key}|${ink}`;
	const hit = cache.get(cacheKey);
	if (hit) return hit;
	const svg = iconSvg(key, ink);
	if (!svg) return null;
	const loading = new Promise<HTMLImageElement>((resolve, reject) => {
		const img = new Image();
		img.onload = () => {
			ready.set(cacheKey, img);
			resolve(img);
		};
		img.onerror = () => reject(new Error(`icon ${key} failed to load`));
		img.src = `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
	});
	cache.set(cacheKey, loading);
	// A failure is forgotten, so the next render tries again rather than
	// the icon staying blank for the rest of the session.
	loading.catch(() => cache.delete(cacheKey));
	return loading;
}
