import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';
import { useAppNavigate } from './AppLink';
import { Button, SpatialButton, SpatialButtonLink } from './Button';
import styles from './HeroCarousel.module.css';
import { PosterCard } from './PosterCard';
import { RowList } from './Row';
import { setSpatialFocus, SpatialGroup } from './spatial';

export interface HeroSlide {
  id: string;
  title: string;
  /** Same-origin backdrop URL (w1280); the poster is used blurred when there is none. */
  backdropUrl?: string;
  /** The same backdrop in w780 for phones; the browser picks it through `srcset`. */
  backdropSmallUrl?: string;
  posterUrl?: string;
  /** The same poster in w200, offered to the poster row under the band through `srcset`. */
  posterSmallUrl?: string;
  /** The title's accent colour (`#rrggbb`, ItemRef.accent) for its card in the poster row. */
  accent?: string;
  /** Line under the card's title in the poster row (year or author, like the start page's rows). */
  caption?: string;
  /** Metadata line (chips for year, runtime, FSK). */
  meta?: ReactNode;
  /** Personal note of the main user on this pick (E-2), shown under the title. */
  note?: string;
  /** The title has a trailer (only `true` shows the trailer button; absent means unknown). */
  hasTrailer?: boolean;
  playHref: string;
  detailsHref: string;
}

interface HeroCarouselProps {
  /** Visible eyebrow and accessible name of the carousel. */
  label: string;
  slides: readonly HeroSlide[];
  playLabel: string;
  detailsLabel: string;
  trailerLabel: string;
  /** Accessible name of the poster row under the band. */
  rowLabel: string;
  /** Name of the pause button (its state is `aria-pressed`). */
  pauseLabel: string;
  /** Tooltip of the pause button while the band is paused by it. */
  resumeLabel: string;
  /** Opens the trailer of a slide; the button stands only on slides with `hasTrailer: true`. */
  onTrailerPress?: (slide: HeroSlide) => void;
  /** Holds the carousel on its slide, for example while a dialog opened from it is on screen. */
  paused?: boolean;
  /**
   * Index of the slide the band begins with (HomePage.heroStart, drawn by the server, concept 08,
   * F-2). Read once when the carousel mounts: later values (a refetch) never move the band.
   */
  start?: number;
  /** Seconds one slide stays (HomePage.heroIntervalSeconds, F-10). */
  intervalSeconds?: number;
  /** Called once the first image is on screen (loaded and painted) or has failed. */
  onArtShown?: () => void;
}

export const HERO_PLAY_KEY = 'hero-play';
const HERO_DETAILS_KEY = 'hero-details';
const HERO_TRAILER_KEY = 'hero-trailer';
const stripKey = (slide: HeroSlide) => `hero-strip-${slide.id}`;

const prefersReducedMotion = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** Whether the element shows a keyboard focus (`:focus-visible`); false where unsupported. */
function focusVisible(element: Element | null): boolean {
  try {
    return element?.matches(':focus-visible') ?? false;
  } catch {
    return false;
  }
}

const MODIFIERS = new Set(['Shift', 'Control', 'Alt', 'Meta']);

// Rendered width of the stage: the viewport less the side navigation's column from 768 px (concept
// 08, 8). The server's hero preload and prepaint copy it (heroSizes in server/internal/api/
// preload.go); change them together.
const HERO_SIZES = '(width >= 768px) calc(100vw - 15rem), 100vw';

/** Source attributes of a slide image: the small backdrop first, the large one for wide screens. */
function imageSource(slide: HeroSlide) {
  const { backdropUrl, backdropSmallUrl, posterUrl } = slide;
  if (backdropUrl && backdropSmallUrl) {
    return {
      src: backdropSmallUrl,
      srcSet: `${backdropSmallUrl} 780w, ${backdropUrl} 1280w`,
      sizes: HERO_SIZES,
    };
  }
  return { src: backdropUrl ?? backdropSmallUrl ?? posterUrl };
}

/**
 * Fades out the first hero image the server wrote into the start page's shell (`#hero-prepaint`,
 * api/preload.go) and removes it afterwards. Safe to call more than once or without the image.
 */
export function dismissHeroPrepaint() {
  const prepaint = document.getElementById('hero-prepaint');
  if (!prepaint || prepaint.dataset.done !== undefined) return;
  prepaint.addEventListener('transitionend', () => prepaint.remove(), { once: true });
  prepaint.dataset.done = '';
}

