import type { CSSProperties } from 'react';

// Accent colours at runtime. Colours in CSS come only from tokens (rule 9); the two exceptions are
// data, set as custom properties whose every use carries a fallback (a token, or the keyword
// `transparent` where no colour is wanted):
//
// - `--sc-accent` / `--sc-on-accent`: the active profile's accent. The profile names one of the
//   token accents (`accentNames`); the value is a reference to that token, never a literal colour.
// - `--sc-item-accent`: a title's own colour from its poster (ItemRef.accent, image pipeline),
//   used for the poster card glow and the detail backdrop tint. CSS reads it as
//   `var(--sc-item-accent, <token>)` or `var(--sc-item-accent, transparent)` (no accent, no
//   tint). Every element that sets it from the inline style resets it to `initial`
//   in its own rule, so an accent never leaks into a nested title. It is deliberately not an
//   `@property` with `inherits: false`: the glows and tints are pseudo-elements and children of
//   the element carrying the style, and a registered initial value would bypass the fallbacks.

const HEX_COLOUR = /^#[0-9a-f]{6}$/i;

/** Inline style that hands a title's accent to the element and its children, if it is valid. */
export function itemAccentStyle(accent: string | undefined): CSSProperties | undefined {
  if (!accent || !HEX_COLOUR.test(accent)) return undefined;
  return { '--sc-item-accent': accent.toLowerCase() } as CSSProperties;
}
