import { describe, expect, it } from 'vitest';
import { iconSvg } from './token-icon-image';

// The canvas can't draw a component, so an icon goes through its SVG text.
// What matters is that the text is a standalone, drawable SVG in the ink
// it was asked for — an image decoder is less forgiving than the DOM.
describe('iconSvg', () => {
	it('renders a standalone SVG in the requested ink', () => {
		const svg = iconSvg('lucide:skull', '#18181b')!;
		expect(svg.startsWith('<svg')).toBe(true);
		expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"');
		expect(svg).toContain('stroke="#18181b"');
		expect(svg).toContain('<path');
		expect(svg).not.toContain('<!--');
	});

	it('gives nothing for a key that is not an icon', () => {
		expect(iconSvg('', '#ffffff')).toBeNull();
		expect(iconSvg('lucide:dragon', '#ffffff')).toBeNull();
	});
});
