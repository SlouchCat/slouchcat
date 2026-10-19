// Scrolls a newly focused element into view within the focus motion budget (motion.duration.focus,
// at most 200 ms). `scrollIntoView({ behavior: 'smooth' })` leaves the duration to the browser:
// Chromium took 263 ms for one card step, so the scroll is animated here instead.
//
// The target comes from the browser itself: an instant scrollIntoView (with scroll snapping) is
// measured on every scroll container on the way, the old positions are put back, and each
// container then glides from old to new in one animation frame loop.

/** cubic-bezier(x1, y1, x2, y2) as an easing function of t in [0, 1] (CSS timing function). */
export function cubicBezier(x1: number, y1: number, x2: number, y2: number) {
  const curve = (a: number, b: number, t: number) =>
    3 * a * t * (1 - t) ** 2 + 3 * b * t ** 2 * (1 - t) + t ** 3;
  const slope = (a: number, b: number, t: number) =>
    3 * a * (1 - t) ** 2 + 6 * (b - a) * t * (1 - t) + 3 * (1 - b) * t ** 2;
  return (x: number): number => {
    if (x <= 0) return 0;
    if (x >= 1) return 1;
    // Newton steps on x(t) = x, then bisection if the slope is too flat.
    let t = x;
    for (let step = 0; step < 8; step += 1) {
      const error = curve(x1, x2, t) - x;
      if (Math.abs(error) < 1e-5) return curve(y1, y2, t);
      const d = slope(x1, x2, t);
      if (Math.abs(d) < 1e-6) break;
      t -= error / d;
    }
    let low = 0;
    let high = 1;
    t = x;
    for (let step = 0; step < 30 && high - low > 1e-6; step += 1) {
      if (curve(x1, x2, t) < x) low = t;
      else high = t;
      t = (low + high) / 2;
    }
    return curve(y1, y2, t);
  };
}

// motion.easing.standard; read from the token at runtime would need parsing for no gain.
const standard = cubicBezier(0.2, 0, 0, 1);
const FALLBACK_MS = 180;

/** motion.duration.focus in milliseconds, from the token's custom property. */
function focusDuration(): number {
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue('--sc-motion-duration-focus')
    .trim();
  const ms = value.endsWith('ms') ? parseFloat(value) : parseFloat(value) * 1000;
  return Number.isFinite(ms) && ms > 0 ? Math.min(ms, 200) : FALLBACK_MS;
}

const reducedMotion = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** The element and every scrolling ancestor, plus the document's scroller. */
function scrollers(node: Element): Element[] {
  const found: Element[] = [];
  for (let element = node.parentElement; element; element = element.parentElement) {
    if (element.scrollWidth > element.clientWidth || element.scrollHeight > element.clientHeight) {
      found.push(element);
    }
  }
  const root = document.scrollingElement;
  if (root && !found.includes(root)) found.push(root);
  return found;
}

interface Glide {
  frame: number;
  /** Inline scroll-snap-type before the first glide took snapping off. */
  snap: string;
}

const gliding = new Map<Element, Glide>();

/** Stops a running glide of `element`; returns the inline snap type to restore later. */
function stop(element: Element): string | undefined {
  const glide = gliding.get(element);
  if (!glide) return undefined;
  cancelAnimationFrame(glide.frame);
  gliding.delete(element);
  return glide.snap;
}

/** Scrolls a focused element into view the way the rows expect it (centred horizontally). */
export function revealElement(node: Element): void {
  if (typeof node.scrollIntoView !== 'function') return;
  const options: ScrollIntoViewOptions = { block: 'nearest', inline: 'center' };
  const containers = scrollers(node);
  // A glide still on its way hands over its original snap type and its current position.
  const snaps = new Map(containers.map((element) => [element, stop(element)] as const));
  for (const [element, snap] of snaps) {
    if (snap !== undefined) (element as HTMLElement).style.scrollSnapType = snap;
  }
  if (reducedMotion()) {
    node.scrollIntoView({ ...options, behavior: 'instant' });
    return;
  }

  const from = containers.map((element) => [element.scrollLeft, element.scrollTop] as const);
  node.scrollIntoView({ ...options, behavior: 'instant' });
  const duration = focusDuration();
  containers.forEach((element, index) => {
    const [left, top] = from[index] ?? [element.scrollLeft, element.scrollTop];
    const toLeft = element.scrollLeft;
    const toTop = element.scrollTop;
    if (left === toLeft && top === toTop) return;
    const { style } = element as HTMLElement;
    const snap = style.scrollSnapType;
    // Snapping would pull every intermediate position to a snap point; the target is snapped.
    style.scrollSnapType = 'none';
    element.scrollLeft = left;
    element.scrollTop = top;
    const start = performance.now();
    const step = (now: number) => {
      const progress = standard(Math.min(1, (now - start) / duration));
      element.scrollLeft = left + (toLeft - left) * progress;
      element.scrollTop = top + (toTop - top) * progress;
      if (progress < 1) {
        gliding.set(element, { frame: requestAnimationFrame(step), snap });
      } else {
        gliding.delete(element);
        style.scrollSnapType = snap;
      }
    };
    gliding.set(element, { frame: requestAnimationFrame(step), snap });
  });
}
