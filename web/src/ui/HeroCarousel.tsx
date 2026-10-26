import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';
import { Button, SpatialButtonLink } from './Button';
import styles from './HeroCarousel.module.css';
import { PosterFallback } from './PosterFallback';
import { SpatialGroup, useSpatialItem } from './spatial';

export interface HeroSlide {
  id: string;
  title: string;
  /** Same-origin backdrop URL (w1280); the poster is used blurred when there is none. */
  backdropUrl?: string;
  /** The same backdrop in w780 for phones; the browser picks it through `srcset`. */
  backdropSmallUrl?: string;
  posterUrl?: string;
  /** The same poster in w200, shown in the poster strip under the band. */
  posterSmallUrl?: string;
  /** Metadata line (chips for year, runtime, FSK). */
  meta?: ReactNode;
  /** Personal note of the main user on this pick (E-2), shown under the title. */
  note?: string;
  playHref: string;
  detailsHref: string;
}

interface HeroCarouselProps {
  /** Visible eyebrow and accessible name of the carousel. */
  label: string;
  slides: readonly HeroSlide[];
  playLabel: string;
  detailsLabel: string;
  intervalMs?: number;
  /** Called once the first image is on screen (loaded and painted) or has failed. */
  onArtShown?: () => void;
}

export const HERO_PLAY_KEY = 'hero-play';
const HERO_DETAILS_KEY = 'hero-details';
const stripKey = (slide: HeroSlide) => `hero-strip-${slide.id}`;

