import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate, formatDateTime } from '@/lib/format';
import { PosterFallback, SpatialButton, SpatialButtonLink, SpatialGroup } from '@/ui';
import type { Wish } from './queries';
import { eventText, failText, seasonsText, WishStateChip } from './state';
import { WishProgressLine } from './WishProgress';
import styles from './Wishes.module.css';

/** The poster's rendered width (`--sc-space-16`, `.row` in Wishes.module.css) for its `srcset`. */
const POSTER_SIZES = '4rem';

/** The focus key of a row's first control (its title link), for the page's start and its arrows. */
export const wishRowKey = (wish: Wish) => `wish-${wish.id}`;

interface WishRowProps {
  wish: Wish;
  onWithdraw: (wish: Wish) => void;
  onRequestAgain: (wish: Wish) => void;
  /** A withdrawal of this wish is on its way. */
  busy: boolean;
}

/**
 * One wish of «Meine Wünsche» (concept 25, 5.3, M-08): the poster without a frame, the title, the
 * seasons, the state with its date and, in procurement, its download progress, why it failed, the history folded away (closed at first, its
 * last step in the toggle), and the actions the server allows: «Zum Titel» (the library title
 * once the profile sees it, else the preview), «Zurückziehen», «Erneut anfordern».
 */
export function WishRow({ wish, onWithdraw, onRequestAgain, busy }: WishRowProps) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const [open, setOpen] = useState(false);
  const historyId = useId();
  const key = wishRowKey(wish);
  // The library title's poster, else the one of the title outside the library: w200 and w400.
  const images = wish.item?.images ?? wish.external?.images;
  const posterUrl = imageUrl(images?.poster);
  const posterSmallUrl = imageUrl(images?.posterSmall);
  const source = posterSmallUrl ?? posterUrl;
  const href = wish.item ? paths.item(wish.item.id) : paths.title(wish.media, wish.tmdbId);
  const when = wish.state === 'available' && wish.availableAt ? wish.availableAt : wish.updatedAt;
  const last = wish.events[0];
  const facts = [
    wish.media === 'show' ? seasonsText(t, wish.seasons) : t('wishes.kind.movie'),
    ...(wish.role === 'joined' ? [t('wishes.card.joined')] : []),
    ...(wish.origin === 'seerr' ? [t('wishes.row.fromSeerr')] : []),
  ].filter(Boolean);
  return (
    <article className={styles.row} aria-labelledby={`${historyId}-title`}>
      <span className={styles.rowPoster}>
        {source ? (
          <img
            src={source}
            srcSet={
              posterUrl && posterSmallUrl ? `${posterSmallUrl} 200w, ${posterUrl} 400w` : undefined
            }
            sizes={posterUrl && posterSmallUrl ? POSTER_SIZES : undefined}
            alt=""
            width={200}
            height={300}
            loading="lazy"
            decoding="async"
          />
        ) : (
          <PosterFallback />
        )}
      </span>
      <div className={styles.rowBody}>
        {/* Under the list's h2 (the chosen filter). */}
        <h3 id={`${historyId}-title`} className={styles.rowTitle}>
          {wish.title}
          {wish.year ? <span className={styles.rowYear}>{wish.year}</span> : null}
        </h3>
        <p className={styles.line}>{facts.join(t('format.separator'))}</p>
        <p className={styles.rowState}>
          <WishStateChip state={wish.state} detail={wish.stateDetail} />
          <time dateTime={when}>{formatDate(when, language)}</time>
        </p>
        <WishProgressLine progress={wish.progress} />
        {wish.failReason && <p className={styles.line}>{failText(t, wish.failReason)}</p>}
        <SpatialGroup className={styles.actions}>
          <SpatialButtonLink focusKey={key} href={href} variant="secondary" size="sm">
            {t('wishes.row.open')}
          </SpatialButtonLink>
          {wish.canWithdraw && (
            <SpatialButton
              focusKey={`${key}-withdraw`}
              variant="ghost"
              size="sm"
              icon="close"
              aria-disabled={busy || undefined}
              aria-label={t(
                wish.role === 'joined'
                  ? 'wishes.row.withdrawJoinedLabel'
                  : 'wishes.row.withdrawLabel',
                { title: wish.title },
              )}
              onPress={() => {
                if (!busy) onWithdraw(wish);
              }}
            >
              {t(
                wish.role === 'joined' ? 'wishes.withdraw.actionJoined' : 'wishes.withdraw.action',
              )}
            </SpatialButton>
          )}
          {wish.canRequestAgain && (
            <SpatialButton
              focusKey={`${key}-again`}
              variant="ghost"
              size="sm"
              icon="refresh"
              aria-label={t('wishes.row.againLabel', { title: wish.title })}
              onPress={() => onRequestAgain(wish)}
            >
              {t('wishes.row.again')}
            </SpatialButton>
          )}
          {wish.events.length > 0 && (
            <SpatialButton
              focusKey={`${key}-history`}
              variant="ghost"
              size="sm"
              icon={open ? 'chevronUp' : 'chevronDown'}
              aria-expanded={open}
              aria-controls={historyId}
              onPress={() => setOpen(!open)}
            >
              {last
                ? t('wishes.row.history', { last: eventText(t, last) })
                : t('wishes.card.history')}
            </SpatialButton>
          )}
        </SpatialGroup>
        <ol
          id={historyId}
          hidden={!open}
          className={styles.events}
          aria-label={t('wishes.card.history')}
        >
          {wish.events.map((event, index) => (
            <li key={`${event.at}-${index}`} className={styles.event}>
              <span>{eventText(t, event)}</span>
              <time className={styles.eventTime} dateTime={event.at}>
                {formatDateTime(event.at, language)}
              </time>
            </li>
          ))}
        </ol>
      </div>
    </article>
  );
}
