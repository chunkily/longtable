// Timing for pointer pings, in one place because it's shared across a
// boundary that would otherwise hide a bug: the canvas animates the
// pulses, but RoomClient decides how long the ping stays in state, and
// if that were shorter than the animation the marker would vanish
// mid-sequence.

/** Pulses per ping — a single flash is easy to miss if you glanced away. */
export const PING_PULSES = 3;
export const PING_PULSE_INTERVAL_MS = 1000;
export const PING_PULSE_SECONDS = 1.2;

/** How long a ping marker lives: until its last pulse has finished. */
export const PING_LIFETIME_MS =
	(PING_PULSES - 1) * PING_PULSE_INTERVAL_MS + PING_PULSE_SECONDS * 1000;

// Ignore pings sent within this of the last one. Not a defence against a
// hostile client — nothing here is, and the server applies no limit of
// its own — but it stops an impatient double-click from stacking markers
// on top of the very thing they're pointing at.
export const PING_COOLDOWN_MS = 1000;
