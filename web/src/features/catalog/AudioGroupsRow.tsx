import type { components } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { CollectionCard, Row } from '@/ui';
import { useBackFocus } from './useBackFocus';
import styles from './LibraryPage.module.css';

type AudioGroup = components['schemas']['AudioGroup'];

/** Focus key prefix of the row's tiles: `library-group-<id>`. */
const GROUP_KEY = 'library-group';

export const groupKey = (group: Pick<AudioGroup, 'id'>) => `${GROUP_KEY}-${group.id}`;

/**
 * The row «Reihen und Sammlungen» above the grid of an audiobook library (concept 19 D4.2,
 * variant B): one square tile per series and audiobook collection, in the server's order (by
 * name), with the number of volumes (or how many of them pass the filters) and a bar of the
 * volumes heard. A series opens its page, a collection the collection page. The browser's back
 * returns to the tile it was left from.
 */
export function AudioGroupsRow({
  libraryId,
  groups,
}: {
  libraryId: number;
  groups: readonly AudioGroup[];
}) {
  const { t } = useTranslation();
  useBackFocus(GROUP_KEY, groups.map(groupKey));
  return (
    <div className={styles.groups}>
      <Row title={t('library.groups.title')} focusKey={GROUP_KEY}>
        {groups.map((group) => {
          const series = group.kind === 'series';
          const { count, matching, completed } = group;
          const amount = series
            ? t('library.groups.volumes', { count })
            : t('library.groups.books', { count });
          const badge =
            matching < count
              ? t(series ? 'library.groups.matchingVolumes' : 'library.groups.matchingBooks', {
                  count,
                  matching,
                })
              : amount;
          const caption = [series ? group.author : t('library.groups.collection'), amount]
            .filter(Boolean)
            .join(t('format.separator'));
          return (
            <CollectionCard
              key={`${group.kind}-${group.id}`}
              focusKey={groupKey(group)}
              href={series ? paths.audioSeries(libraryId, group.id) : paths.collection(group.id)}
              name={group.name}
              caption={caption}
              shape="square"
              badge={badge}
              posterUrl={imageUrl(group.cover?.images.poster)}
              posterSmallUrl={imageUrl(group.cover?.images.posterSmall)}
              accent={group.cover?.accent}
              progress={
                completed > 0 && count > 0
                  ? {
                      percent: (completed / count) * 100,
                      label: t('library.groups.heard', { completed, count }),
                    }
                  : undefined
              }
            />
          );
        })}
      </Row>
    </div>
  );
}