/**
 * The stage of the start page: a framed band with the active slide's backdrop, and under it a row
 * of the slides' posters in the normal card size to choose from (concept 08, 7.2). It begins with
 * the slide `start` and moves on every `intervalSeconds`, with a time bar in the active poster.
 *
 * It holds (`data-paused` on the section) while a trailer is open (`paused`), with one slide, while
 * the viewer paused it with the pause button (F-9), while an element inside it has the keyboard
 * focus (`:focus-visible`; a click with the mouse focuses without it and does not hold, F-3) and
 * while the browser tab is hidden. Never under the pointer. With reduced motion it never moves on
 * by itself and shows no time bar; everything else works.
 *
 * Keyboard: arrow left on the first button and arrow right on the last button page through the
 * slides; the buttons keep focus. The buttons are play, details and, on a slide with a trailer,
 * trailer, so the last one is details or trailer depending on the slide. Tab goes play, details,
 * trailer, the pause button (top right, next to the position), then the active poster. In the
 * poster row the arrow keys move the focus only, Enter (or a click) shows the focused poster's
 * slide without leaving the page (F-5); the poster keeps focus. Entering the row from above or by
 * Tab lands on the active poster (roving tabindex); its last poster holds the focus inside the
 * band, left of the first one is the way out to the side navigation.
 *
 * The poster row follows the band: on every change it scrolls sideways (never the page) until the
 * active poster stands at its left edge (N-4); a space after the last poster lets it get there.
 *
 * The three buttons keep the slide they were pressed on: a change between pointer down and up
 * opens the pressed title, not the one the band moved to (12.1).
 *
 * Artwork loads on demand (P-10): only the active slide's image is in the document at first, eager
 * and with high priority. Once it is on screen, the next two slides' images join (lazy, low
 * priority) so the crossfade has them ready; every slide that was shown stays. The stacked images
 * all sit in the viewport, so `loading="lazy"` alone would not hold the others back. The posters
 * of the row join once the band's first image is on screen.
 */
