import type { CollectionRef, ItemRef } from '@slouchcat/api-client';
import { type ReactNode, useId } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { PosterCard, RowList, SpatialButtonLink, SpatialGroup } from '@/ui';
import { isAudioKind, isFinished } from './labels';
import styles from './ItemPage.module.css';

const memberKey = (collection: CollectionRef, member: ItemRef) =>
  `collection-${collection.id}-${member.id}`;

interface CollectionShelfProps {
  collection: CollectionRef;
  /** The title of the detail page: its card carries the ring (`active`, `aria-current`). */
  currentId: string;
  /**
   * The watchlist button on a member's cover (concept 17 D-9, E-17), drawn beside the card's
   * link like the pen: Tab reaches it after the link, the arrow keys pass it by.
   */
  coverMark?: (member: ItemRef) => ReactNode;
}

/**
 * One collection the title belongs to, right after the head (concept 17 D-4, variant 2; E-8): its
 * name, "N Titel" and "Alle zeigen", then the members in collection order with this title ringed,
 * finished ones with a check and started ones with a bar.
 */
export function CollectionShelf({ collection, currentId, coverMark }: CollectionShelfProps) {
  const { t } = useTranslation();
  const headingId = useId();
  // An older server sends no members: the cover (the first member) stands for the collection.
  const members = collection.members ?? (collection.cover ? [collection.cover] : []);
  const current = members.find((member) => member.id === currentId);
  return (
    <section className={styles.shelf} aria-labelledby={headingId}>
      <SpatialGroup className={`${styles.collectionHead} ${styles.gutter}`}>
        <h2 id={headingId} className={styles.collectionTitle}>
          {collection.name}
        </h2>
        <span className={styles.muted}>{t('collection.count', { count: collection.count })}</span>
        <SpatialButtonLink
          href={paths.collection(collection.id)}
          variant="ghost"
          icon="chevronRight"
          aria-label={t('item.collection.showAllLabel', { name: collection.name })}
        >
          {t('item.collection.showAll')}
        </SpatialButtonLink>
      </SpatialGroup>
      {members.length > 0 && (
        <RowList
          aria-label={collection.name}
          preferredChildFocusKey={current ? memberKey(collection, current) : undefined}
        >
          {members.map((member) => {
            const progress = member.progress;
            const finished = isFinished(member);
            const audio = isAudioKind(member.kind);
            const active = member.id === currentId;
            return (
              <PosterCard
                key={member.id}
                focusKey={memberKey(collection, member)}
                href={paths.item(member.id)}
                title={member.title}
                caption={member.year !== undefined ? String(member.year) : undefined}
                posterUrl={imageUrl(member.images.poster)}
                posterSmallUrl={imageUrl(member.images.posterSmall)}
                accent={member.accent}
                shape={audio ? 'square' : 'poster'}
                active={active}
                aria-current={active ? 'true' : undefined}
                watchedLabel={
                  finished
                    ? t(audio ? 'library.status.heard' : 'library.status.watched')
                    : undefined
                }
                progress={
                  progress && !finished && progress.positionMs > 0
                    ? {
                        percent: progress.percent,
                        label: t(audio ? 'item.listened' : 'item.watched', {
                          percent: Math.round(progress.percent),
                        }),
                      }
                    : undefined
                }
                mark={coverMark?.(member)}
              />
            );
          })}
        </RowList>
      )}
    </section>
  );
}
