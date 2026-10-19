import type { CSSProperties } from 'react';

// Accent colours at runtime. Colours in CSS come only from tokens (rule 9); the two exceptions are
// data, set as custom properties whose every use carries a token fallback:
//
// - `--sc-accent` / `--sc-on-accent`: the active profile's accent. The profile names one of the
//   token accents (`accentNames`); the value is a reference to that token, never a literal colour.
// - `--sc-item-accent`: a title's own colour from its poster (ItemRef.accent, image pipeline),
//   used for the poster card glow and the detail backdrop tint. CSS reads it as
//   `var(--sc-item-accent, <token>)`.

const HEX_COLOUR = /^#[0-9a-f]{6}$/i;

/** Inline style that hands a title's accent to the element and its children, if it is valid. */
export function itemAccentStyle(accent: string | undefined): CSSProperties | undefined {
  if (!accent || !HEX_COLOUR.test(accent)) return undefined;
  return { '--sc-item-accent': accent.toLowerCase() } as CSSProperties;
}
