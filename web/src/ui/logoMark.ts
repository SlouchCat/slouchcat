/**
 * The Slouchcat sign as two paths on a canvas of 512 x 512: the head with its two ears, and the
 * play triangle that is cut out of it. The head's bounding box (x 136 to 376, y 112 to 400) sits
 * in the middle of the canvas, so the sign is centred wherever the canvas is. The triangle is
 * centred on its centroid (x 256), as play signs are, and on the head without the ears.
 *
 * public/icon.svg carries the same two paths (app icons cannot import a module), and so do the
 * copies outside the web app; logoMark.test.ts compares them.
 */
export const LOGO_VIEW_BOX = '0 0 512 512';
export const LOGO_HEAD = 'M136 376V112l80 72h80l80-72v264a24 24 0 0 1-24 24H160a24 24 0 0 1-24-24z';
export const LOGO_PLAY = 'M228 244v96l84-48z';
