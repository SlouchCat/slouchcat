import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate, formatDateTime } from '@/lib/format';
import { PosterFallback, SpatialButton, SpatialButtonLink, SpatialGroup } from '@/ui';
import type { Wish } from './queries';
import { eventText, failText, seasonsText, WishStateChip } from './state';
import styles from './Wishes.module.css';

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
 * seasons, the state with its date, why it failed, the history folded away (closed at first, its
 * last step in the toggle), and the actions the server allows: «Zum Titel» (the library title
 * once the profile sees it, else the preview), «Zurückziehen», «Erneut anfordern».
 */
export function WishRow({ wish, onWithdraw, onRequestAgain, busy }: WishRowProps) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const [open, setOpen] = useState(false);
  const historyId = useId();
  const key = wishRowKey(wish);
  const poster = wish.item?.images.poster ?? wish.external?.images.poster;
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
        {poster ? (
          <img src={imageUrl(poster)} alt="" width={poster.width} height={poster.height} />
        ) : (
          <PosterFallback />
        )}
      </span>
      <div className={styles.rowBody}>
        <h2 id={`${historyId}-title`} className={styles.rowTitle}>
          {wish.title}
          {wish.year ? <span className={styles.rowYear}>{wish.year}</span> : null}
        </h2>
        <p className={styles.line}>{facts.join(t('format.separator'))}</p>
        <p className={styles.rowState}>
          <WishStateChip state={wish.state} detail={wish.stateDetail} />
          <time dateTime={when}>{formatDate(when, language)}</time>
        </p>
        {wish.failReason && <p className={styles.line}>{failText(t, wish.failReason)}</p>}
        <SpatialGroup className={styles.actions}>
          <SpatialButtonLink focusKey={key} href={href} variant="secondary" size="sm">
            {t(wish.item ? 'wishes.row.play' : 'wishes.row.open')}
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
