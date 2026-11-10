import type { CollectionRef, ItemRef } from '@slouchcat/api-client';
import { type ReactNode, useId } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { paths } from '@/app/paths';
import { PosterCard, RowList, SpatialButtonLink, SpatialGroup } from '@/ui';
import { isAudioKind, isFinished } from './labels';
import styles from './ItemPage.module.css';

interface MemberShelfProps {
  /** Focus key prefix of the cards: `<prefix>-<member id>`. */
  keyPrefix: string;
  name: string;
  /** "N Titel", "N Bände". */
  count: string;
  /** The page with every member, and the spoken name of the link to it. */
  href: string;
  showAllLabel: string;
  /** The members in their order (collection order, series order). */
  members: readonly ItemRef[];
  /** The title of the detail page: its card carries the ring (`active`, `aria-current`). */
  currentId: string;
  /**
   * The watchlist button on a member's cover (concept 17 D-9, E-17), drawn beside the card's
   * link like the pen: Tab reaches it after the link, the arrow keys pass it by.
   */
  coverMark?: (member: ItemRef) => ReactNode;
}

/**
 * A shelf of titles that belong together, right after the head of a detail page: its name, the
 * count and "Alle zeigen", then the members in their order with this title ringed, finished ones
 * with a check and started ones with a bar. A collection (CollectionShelf) and an audiobook series
 * (concept 19 D4.3) use it.
 */
export function MemberShelf({
  keyPrefix,
  name,
  count,
  href,
  showAllLabel,
  members,
  currentId,
  coverMark,
}: MemberShelfProps) {
  const { t } = useTranslation();
  const headingId = useId();
  const memberKey = (member: ItemRef) => `${keyPrefix}-${member.id}`;
  const current = members.find((member) => member.id === currentId);
  return (
    <section className={styles.shelf} aria-labelledby={headingId}>
      <SpatialGroup className={`${styles.collectionHead} ${styles.gutter}`}>
        <h2 id={headingId} className={styles.collectionTitle}>
          {name}
        </h2>
        <span className={styles.muted}>{count}</span>
        <SpatialButtonLink
          href={href}
          variant="ghost"
          icon="chevronRight"
          aria-label={showAllLabel}
        >
          {t('item.collection.showAll')}
        </SpatialButtonLink>
      </SpatialGroup>
      {members.length > 0 && (
        <RowList
          aria-label={name}
          preferredChildFocusKey={current ? memberKey(current) : undefined}
        >
          {members.map((member) => {
            const progress = member.progress;
            const finished = isFinished(member);
            const audio = isAudioKind(member.kind);
            const active = member.id === currentId;
            return (
              <PosterCard
                key={member.id}
                focusKey={memberKey(member)}
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

interface CollectionShelfProps {
  collection: CollectionRef;
  /** The title of the detail page: its card carries the ring (`active`, `aria-current`). */
  currentId: string;
  /** The watchlist button on a member's cover (MemberShelf `coverMark`). */
  coverMark?: (member: ItemRef) => ReactNode;
}

/**
 * One collection the title belongs to, right after the head (concept 17 D-4, variant 2; E-8): its
 * name, "N Titel" and "Alle zeigen", then the members in collection order (MemberShelf).
 */
export function CollectionShelf({ collection, currentId, coverMark }: CollectionShelfProps) {
  const { t } = useTranslation();
  // An older server sends no members: the cover (the first member) stands for the collection.
  const members = collection.members ?? (collection.cover ? [collection.cover] : []);
  return (
    <MemberShelf
      keyPrefix={`collection-${collection.id}`}
      name={collection.name}
      count={t('collection.count', { count: collection.count })}
      href={paths.collection(collection.id)}
      showAllLabel={t('item.collection.showAllLabel', { name: collection.name })}
      members={members}
      currentId={currentId}
      coverMark={coverMark}
    />
  );
}
