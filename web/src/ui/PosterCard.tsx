import type { ReactNode, Ref } from 'react';
import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import { Chip } from './Chip';
import { Icon } from './Icon';
import styles from './PosterCard.module.css';
import { PosterFallback } from './PosterFallback';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

// Rendered width of a card per breakpoint (Row.module.css, size.poster.* tokens; below 768 px a
// third of the row, concept 08, N-3); lets the browser pick the w200 or w400 poster. w800 is left
// out on purpose: a phone at 3x would take it, and the start page must not get heavier (P-10). The
// server's poster preload and prepaint copy the srcset widths and this value (posterSizes in
// server/internal/api/preload.go, #poster-prepaint in global.css); change them together.
const CARD_SIZES = '(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, calc(33vw - 1.5rem)';

/**
 * Fades out the first cards the server wrote into the shell of a start page without a stage
 * (`#poster-prepaint`, api/preload.go) and removes them afterwards. Safe to call more than once or
 * without them.
 */
export function dismissPosterPrepaint() {
  const prepaint = document.getElementById('poster-prepaint');
  if (!prepaint || prepaint.dataset.done !== undefined) return;
  prepaint.addEventListener('transitionend', () => prepaint.remove(), { once: true });
  prepaint.dataset.done = '';
}

interface PosterCardProps {
  /** The title's page. Without it the card is a button that calls `onPress` (concept 08, 7.2). */
  href?: string;
  /** Called on Enter, Space or a click on a card without `href`. */
  onPress?: () => void;
  /** The chosen card of a row of buttons: a ring in the title's accent. */
  active?: boolean;
  /** A card of such a row that is not the chosen one: the poster at reduced strength. */
  dimmed?: boolean;
  'aria-current'?: 'true';
  /** Roving tabindex of a row of buttons: only the chosen card is a Tab stop. */
  tabIndex?: number;
  /** Return false to consume an arrow press (the ends of a row that must hold the focus). */
  onArrowPress?: (direction: string) => boolean;
  /** Drawn inside the poster's frame, over the image (the time bar of the spotlight row). */
  children?: ReactNode;
  title: string;
  /** Secondary line under the title (year, remaining time). */
  caption?: string;
  /** The caption on one line, cut with an ellipsis (a grid whose rows have a fixed height). */
  clampCaption?: boolean;
  /**
   * Spoken name of the card when the title and caption shown do not say enough (a card of
   * "Weiterschauen" that shows the series also names its episode, concept 15 P-1). Without it the
   * card is named by its text.
   */
  label?: string;
  /** Same-origin poster URL (w400), or undefined for the lettered fallback tile. */
  posterUrl?: string;
  /** The same poster in w200, offered to the browser through `srcset`. */
  posterSmallUrl?: string;
  /** The title's accent colour (`#rrggbb`, ItemRef.accent): placeholder tint and focus glow. */
  accent?: string;
  /** Small label on the poster (the kind of an extra: "Trailer", "Featurette"). */
  badge?: string;
  /**
   * A finished title: a check mark on the poster, with this as its spoken name ("Gesehen"). A
   * display only, for cards without `check` (the offline library).
   */
  watchedLabel?: string;
  /** Watch progress 0 … 100 with its accessible label. */
  progress?: { percent: number; label: string };
  /** Load the image eagerly (the first cards of the first row); every other poster is lazy. */
  eager?: boolean;
  /**
   * Fetch the image with high priority: the first card of a start page without a stage, the page's
   * first image, which the server also preloads with these URLs and sizes (P-10).
   */
  priority?: boolean;
  /** Called once the image is on screen (loaded and painted) or has failed. */
  onImageShown?: () => void;
  /** Keep the image out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  /**
   * `square` for audiobooks and podcasts (cover 1:1, M6); the image URLs stay the same. The frame
   * stays 2:3 like every other card, so the titles of a row or a grid start on one line: the
   * cover sits in its middle on a blurred, darkened copy of itself (concept 18 C-1, variant B).
   */
  shape?: 'poster' | 'square';
  /**
   * `own`: a square cover in a square frame of its own, without the blurred ground (concept 19 C):
   * the library overview of audiobooks and podcasts, where every card of the grid is square.
   * Ignored for `shape="poster"`.
   */
  frame?: 'poster' | 'own';
  /**
   * A control beside the card's link (or button), over the top left of its cover: the pen
   * (concept 15 7.4; on the left since concept 20, the check mark has the right). Not inside the link, so Tab reaches it after the link and the arrow keys pass
   * it by. Card and tool then share one cell that carries the focus scale, so the card stays large
   * while the pointer or the focus is on the tool (concept 17 D-7).
   */
  tool?: ReactNode;
  /**
   * The check mark «gesehen» beside the card's link, over the top right of its cover (concept 20
   * P-5, WatchedToggle): after the pen in the DOM, so Tab goes link, pen, check, mark. With a
   * pointer an empty check shows only while the pointer is on the cell or the focus is in it; a
   * set or half one always, and on screens without hover every check shows (O-1).
   */
  check?: ReactNode;
  /**
   * A second control beside the card's link, over the bottom right of its cover: the watchlist
   * button (concept 17 D-9). In the same cell as `tool` and after it in the DOM, so Tab goes link,
   * pen, mark. Shown while the pointer is on the cell or the focus is in it, always while it is
   * pressed (`aria-pressed="true"`) and on screens without hover. With a mark the progress bar
   * ends left of the button.
   */
  mark?: ReactNode;
  focusKey?: string;
}

