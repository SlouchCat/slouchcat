import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';
import { SpatialButtonLink } from './Button';
import styles from './HeroCarousel.module.css';
import { SpatialGroup } from './spatial';

export interface HeroSlide {
  id: string;
  title: string;
  /** Same-origin backdrop URL (w1280); the poster is used blurred when there is none. */
  backdropUrl?: string;
  /** The same backdrop in w780 for phones; the browser picks it through `srcset`. */
  backdropSmallUrl?: string;
  posterUrl?: string;
  /** Metadata line (chips for year, runtime, FSK). */
  meta?: ReactNode;
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
 * Full-bleed carousel at the top of the start page. Rotates every `intervalMs` unless the pointer
 * or focus is inside it (or reduced motion is requested). Arrow left on the first button and arrow
 * right on the last button page through the slides; the buttons keep focus.
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

  const pageBack = (direction: string) => {
    if (direction !== 'left' || count < 2) return true;
    stepRef.current(-1);
    return false;
  };
  const pageForward = (direction: string) => {
    if (direction !== 'right' || count < 2) return true;
    stepRef.current(1);
    return false;
  };

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
              onError={active ? onArtShown : undefined}
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
        <div className={styles.dots}>
          {slides.map((item, position) => (
            <button
              key={item.id}
              type="button"
              tabIndex={-1}
              className={styles.dot}
              data-active={position === current || undefined}
              data-paused={paused || undefined}
              aria-current={position === current ? 'true' : undefined}
              aria-label={t('ui.carousel.show', { index: position + 1, count })}
              onClick={() => setIndex(position)}
            >
              {position === current && <span key={current} className={styles.timer} />}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}
