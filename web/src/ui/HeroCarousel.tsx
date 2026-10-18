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
}

export const HERO_PLAY_KEY = 'hero-play';
const HERO_DETAILS_KEY = 'hero-details';

const prefersReducedMotion = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

/**
 * Full-bleed carousel at the top of the start page. Rotates every `intervalMs` unless the pointer
 * or focus is inside it (or reduced motion is requested). Arrow left on the first button and arrow
 * right on the last button page through the slides; the buttons keep focus.
 */
export function HeroCarousel({
  label,
  slides,
  playLabel,
  detailsLabel,
  intervalMs = 8000,
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
          const src = item.backdropUrl ?? item.posterUrl;
          if (!src) return null;
          return (
            <img
              key={item.id}
              className={`${styles.image} ${item.backdropUrl ? '' : styles.blurred}`}
              data-active={position === current || undefined}
              src={src}
              alt=""
              width={1280}
              height={720}
              loading={position === 0 ? 'eager' : 'lazy'}
              fetchPriority={position === 0 ? 'high' : 'low'}
              decoding={position === 0 ? 'sync' : 'async'}
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