const prefersReducedMotion = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/** Source attributes of a slide image: the small backdrop first, the large one for wide screens. */
function imageSource(slide: HeroSlide) {
  const { backdropUrl, backdropSmallUrl, posterUrl } = slide;
  if (backdropUrl && backdropSmallUrl) {
    return {
      src: backdropSmallUrl,
      srcSet: `${backdropSmallUrl} 780w, ${backdropUrl} 1280w`,
      sizes: '100vw',
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

interface StripPosterProps {
  slide: HeroSlide;
  active: boolean;
  /** The image joins once the band's own artwork is on screen (P-10); the frame is there at once. */
  showImage: boolean;
  paused: boolean;
  onSelect: () => void;
  onArrowPress: (direction: string) => boolean;
}

/**
 * One poster of the strip: a button named after its title that shows the slide. Only the active
 * poster is in the Tab order (roving tabindex); arrow keys reach the others through the spatial
 * navigation.
 */
function StripPoster({
  slide,
  active,
  showImage,
  paused,
  onSelect,
  onArrowPress,
}: StripPosterProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({
    focusKey: stripKey(slide),
    onArrowPress,
  });
  const source = slide.posterSmallUrl ?? slide.posterUrl;
  return (
    <button
      ref={ref}
      type="button"
      className={styles.poster}
      tabIndex={active ? 0 : -1}
      data-active={active || undefined}
      data-paused={paused || undefined}
      aria-current={active ? 'true' : undefined}
      aria-label={slide.title}
      onFocus={onFocus}
      onClick={onSelect}
    >
      {source ? (
        showImage && (
          <img
            className={styles.posterImage}
            src={source}
            alt=""
            width={200}
            height={300}
            loading="lazy"
            decoding="async"
            draggable={false}
          />
        )
      ) : (
        <PosterFallback />
      )}
      {active && <span className={styles.timer} />}
    </button>
  );
}

/**
 * The stage of the start page: a framed band with the active slide's backdrop, and under it a strip
 * of the slides' posters to choose from. Rotates every `intervalMs` unless the pointer or focus is
 * inside it (or reduced motion is requested).
 *
 * Keyboard: arrow left on the first button and arrow right on the last button page through the
 * slides; the buttons keep focus. In the poster strip the arrow keys move the focus only, Enter (or
 * a click) shows the focused poster's slide; the poster keeps focus. Entering the strip from above
 * or by Tab lands on the active poster; the strip's ends hold the focus inside the band.
 *
 * Artwork loads on demand (P-10): only the active slide's image is in the document at first, eager
 * and with high priority. Once it is on screen, the next slide's image joins (lazy, low priority) so
 * the crossfade has it ready; every slide that was shown stays. The stacked images all sit in the
 * viewport, so `loading="lazy"` alone would not hold the others back.
 */
export function HeroCarousel({
  label,
  slides,
  playLabel,
  detailsLabel,
  intervalMs = 8000,
  onArtShown,
}: HeroCarouselProps) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const [hovered, setHovered] = useState(false);
  const [focusWithin, setFocusWithin] = useState(false);
  const [reducedMotion] = useState(prefersReducedMotion);
  const count = slides.length;
  const paused = hovered || focusWithin || reducedMotion || count < 2;

  const current = Math.min(index, count - 1);
  const slide = slides[current];
  // Slide positions whose image is in the document.
  const [armed, setArmed] = useState<ReadonlySet<number>>(() => new Set([current]));
  const [activeLoaded, setActiveLoaded] = useState(false);
  // The strip's posters wait for the band's first image (P-10), or come at once when it has none.
  const [artFailed, setArtFailed] = useState(false);
  const firstArt = slides[0] && imageSource(slides[0]).src;
  const stripImages = activeLoaded || artFailed || !firstArt;
  const next = (current + 1) % count;
  if (!armed.has(current) || (activeLoaded && !armed.has(next))) {
    setArmed(new Set(armed).add(current).add(activeLoaded ? next : current));
  }

  const step = (delta: number) => setIndex((value) => (value + delta + count) % count);
  // norigin keeps the handlers it saw at registration; route them through a ref.
  const stepRef = useRef(step);
  useLayoutEffect(() => {
    stepRef.current = step;
  });

  useEffect(() => {
    if (paused) return;
    const timer = window.setTimeout(() => setIndex((value) => (value + 1) % count), intervalMs);
    return () => window.clearTimeout(timer);
  }, [paused, current, count, intervalMs]);

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
  // The ends of the poster strip hold the focus inside the band.
  const holdEnds = (position: number) => (direction: string) =>
    !(direction === 'left' && position === 0) && !(direction === 'right' && position === count - 1);

  if (!slide) return null;

  return (
    <section
      className={styles.hero}
      aria-roledescription={t('ui.carousel.roledescription')}
      aria-label={label}
      onPointerEnter={() => setHovered(true)}
      onPointerLeave={() => setHovered(false)}
      onFocus={() => setFocusWithin(true)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setFocusWithin(false);
      }}
      style={{ '--hero-interval': `${intervalMs}ms` } as CSSProperties}
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
              variant="primary"
              size="lg"
              icon="play"
            >
              {playLabel}
            </SpatialButtonLink>
            <SpatialButtonLink
              focusKey={HERO_DETAILS_KEY}
              onArrowPress={pageForward}
              href={slide.detailsHref}
              variant="secondary"
              size="lg"
              icon="info"
            >
              {detailsLabel}
            </SpatialButtonLink>
          </SpatialGroup>
        </div>

        {count > 1 && (
          <>
            {/* Pointer only: the keyboard pages with the buttons and the strip. */}
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
        <div className={styles.strip}>
          <SpatialGroup
            focusKey="hero-strip"
            as="ul"
            className={styles.posters}
            rememberLastChild={false}
            preferredChildFocusKey={stripKey(slide)}
          >
            {slides.map((item, position) => (
              <li key={item.id} className={styles.item}>
                <StripPoster
                  slide={item}
                  active={position === current}
                  showImage={stripImages}
                  paused={paused}
                  onSelect={() => setIndex(position)}
                  onArrowPress={holdEnds(position)}
                />
              </li>
            ))}
          </SpatialGroup>
        </div>
      )}
    </section>
  );
}
