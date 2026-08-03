// Turning "where the map's squares actually start" into the padding the
// server bakes into the stored image.
//
// The canvas draws its grid from the world origin in multiples of the
// scene's grid size, and has no offset to apply — deliberately, per the
// room-member-align-map-grid-offset story. So alignment means moving the
// art instead: pad the left and top until the art's own squares land on
// those multiples.

/**
 * Pixels to pad onto one edge so a line at `origin` moves to a multiple
 * of `gridSize`.
 *
 * Padding rather than cropping, so nothing is thrown away — a map whose
 * squares start 12px in gets 58px of transparency on the left rather
 * than losing the 12px strip. The modulo runs twice because JavaScript's
 * `%` keeps the sign of its left operand, and an origin dragged past a
 * square boundary (or below zero) would otherwise produce a negative pad,
 * which is a crop.
 */
export function paddingForOrigin(gridSize: number, origin: number): number {
	if (!Number.isFinite(gridSize) || gridSize <= 0) return 0;
	const within = ((Math.round(origin) % gridSize) + gridSize) % gridSize;
	return (gridSize - within) % gridSize;
}
