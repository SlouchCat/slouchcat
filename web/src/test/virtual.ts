import { vi } from 'vitest';

// jsdom has no layout: the virtual library grid (LibraryGrid) reads its columns and gaps from the
// computed style of its list, the text height and the header from its probe, its top from the
// list's rectangle and the window's size and scroll position. This lays out a fixed grid for it
// and makes the window scrollable (concept 17 plan m-9).

export interface VirtualLayout {
  columns?: number;
  /** Width of one column in px. */
  column?: number;
  rowGap?: number;
  /** Height of a card's text under the cover (the probe). */
  text?: number;
  header?: number;
  /** Top of the grid's list in the document. */
  top?: number;
  viewport?: { width: number; height: number };
}

/** Lays out the grid and returns the scroll position; undone by vi.restoreAllMocks. */
export function virtualLayout({
  columns = 6,
  column = 100,
  rowGap = 20,
  text = 40,
  header = 64,
  top = 300,
  viewport = { width: 1280, height: 800 },
}: VirtualLayout = {}) {
  let scrollY = 0;
  const observers = new Set<() => void>();
  vi.stubGlobal(
    'ResizeObserver',
    class {
      private readonly run: () => void;
      constructor(callback: ResizeObserverCallback) {
        this.run = () => callback([], this);
      }
      observe() {
        observers.add(this.run);
        queueMicrotask(this.run);
      }
      unobserve() {}
      disconnect() {
        observers.delete(this.run);
      }
    },
  );
  vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(viewport.width);
  vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(viewport.height);
  vi.spyOn(window, 'scrollY', 'get').mockImplementation(() => scrollY);
  vi.spyOn(window, 'pageYOffset', 'get').mockImplementation(() => scrollY);
  const scroll = (y: number) => {
    scrollY = Math.max(0, y);
    window.dispatchEvent(new Event('scroll'));
  };
  vi.spyOn(window, 'scrollTo').mockImplementation((options?: ScrollToOptions | number) => {
    if (typeof options === 'object' && options.top !== undefined) scroll(options.top);
  });

  // The document is as tall as any list: the virtualizer clamps its scroll to it.
  vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockImplementation(function (this: Element) {
    return this === document.documentElement ? 10_000_000 : 0;
  });
  const computed = window.getComputedStyle.bind(window);
  vi.spyOn(window, 'getComputedStyle').mockImplementation((element, pseudo) => {
    const style = computed(element, pseudo);
    if (element.tagName !== 'UL') return style;
    return new Proxy(style, {
      get(target, property) {
        if (property === 'gridTemplateColumns') return Array(columns).fill(`${column}px`).join(' ');
        if (property === 'rowGap') return `${rowGap}px`;
        const value = Reflect.get(target, property) as unknown;
        return typeof value === 'function' ? (value as () => unknown).bind(target) : value;
      },
    });
  });
  const isProbe = (element: Element) =>
    element.tagName === 'SPAN' && element.nextElementSibling?.tagName === 'UL';
  const rect = (x: number, y: number, width: number, height: number) =>
    ({ x, y, top: y, left: x, width, height, right: x + width, bottom: y + height }) as DOMRect;
  const original = Object.getOwnPropertyDescriptor(Element.prototype, 'getBoundingClientRect')!
    .value as (this: Element) => DOMRect;
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
    this: HTMLElement,
  ) {
    if (isProbe(this)) return rect(0, 0, header, text);
    if (this.tagName === 'UL') return rect(0, top - scrollY, columns * column, 0);
    return original.call(this);
  });

  return {
    /** Scrolls the window to `y` as the reader would (wheel, scroll bar). */
    scroll,
    get scrollY() {
      return scrollY;
    },
    /** The height of one row of cards plus its gap. */
    rowSize: Math.ceil(column * 1.5 + text) + rowGap,
    top,
    viewport,
    /** Lets the observers measure again (a new width, a panel above the grid). */
    resize: () => observers.forEach((run) => run()),
  };
}
