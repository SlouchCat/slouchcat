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
import { AppLink, useAppNavigate } from './AppLink';
import { Button, SpatialButton, SpatialButtonLink } from './Button';
import styles from './HeroCarousel.module.css';
import { PosterCard } from './PosterCard';
import { TitleOrLogo } from './TitleOrLogo';
import { RowList, RowTitle } from './Row';
import { setSpatialFocus, SpatialGroup } from './spatial';

export interface HeroSlide {
  id: string;
  title: string;
  /** The title logo (w400, WebP or PNG with transparency, ADR 0018); shown in place of the text. */
  logoUrl?: string;
  /** Same-origin backdrop URL (w1280); the poster is used blurred when there is none. */
  backdropUrl?: string;
  /** The same backdrop in w780 for phones; the browser picks it through `srcset`. */
  backdropSmallUrl?: string;
  /** The same backdrop in w1920 for wide screens. */
  backdropLargeUrl?: string;
  /**
   * The same backdrop for 4K and high-density screens (w3840, only from a wide original) and its
   * real width, which the `srcset` names (an original between 2560 and 3839 px keeps its width).
   */
  backdropUhdUrl?: string;
  backdropUhdWidth?: number;
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
  /** Heading over the stage and accessible name of the carousel. */
  label: string;
  /**
   * The page of the whole list (concept 20 P-7): the heading over the stage becomes its link, like
   * the title of a row, and a stop of the arrow keys above the stage's buttons.
   */
  href?: string;
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
  /**
   * Asked before «Abspielen» plays a slide: return true when the page takes the press over (a
   * started film asks whether to resume, concept 20 P-4); the link then does not navigate.
   */
  onPlayPress?: (slide: HeroSlide) => boolean;
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
  /**
   * A control beside a poster of the row, over the top right of its cover (the start page's pen,
   * concept 15 P-6), not inside the poster's button; `active` for the active poster, the row's one
   * Tab stop, so the tools of the others leave the Tab order. It turns with its poster (CSS
   * `order` of the list item); while it has the focus the band holds like on the poster.
   */
  tool?: (slide: HeroSlide, active: boolean) => ReactNode;
  /**
   * A second control over the bottom right of a poster of the row (the watchlist button, concept
   * 17 D-9), after `tool` in the Tab order and, like it, out of the Tab order for every poster but
   * the active one. Not on the band above.
   */
  mark?: (slide: HeroSlide, active: boolean) => ReactNode;
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

/** How long the poster row slides on a change (concept 15, 6.3). */
const GLIDE_MS = 450;

/** The sideways shift of the row's track in px (its transform, which a running glide sets). */
function trackShift(track: HTMLElement): number {
  const transform = getComputedStyle(track).transform;
  return transform && transform !== 'none' ? new DOMMatrixReadOnly(transform).m41 : 0;
}

// Rendered width of the stage: the viewport less the side navigation's column from 768 px (concept
// 08, 8). The server's hero preload and prepaint copy it (heroSizes in server/internal/api/
// preload.go); change them together.
const HERO_SIZES = '(width >= 768px) calc(100vw - 15rem), 100vw';

/**
 * Source attributes of a slide image: the small backdrop first, the larger ones for wide and dense
 * screens (the browser picks by `sizes` and the device pixel ratio).
 */
function imageSource(slide: HeroSlide) {
  const {
    backdropUrl,
    backdropSmallUrl,
    backdropLargeUrl,
    backdropUhdUrl,
    backdropUhdWidth,
    posterUrl,
  } = slide;
  if (backdropUrl && backdropSmallUrl) {
    const candidates = [`${backdropSmallUrl} 780w`, `${backdropUrl} 1280w`];
    if (backdropLargeUrl) candidates.push(`${backdropLargeUrl} 1920w`);
    if (backdropUhdUrl && backdropUhdWidth)
      candidates.push(`${backdropUhdUrl} ${backdropUhdWidth}w`);
    return { src: backdropSmallUrl, srcSet: candidates.join(', '), sizes: HERO_SIZES };
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
 * focus (`:focus-visible`; a click with the mouse focuses without it and does not hold, F-3), while
 * the focus is anywhere in the poster row, visible or not, so a screen reader reads the row in
 * peace (concept 15, 6.3; a poster pressed with the pointer takes no focus), and while the browser
 * tab is hidden. Never under the pointer. With reduced motion it never moves on
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
 * The poster row is an endless band (concept 15, 6.3): the active slide's poster always stands
 * first, with the marker, followed by the next ones; the ones already shown move to the end. The
 * DOM keeps the server's order, only CSS `order` turns the row, and each poster keeps its fixed
 * position for assistive technology («3 von 12»); the spatial navigation follows what is seen. On
 * a change forward (the interval, arrow right on the band, a poster) the row slides left until the
 * chosen poster stands first, then turns; on a change back the last poster slides in from the
 * left. With reduced motion it turns at once. A swipe scrolls the row freely; the next change
 * brings it back to its start.
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
  href,
  slides,
  playLabel,
  detailsLabel,
  trailerLabel,
  rowLabel,
  pauseLabel,
  resumeLabel,
  onTrailerPress,
  onPlayPress,
  paused: held = false,
  start = 0,
  intervalSeconds = 5,
  onArtShown,
  tool,
  mark,
}: HeroCarouselProps) {
  const { t } = useTranslation();
  const navigate = useAppNavigate();
  const count = slides.length;
  // The server's draw, taken once (concept 08, 5.2).
  const [first] = useState(() => Math.min(Math.max(start, 0), Math.max(count - 1, 0)));
  const [index, setIndex] = useState(first);
  const [userPaused, setUserPaused] = useState(false);
  const [focusHeld, setFocusHeld] = useState(false);
  const [hiddenTab, setHiddenTab] = useState(() => document.visibilityState === 'hidden');
  const [reducedMotion] = useState(prefersReducedMotion);
  const paused = held || userPaused || focusHeld || hiddenTab || reducedMotion || count < 2;

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

  // The slide whose poster leads the row (CSS order 0); it follows `current` once the row has slid.
  const [lead, setLead] = useState(first);
  const leader = Math.min(lead, count - 1);
  // A step back turns the row at once and lets the poster that came first slide in from the left.
  const backward = useRef(false);
  const step = (delta: number) => {
    backward.current = delta < 0;
    setIndex((value) => (value + delta + count) % count);
  };
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

  // Keyboard focus holds the band, a pointer's focus does not (F-3), except in the poster row, where
  // any focus holds (concept 15, 6.3): decided when the focus arrives, and again on a key press,
  // which makes the focus a keyboard one.
  const section = useRef<HTMLElement>(null);
  // The poster row's scroll box; its first child is the track that turns (below).
  const scroller = useRef<HTMLDivElement>(null);
  const shown = Boolean(slides[current]);
  useEffect(() => {
    const node = section.current;
    if (!node) return;
    const onFocusIn = (event: FocusEvent) => {
      const target = event.target as Element;
      setFocusHeld(focusVisible(target) || (scroller.current?.contains(target) ?? false));
    };
    const onFocusOut = (event: FocusEvent) => {
      if (!node.contains(event.relatedTarget as Node | null)) setFocusHeld(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (MODIFIERS.has(event.key)) return;
      pressed.current = undefined;
      setFocusHeld(true);
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

  // The endless band (concept 15, 6.3). Forward: the track slides left by as many cards as the
  // chosen poster stood from the front, from wherever a swipe or a running glide left it; its last
  // frame holds until the row has turned. Back: the row turns at once and the track slides in from
  // one card (plus the swipe) to the left. Either way the row ends at its start (scrollLeft 0).
  const glide = useRef<Animation | undefined>(undefined);
  const entering = useRef(0);
  useLayoutEffect(() => {
    // Taken on every change: a step back that only undid a running glide must not leave the flag
    // for the next change forward (a click on a poster or the timer, which set no direction).
    const back = backward.current;
    backward.current = false;
    const node = scroller.current;
    const track = node?.firstElementChild;
    const card = track?.firstElementChild;
    const animated =
      !reducedMotion &&
      track instanceof HTMLElement &&
      card instanceof HTMLElement &&
      typeof track.animate === 'function';
    const easing = animated
      ? getComputedStyle(track).getPropertyValue('--sc-motion-easing-standard').trim() || 'ease'
      : 'ease';
    if (current === leader) {
      // The row has turned: the finished glide goes in the same frame.
      glide.current?.cancel();
      glide.current = undefined;
      if (animated && entering.current) {
        track.animate(
          [{ transform: `translateX(${-entering.current}px)` }, { transform: 'none' }],
          { duration: GLIDE_MS, easing },
        );
      }
      entering.current = 0;
      return;
    }
    if (!node || !animated) {
      if (node) node.scrollLeft = 0;
      setLead(current);
      return;
    }
    const from = node.scrollLeft - trackShift(track);
    glide.current?.cancel();
    glide.current = undefined;
    node.scrollLeft = 0;
    const stride = card.offsetWidth + (parseFloat(getComputedStyle(track).columnGap) || 0);
    if (back) {
      entering.current = stride + from;
      setLead(current);
      return;
    }
    const shift = ((current - leader + count) % count) * stride;
    const animation = track.animate(
      [{ transform: `translateX(${-from}px)` }, { transform: `translateX(${-shift}px)` }],
      { duration: GLIDE_MS, easing, fill: 'forwards' },
    );
    animation.onfinish = () => setLead(current);
    glide.current = animation;
  }, [current, leader, count, reducedMotion]);
  // A glide still running when the band goes must not turn the row afterwards.
  useEffect(
    () => () => {
      const animation = glide.current;
      if (!animation) return;
      animation.onfinish = null;
      animation.cancel();
      glide.current = undefined;
    },
    [],
  );

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
    const plain =
      event.button === 0 && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey;
    // The slide the press meant: the one under the pointer when it went down, else the current.
    const meant = target && event.detail !== 0 ? target : slide;
    if (link === 'playHref' && plain && meant && onPlayPress?.(meant)) {
      event.preventDefault();
      return;
    }
    if (!target || target.id === slide?.id || event.detail === 0 || !plain) return;
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
  // The row's visual place of a slide's poster: the leading one first, the rest after it in turn.
  const order = (position: number) => (position - leader + count) % count;
  // The row's last poster (as seen) holds the focus inside the band; left of the first one is the
  // band's way out to the side navigation (concept 08, 7.1), or nothing where there is none.
  const holdEnds = (position: number) => (direction: string) =>
    !(direction === 'right' && order(position) === count - 1);

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
      <div className={styles.header}>
        <RowTitle title={label} href={href} focusKey="hero-title" />
      </div>
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
          <div
            key={slide.id}
            className={styles.slide}
            role="group"
            aria-roledescription={t('ui.carousel.slide')}
            aria-label={t('ui.carousel.position', { index: current + 1, count })}
            aria-live={paused ? 'polite' : 'off'}
          >
            {/* The film title leads to its detail page (concept 20 P-7) for the pointer; the keyboard
                has «Details» right below, so the link is no extra stop. */}
            <h3 className={styles.title}>
              <AppLink
                href={slide.detailsHref}
                className={styles.titleLink}
                tabIndex={-1}
                onPointerDown={press}
                onClick={(event) => follow(event, 'detailsHref')}
              >
                <TitleOrLogo title={slide.title} logoUrl={slide.logoUrl} className={styles.logo} />
              </AppLink>
            </h3>
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
          order={order}
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
                tool={tool?.(item, active)}
                mark={mark?.(item, active)}
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