/**
 * 2:3 poster (or a square cover in a 2:3 frame, or in a square frame of its own) with its title below, one link to the title. The link carries the focus ring and
 * scales on focus (focus.scale token); a soft glow in the title's accent colour lights up behind
 * the poster, and the same colour tints the frame while the image is still on its way.
 *
 * Without `href` the card is a `<button>` with the same look that calls `onPress` instead of
 * navigating: the spotlight row under the band, where a card shows its title in the band (F-5).
 */
export function PosterCard({
  href,
  onPress,
  active = false,
  dimmed = false,
  'aria-current': ariaCurrent,
  tabIndex,
  onArrowPress,
  children,
  title,
  caption,
  clampCaption = false,
  label,
  posterUrl,
  posterSmallUrl,
  accent,
  badge,
  watchedLabel,
  progress,
  eager = false,
  priority = false,
  onImageShown,
  deferImage = false,
  shape = 'poster',
  frame = 'poster',
  tool,
  check,
  mark,
  focusKey,
}: PosterCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLElement>({ focusKey, onArrowPress });
  const source = posterUrl ?? posterSmallUrl;
  const srcSet =
    posterUrl && posterSmallUrl ? `${posterSmallUrl} 200w, ${posterUrl} 400w` : undefined;
  const sizes = posterUrl && posterSmallUrl ? CARD_SIZES : undefined;
  // A square cover in a frame of its own (concept 19 C), else every card has the 2:3 frame.
  const own = shape === 'square' && frame === 'own';
  const framed = shape === 'square' && !own;
  const className = [
    styles.card,
    own && styles.own,
    active && styles.active,
    dimmed && styles.dimmed,
  ]
    .filter(Boolean)
    .join(' ');
  const content = (
    <>
      <span className={styles.frame}>
        {deferImage ? null : source ? (
          <>
            {framed && (
              // The blurred ground above and below a square cover (concept 18 C-1, variant B): the
              // same image with the same srcset and sizes, so the browser picks the same file and
              // loads it once.
              <img
                className={styles.ground}
                src={source}
                srcSet={srcSet}
                sizes={sizes}
                alt=""
                width={400}
                height={600}
                loading={eager ? 'eager' : 'lazy'}
                decoding="async"
                draggable={false}
                aria-hidden="true"
              />
            )}
            <img
              className={framed ? `${styles.image} ${styles.cover}` : styles.image}
              src={source}
              srcSet={srcSet}
              sizes={sizes}
              alt=""
              width={400}
              height={shape === 'square' ? 400 : 600}
              loading={eager ? 'eager' : 'lazy'}
              fetchPriority={priority ? 'high' : undefined}
              decoding="async"
              draggable={false}
              onLoad={
                onImageShown &&
                (() => {
                  // Two frames: the one that paints the image, then the next (as HeroCarousel does).
                  requestAnimationFrame(() => requestAnimationFrame(onImageShown));
                })
              }
              onError={onImageShown}
            />
          </>
        ) : (
          <PosterFallback />
        )}
        {badge && (
          <span className={styles.badge}>
            <Chip>{badge}</Chip>
          </span>
        )}
        {watchedLabel && (
          <span className={styles.watched} role="img" aria-label={watchedLabel} data-watched="">
            <Icon name="check" size="sm" />
          </span>
        )}
        {progress && (
          <span className={styles.progress}>
            <ProgressBar value={progress.percent} label={progress.label} />
          </span>
        )}
        {children}
      </span>
      <span className={styles.meta}>
        <span className={styles.title}>{title}</span>
        {caption && (
          <span className={clampCaption ? `${styles.caption} ${styles.clamped}` : styles.caption}>
            {caption}
          </span>
        )}
      </span>
    </>
  );
  const card =
    href === undefined ? (
      <button
        ref={ref as Ref<HTMLButtonElement>}
        type="button"
        className={`${styles.button} ${className}`}
        style={itemAccentStyle(accent)}
        aria-label={label}
        aria-current={ariaCurrent}
        tabIndex={tabIndex}
        onFocus={onFocus}
        // A press with the pointer chooses without taking the focus: the focus would scroll the
        // row to the card (spatial reveal) under the pointer and lose the click.
        onMouseDown={(event) => event.preventDefault()}
        onClick={onPress}
      >
        {content}
      </button>
    ) : (
      <AppLink
        ref={ref as Ref<HTMLAnchorElement>}
        href={href}
        className={className}
        style={itemAccentStyle(accent)}
        aria-label={label}
        aria-current={ariaCurrent}
        tabIndex={tabIndex}
        onFocus={onFocus}
      >
        {content}
      </AppLink>
    );
  if (tool === undefined && check === undefined && mark === undefined) return card;
  return (
    <div
      className={[styles.cell, mark !== undefined && styles.marked, own && styles.own]
        .filter(Boolean)
        .join(' ')}
    >
      {card}
      {tool !== undefined && <div className={styles.tool}>{tool}</div>}
      {check !== undefined && <div className={styles.check}>{check}</div>}
      {mark !== undefined && (
        <div className={styles.markArea}>
          <div className={styles.mark}>{mark}</div>
        </div>
      )}
    </div>
  );
}
