import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate, formatDateTime } from '@/lib/format';
import { SpatialButton, SpatialButtonLink, SpatialGroup } from '@/ui';
import type { Wish } from './queries';
import { eventText, failText, seasonsText, WishStateChip } from './state';
import styles from './Wishes.module.css';

/** The newest steps of a wish the card shows; «Meine Wünsche» has the whole history. */
const CARD_EVENTS = 5;

export const WITHDRAW_KEY = 'wish-withdraw';
const MY_WISHES_KEY = 'wish-mine';

interface WishCardProps {
  wish: Wish;
  /** «Zurückziehen» (owner, not approved yet) or «Nicht mehr mitwünschen» (joined, F-17). */
  onWithdraw: () => void;
  withdrawing: boolean;
}

/**
 * The wish card under the head of the preview page (concept 25, 5.1, M-03): the state, the seasons
 * and the date, whether the profile joined another profile's wish, the newest steps, and the
 * actions «Zu meinen Wünschen» and «Zurückziehen» where the server allows it.
 */
export function WishCard({ wish, onWithdraw, withdrawing }: WishCardProps) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const joined = wish.role === 'joined';
  const line = [
    wish.media === 'show' ? seasonsText(t, wish.seasons) : t('wishes.kind.movie'),
    t('wishes.card.requestedOn', { date: formatDate(wish.createdAt, language) }),
    ...(joined ? [t('wishes.card.joined')] : []),
  ]
    .filter(Boolean)
    .join(t('format.separator'));
  return (
    <section className={styles.card} aria-labelledby="wish-card-title">
      <div className={styles.cardHead}>
        <h2 id="wish-card-title" className={styles.cardTitle}>
          {t('wishes.card.title')}
        </h2>
        <WishStateChip state={wish.state} detail={wish.stateDetail} />
      </div>
      <p className={styles.line}>{line}</p>
      {wish.failReason && <p className={styles.line}>{failText(t, wish.failReason)}</p>}
      {wish.events.length > 0 && (
        <ol className={styles.events} aria-label={t('wishes.card.history')}>
          {wish.events.slice(0, CARD_EVENTS).map((event, index) => (
            <li key={`${event.at}-${index}`} className={styles.event}>
              <span>{eventText(t, event)}</span>
              <time className={styles.eventTime} dateTime={event.at}>
                {formatDateTime(event.at, language)}
              </time>
            </li>
          ))}
        </ol>
      )}
      <SpatialGroup className={styles.actions}>
        <SpatialButtonLink
          focusKey={MY_WISHES_KEY}
          href={paths.wishes}
          variant="secondary"
          icon="heart"
        >
          {t('wishes.card.mine')}
        </SpatialButtonLink>
        {wish.canWithdraw && (
          <SpatialButton
            focusKey={WITHDRAW_KEY}
            variant="ghost"
            icon="close"
            aria-disabled={withdrawing || undefined}
            onPress={() => {
              if (!withdrawing) onWithdraw();
            }}
          >
            {t(joined ? 'wishes.withdraw.actionJoined' : 'wishes.withdraw.action')}
          </SpatialButton>
        )}
      </SpatialGroup>
    </section>
  );
}
