import type { ItemRef } from '@slouchcat/api-client';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl, imageUrl } from '@/api/client';
import { Backdrop, itemAccentStyle, PosterFallback, SpatialButton, TitleOrLogo } from '@/ui';
import styles from './DetailHeader.module.css';

/** The round back arrow over the head; the page's start focus when it has no big button. */
export const BACK_KEY = 'detail-back';

/** The page around a DetailHeader: room for its backdrop, the sections below in one column. */
export function DetailPage({ children }: { children: ReactNode }) {
  return <main className={styles.page}>{children}</main>;
}

/** The sections under the head (collection, seasons, file, extras, cast), in this order (E-9). */
export function DetailBody({ children }: { children: ReactNode }) {
  return <div className={styles.body}>{children}</div>;
}

// Rendered cover width (DetailHeader.module.css, wide layout only).
const POSTER_SIZES = 'clamp(13rem, 20vw, 18rem)';

interface DetailHeaderProps {
  images: ItemRef['images'];
  accent?: string;
  /** `poster` 2:3, `square` for audiobooks and podcasts (M6), `still` 16:9 for an episode. */
  cover: 'poster' | 'square' | 'still';
  onBack: () => void;
  /** Over the top right of the cover: the pen of the metadata editor (concept 15 7.4). */
  coverTool?: ReactNode;
  /**
   * Over the bottom right of the cover: the watchlist button (concept 17 D-9, E-17). Like the
   * cover itself only shown from 900 px; below, "Merken" in the row of actions stays.
   */
  coverMark?: ReactNode;
  /** The line above the title: "S01E02 · Serie", or the series as a link over a season. */
  kicker?: ReactNode;
  title: string;
  logoUrl?: string;
  /** Lines under the title: the original title, author and narrator of an audio title. */
  lines?: readonly string[];
  /** Year, runtime, episodes … (no age rating, concept 17 F-10). */
  facts?: ReactNode;
  genres?: string;
  tagline?: string;
  /** The big button and the row of round actions (DetailActions). */
  actions?: ReactNode;
  /** Links to the neighbours of an episode, an extra or a podcast episode. */
  related?: ReactNode;
  overview?: string;
}

/**
 * The head of every detail page (concept 17 D-3): film, series, season, episode, audio titles and
 * collections share it. The round back arrow top left, the cover, logo or title with its facts,
 * tagline, actions and overview over the title's backdrop.
 */
export function DetailHeader({
  images,
  accent,
  cover,
  onBack,
  coverTool,
  coverMark,
  kicker,
  title,
  logoUrl,
  lines = [],
  facts,
  genres,
  tagline,
  actions,
  related,
  overview,
}: DetailHeaderProps) {
  const { t } = useTranslation();
  const still = images.backdropSmall ?? images.backdrop;
  const hasArt = Boolean(
    images.backdrop ?? images.backdropSmall ?? images.backdropLarge ?? images.poster,
  );

  let coverImage: ReactNode;
  if (cover === 'still' && still) {
    coverImage = (
      <img
        src={apiUrl(still.url)}
        alt=""
        width={still.width}
        height={still.height}
        className={styles.stillImage}
      />
    );
  } else if (images.poster) {
    coverImage = (
      <img
        src={apiUrl(images.poster.url)}
        srcSet={
          images.posterLarge
            ? `${apiUrl(images.poster.url)} 400w, ${apiUrl(images.posterLarge.url)} 800w`
            : undefined
        }
        sizes={images.posterLarge ? POSTER_SIZES : undefined}
        alt=""
        width={400}
        height={cover === 'square' ? 400 : 600}
        className={cover === 'square' ? styles.coverImage : styles.posterImage}
      />
    );
  } else {
    coverImage = (
      <span
        className={
          cover === 'still'
            ? styles.stillFallback
            : cover === 'square'
              ? styles.coverFallback
              : styles.posterFallback
        }
      >
        <PosterFallback />
      </span>
    );
  }

  return (
    <div className={styles.head}>
      <Backdrop
        url={imageUrl(images.backdrop)}
        smallUrl={imageUrl(images.backdropSmall)}
        largeUrl={imageUrl(images.backdropLarge)}
        uhdUrl={imageUrl(images.backdropUhd)}
        uhdWidth={images.backdropUhd?.width}
        posterUrl={imageUrl(images.poster)}
        accent={accent}
      />
      <div className={styles.back}>
        <SpatialButton
          focusKey={BACK_KEY}
          variant="secondary"
          icon="back"
          iconOnly
          overlay
          aria-label={t('common.back')}
          title={t('common.back')}
          onPress={onBack}
        />
      </div>

      <div className={hasArt ? styles.layout : `${styles.layout} ${styles.noArt}`}>
        <div className={styles.poster} style={itemAccentStyle(accent)}>
          {coverImage}
          {coverTool && <div className={styles.posterTool}>{coverTool}</div>}
          {coverMark && <div className={styles.posterMark}>{coverMark}</div>}
        </div>

        <div className={styles.info}>
          <div className={styles.heading}>
            {kicker && <p className={styles.kicker}>{kicker}</p>}
            <h1 className={styles.title}>
              <TitleOrLogo title={title} logoUrl={logoUrl} />
            </h1>
            {lines.map((line) => (
              <p key={line} className={styles.line}>
                {line}
              </p>
            ))}
          </div>
          {(facts || genres) && (
            <div className={styles.factLines}>
              {facts}
              {genres && <p className={styles.genres}>{genres}</p>}
            </div>
          )}
          {tagline && <p className={styles.tagline}>{tagline}</p>}
          {actions}
          {related}
          {overview && <p className={styles.overview}>{overview}</p>}
        </div>
      </div>
    </div>
  );
}
