import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { quotaText } from '@/features/shared/quota';
import { intlLocale } from '@/i18n';
import { SpatialButtonLink, SpatialGroup, type IconName } from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import { wishCountsQuery, wishQuotaQuery } from '../queries';
import styles from './WishesArea.module.css';

export type WishesArea = 'discover' | 'movies' | 'series' | 'search' | 'mine';

const AREAS: readonly { key: WishesArea; href: string; icon: IconName }[] = [
  { key: 'discover', href: paths.discover, icon: 'star' },
  { key: 'movies', href: paths.discoverMovies, icon: 'film' },
  { key: 'series', href: paths.discoverSeries, icon: 'tv' },
  { key: 'search', href: paths.discoverSearch, icon: 'search' },
  { key: 'mine', href: paths.wishes, icon: 'heart' },
];

/**
 * The links between the pages of the area «Wünsche» (concept 25, 5.4, M-31 to M-34, M-08):
 * «Entdecken», «Filme», «Serien», «Suche» and «Meine Wünsche», the current one marked. Real
 * links, no tabs: each is a page of its own. The four pages of discovery show only while the
 * session says so (`wishes.discover`, the server's answer); with «Meine Wünsche» alone there is
 * nothing to choose and the row is left out. `limit` adds the account's limit under it (the
 * discovery pages; «Meine Wünsche» writes its own).
 */
export function WishesNav({ current, limit = false }: { current: WishesArea; limit?: boolean }) {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const shown = session.wishes?.discover === true;
  const counts = useQuery({ ...wishCountsQuery, enabled: shown });
  const quota = useQuery({ ...wishQuotaQuery, enabled: shown && limit });
  if (!shown) return null;
  const open = counts.data?.open ?? 0;
  const language = intlLocale(i18n.language);
  const limitText =
    limit && quota.data && !isApiError(quota.error)
      ? [quotaText(t, language, 'movie', quota.data), quotaText(t, language, 'tv', quota.data)]
          .filter(Boolean)
          .join(t('format.separator'))
      : '';
  return (
    <div className={styles.navBlock}>
      <nav aria-label={t('wishes.area.label')}>
        <SpatialGroup focusKey="wishes-area" className={styles.nav}>
          {AREAS.map((area) => (
            <SpatialButtonLink
              key={area.key}
              focusKey={`wishes-area-${area.key}`}
              href={area.href}
              icon={area.icon}
              size="sm"
              variant={area.key === current ? 'primary' : 'ghost'}
              aria-current={area.key === current ? 'page' : undefined}
            >
              {area.key === 'mine' && open > 0
                ? t('wishes.area.mineCount', { count: open })
                : t(`wishes.area.${area.key}`)}
            </SpatialButtonLink>
          ))}
        </SpatialGroup>
      </nav>
      {limitText && (
        <p className={discover.muted}>{t('wishes.page.limit', { limit: limitText })}</p>
      )}
    </div>
  );
}