export function HeroCarousel({
  label,
  slides,
  playLabel,
  detailsLabel,
  trailerLabel,
  rowLabel,
  pauseLabel,
  resumeLabel,
  onTrailerPress,
  paused: held = false,
  start = 0,
  intervalSeconds = 5,
  onArtShown,
}: HeroCarouselProps) {
  const { t } = useTranslation();
  const navigate = useAppNavigate();
  const count = slides.length;
  // The server's draw, taken once (concept 08, 5.2).
  const [first] = useState(() => Math.min(Math.max(start, 0), Math.max(count - 1, 0)));
  const [index, setIndex] = useState(first);
  const [userPaused, setUserPaused] = useState(false);
  const [keyboardFocus, setKeyboardFocus] = useState(false);
  const [hiddenTab, setHiddenTab] = useState(() => document.visibilityState === 'hidden');
  const [reducedMotion] = useState(prefersReducedMotion);
  const paused = held || userPaused || keyboardFocus || hiddenTab || reducedMotion || count < 2;

  const current = Math.min(index, count - 1);
  const slide = slides[current];
  // Slide positions whose image is in the document.
  const [armed, setArmed] = useState<ReadonlySet<number>>(() => new Set([current]));
  const [activeLoaded, setActiveLoaded] = useState(false);
  // The row's posters wait for the band's first image (P-10), or come at once when it has none.
  const [artFailed, setArtFailed] = useState(false);
  const firstArt = slides[first] && imageSource(slides[first]).src;
  const stripImages = activeLoaded || artFailed || !firstArt;
  const next = (current + 1) % count;
  const afterNext = (current + 2) % count;
  if (!armed.has(current) || (activeLoaded && !(armed.has(next) && armed.has(afterNext)))) {
    const add = new Set(armed).add(current);
    if (activeLoaded) add.add(next).add(afterNext);
    setArmed(add);
  }

  const step = (delta: number) => setIndex((value) => (value + delta + count) % count);
  // norigin keeps the handlers it saw at registration; route them through a ref.
  const stepRef = useRef(step);
  useLayoutEffect(() => {
    stepRef.current = step;
  });

  useEffect(() => {
    if (paused) return;
    const timer = window.setTimeout(
      () => setIndex((value) => (value + 1) % count),
      intervalSeconds * 1000,
    );
    return () => window.clearTimeout(timer);
  }, [paused, current, count, intervalSeconds]);

  useEffect(() => {
    const update = () => setHiddenTab(document.visibilityState === 'hidden');
    document.addEventListener('visibilitychange', update);
    return () => document.removeEventListener('visibilitychange', update);
  }, []);

  // The slide a band button was pressed on (12.1); a keyboard click (detail 0) takes the current.
  const pressed = useRef<HeroSlide | undefined>(undefined);

  // Keyboard focus holds the band, a pointer's focus does not (F-3): decided when the focus
  // arrives, and again on a key press, which makes the focus a keyboard one.
  const section = useRef<HTMLElement>(null);
  const shown = Boolean(slides[current]);
  useEffect(() => {
    const node = section.current;
    if (!node) return;
    const onFocusIn = (event: FocusEvent) =>
      setKeyboardFocus(focusVisible(event.target as Element));
    const onFocusOut = (event: FocusEvent) => {
      if (!node.contains(event.relatedTarget as Node | null)) setKeyboardFocus(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (MODIFIERS.has(event.key)) return;
      pressed.current = undefined;
      setKeyboardFocus(true);
    };
    node.addEventListener('focusin', onFocusIn);
    node.addEventListener('focusout', onFocusOut);
    node.addEventListener('keydown', onKeyDown);
    return () => {
      node.removeEventListener('focusin', onFocusIn);
      node.removeEventListener('focusout', onFocusOut);
      node.removeEventListener('keydown', onKeyDown);
    };
  }, [shown]);

  // The poster row puts the active poster at its left edge: sideways only, so the page never moves
  // (scrollIntoView would scroll it too). At once on the first slide, gliding afterwards.
  const scroller = useRef<HTMLDivElement>(null);
  const placed = useRef(false);
  useEffect(() => {
    const node = scroller.current;
    const card = node?.querySelectorAll('li')[current];
    const list = node?.firstElementChild;
    if (!node || !card || !list) return;
    const gutter = parseFloat(getComputedStyle(list).paddingLeft) || 0;
    const left = Math.max(
      0,
      card.getBoundingClientRect().left -
        node.getBoundingClientRect().left +
        node.scrollLeft -
        gutter,
    );
    if (Math.abs(left - node.scrollLeft) > 1) {
      node.scrollTo({ left, behavior: placed.current && !reducedMotion ? 'smooth' : 'auto' });
    }
    placed.current = true;
  }, [current, reducedMotion]);

  const press = () => {
    pressed.current = slide;
  };
  const pressedSlide = () => {
    const target = pressed.current;
    pressed.current = undefined;
    return target;
  };
  const follow = (event: MouseEvent<HTMLAnchorElement>, link: 'playHref' | 'detailsHref') => {
    const target = pressedSlide();
    if (
      !target ||
      target.id === slide?.id ||
      event.detail === 0 ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }
    event.preventDefault();
    navigate(target[link]);
  };

  // Left of the first and right of the last button is the carousel's own: it pages (with one
  // slide it does nothing), so the focus never jumps sideways out of the hero into the header.
  const pageBack = (direction: string) => {
    if (direction !== 'left') return true;
    if (count > 1) stepRef.current(-1);
    return false;
  };
  const pageForward = (direction: string) => {
    if (direction !== 'right') return true;
    if (count > 1) stepRef.current(1);
    return false;
  };
  const trailer = slide?.hasTrailer === true && onTrailerPress !== undefined;
  // Paging away from the trailer button to a slide without a trailer takes the button away: the
  // focus moves to details first, which is the last button then.
  const pageFromTrailer = (direction: string) => {
    if (direction === 'right' && count > 1 && slides[next]?.hasTrailer !== true) {
      setSpatialFocus(HERO_DETAILS_KEY);
    }
    return pageForward(direction);
  };
  // The row's last poster holds the focus inside the band; left of the first one is the band's
  // way out to the side navigation (concept 08, 7.1), or nothing where there is none.
  const holdEnds = (position: number) => (direction: string) =>
    !(direction === 'right' && position === count - 1);

  if (!slide) return null;

  return (
    <section
      ref={section}
      className={styles.hero}
      aria-roledescription={t('ui.carousel.roledescription')}
      aria-label={label}
      data-paused={paused || undefined}
      style={{ '--hero-interval': `${intervalSeconds}s` } as CSSProperties}
    >
      <div className={styles.stage}>
        <div className={styles.art} aria-hidden="true">
          {slides.map((item, position) => {
            const source = imageSource(item);
            if (!source.src || !armed.has(position)) return null;
            const active = position === current;
            return (
              <img
                key={item.id}
                className={`${styles.image} ${item.backdropUrl || item.backdropSmallUrl ? '' : styles.blurred}`}
                data-active={active || undefined}
                {...source}
                alt=""
                width={1280}
                height={720}
                loading={active ? 'eager' : 'lazy'}
                fetchPriority={active ? 'high' : 'low'}
                decoding={active ? 'sync' : 'async'}
                onLoad={
                  active
                    ? () => {
                        // Two frames: the one that paints the image, then the next. The next slide
                        // waits for them too: the server preloads the first image, so it may be
                        // loaded before it is painted.
                        requestAnimationFrame(() =>
                          requestAnimationFrame(() => {
                            setActiveLoaded(true);
                            onArtShown?.();
                          }),
                        );
                      }
                    : undefined
                }
                onError={
                  active
                    ? () => {
                        setArtFailed(true);
                        onArtShown?.();
                      }
                    : undefined
                }
              />
            );
          })}
          <div className={styles.scrim} />
        </div>

        <div className={styles.content}>
          <p className={styles.eyebrow}>
            <span className={styles.mark} aria-hidden="true" />
            {label}
          </p>
          <div
            key={slide.id}
            className={styles.slide}
            role="group"
            aria-roledescription={t('ui.carousel.slide')}
            aria-label={t('ui.carousel.position', { index: current + 1, count })}
            aria-live={paused ? 'polite' : 'off'}
          >
            <h2 className={styles.title}>{slide.title}</h2>
            {slide.note && <p className={styles.note}>{slide.note}</p>}
            {slide.meta && <div className={styles.meta}>{slide.meta}</div>}
          </div>
          <SpatialGroup focusKey="hero" className={styles.actions}>
            <SpatialButtonLink
              focusKey={HERO_PLAY_KEY}
              onArrowPress={pageBack}
              href={slide.playHref}
              onPointerDown={press}
              onClick={(event) => follow(event, 'playHref')}
              variant="primary"
              size="lg"
              icon="play"
            >
              {playLabel}
            </SpatialButtonLink>
            <SpatialButtonLink
              focusKey={HERO_DETAILS_KEY}
              onArrowPress={trailer ? undefined : pageForward}
              href={slide.detailsHref}
              onPointerDown={press}
              onClick={(event) => follow(event, 'detailsHref')}
              variant="secondary"
              size="lg"
              icon="info"
            >
              {detailsLabel}
            </SpatialButtonLink>
            {trailer && (
              <SpatialButton
                focusKey={HERO_TRAILER_KEY}
                onArrowPress={pageFromTrailer}
                variant="secondary"
                size="lg"
                icon="play"
                onPointerDown={press}
                onPress={() => {
                  const target = pressedSlide();
                  onTrailerPress?.(target?.hasTrailer === true ? target : slide);
                }}
              >
                {trailerLabel}
              </SpatialButton>
            )}
          </SpatialGroup>
        </div>

        {count > 1 && (
          <>
            {/* Position and the pause button (F-9): Tab reaches it after the band's buttons. */}
            <div className={styles.corner}>
              <span className={styles.position} aria-hidden="true">
                {t('ui.carousel.position', { index: current + 1, count })}
              </span>
              <Button
                icon={userPaused ? 'play' : 'pause'}
                iconOnly
                aria-label={pauseLabel}
                aria-pressed={userPaused}
                title={userPaused ? resumeLabel : pauseLabel}
                onPress={() => setUserPaused((value) => !value)}
              />
            </div>
            {/* Pointer only: the keyboard pages with the buttons and the poster row. */}
            <span className={`${styles.arrow} ${styles.previous}`}>
              <Button
                icon="chevronLeft"
                iconOnly
                excludeFromTabOrder
                aria-label={t('ui.row.previous')}
                onPress={() => step(-1)}
              />
            </span>
            <span className={`${styles.arrow} ${styles.next}`}>
              <Button
                icon="chevronRight"
                iconOnly
                excludeFromTabOrder
                aria-label={t('ui.row.next')}
                onPress={() => step(1)}
              />
            </span>
          </>
        )}
      </div>

      {count > 1 && (
        <RowList
          scrollerRef={scroller}
          focusKey="hero-strip"
          aria-label={rowLabel}
          rememberLastChild={false}
          preferredChildFocusKey={stripKey(slide)}
          trailingSpace
        >
          {slides.map((item, position) => {
            const active = position === current;
            return (
              <PosterCard
                key={item.id}
                focusKey={stripKey(item)}
                title={item.title}
                caption={item.caption}
                posterUrl={item.posterUrl}
                posterSmallUrl={item.posterSmallUrl}
                accent={item.accent}
                deferImage={!stripImages}
                onPress={() => setIndex(position)}
                active={active}
                dimmed={!active}
                aria-current={active ? 'true' : undefined}
                tabIndex={active ? 0 : -1}
                onArrowPress={holdEnds(position)}
              >
                {/* Time to the next slide; gone while the band holds, anew when it moves on. */}
                {active && !paused && <span className={styles.timer} />}
              </PosterCard>
            );
          })}
        </RowList>
      )}
    </section>
  );
}
