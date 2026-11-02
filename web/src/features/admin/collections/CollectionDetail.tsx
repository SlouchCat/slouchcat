import type { AdminCollection, AdminCollectionItem, AdminItemHit } from '@slouchcat/api-client';
import { useMutation, useQuery, type UseQueryResult } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useId, useMemo, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { Icon, Notice, Skeleton, Text } from '@/ui';
import { AriaButton, ImageTile, RankHint, RankSelect, rankTo } from '@/ui/aria';
import { TitleSearch } from '../CurationParts';
import { adminCollectionQuery, setCollectionItems } from '../queries';
import styles from './Collections.module.css';

/** A member as the fold-out shows it: from the stored list, or a hit picked from the search. */
export type Member = Pick<AdminCollectionItem, 'itemId' | 'kind' | 'title' | 'year' | 'posterUrl'>;

/** Kinds a collection may hold: seasons, episodes and extras are refused by the server. */
const MEMBER_KINDS: readonly AdminItemHit['kind'][] = ['movie', 'series', 'audiobook', 'podcast'];

/** «Film · 1995»: what tells two members of the same name apart. */
function memberMeta(t: TFunction, member: Member): string {
  const kind = t(`kind.${member.kind}`);
  return member.year === undefined
    ? kind
    : t('admin.collections.members.meta', { kind, year: String(member.year) });
}

/** The poster of a member: decorative, the title stands next to it. */
function Poster({ member }: { member: Member }) {
  return (
    <ImageTile
      src={member.posterUrl ? apiUrl(member.posterUrl) : undefined}
      title={member.title}
      shape="portrait"
    />
  );
}

/** Loading and failing of the stored members, the same in both fold-outs. */
function StoredState({ stored }: { stored: UseQueryResult<AdminCollection> }) {
  const { t } = useTranslation();
  if (stored.isPending) return <Skeleton shape="block" />;
  if (stored.isError) {
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, stored.error)}
        actions={
          <AriaButton icon="refresh" onPress={() => void stored.refetch()}>
            {t('common.retry')}
          </AriaButton>
        }
      />
    );
  }
  return null;
}

/**
 * The fold-out of a manual collection (SAM-3, in the row instead of the former dialog): the title
 * search adds at the end, each member takes its place by RankSelect (S-16, the others move along)
 * or leaves; «Speichern» replaces the members at once, «Verwerfen» drops the changes. The order
 * being edited is the page's draft of the collection (S-17), so folding the row away or switching
 * tabs keeps it. «Umbenennen» and «Löschen» act on the whole collection.
 */
export function ManualMembers({
  collection,
  hold,
  memberIds,
  dirty,
  lookup,
  onChange,
  onPick,
  onDiscard,
  onSaved,
  onRename,
  onRemove,
}: {
  collection: AdminCollection;
  /** A dialog is open or a draft of an open row differs: no refetch on window focus. */
  hold: boolean;
  /** The member ids in order: the draft, else what the server holds; undefined while loading. */
  memberIds: readonly string[] | undefined;
  dirty: boolean;
  lookup: (itemId: string) => Member | undefined;
  onChange: (collection: AdminCollection, itemIds: string[]) => void;
  onPick: (collection: AdminCollection, hit: AdminItemHit) => void;
  onDiscard: (collection: AdminCollection) => void;
  onSaved: (saved: AdminCollection) => void;
  onRename: (collection: AdminCollection) => void;
  onRemove: (collection: AdminCollection) => void;
}) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const headingId = useId();
  const hintId = useId();
  const searchRef = useRef<HTMLDivElement>(null);
  const saveButton = useRef<HTMLButtonElement>(null);
  const stored = useQuery({
    ...adminCollectionQuery(collection.id),
    refetchOnWindowFocus: !hold,
  });
  const save = useMutation({
    mutationFn: (itemIds: readonly string[]) => setCollectionItems(collection.id, [...itemIds]),
    onSuccess: onSaved,
  });
  const ids = memberIds ?? [];
  // Positions and «Speichern» go by the ids, so a member is never dropped; the page knows every
  // member it has drawn (lookup), so each id has its title.
  const members = ids.flatMap((id, position) => {
    const member = lookup(id);
    return member ? [{ ...member, position }] : [];
  });
  const change = (next: string[]) => {
    if (save.error) save.reset();
    onChange(collection, next);
  };
  // The removed entry and its button are gone: the focus goes back to the search field.
  const focusSearch = () => searchRef.current?.querySelector('input')?.focus();

  return (
    <section
      className={styles.detail}
      aria-label={t('admin.collections.members.title', { name: collection.name })}
    >
      <StoredState stored={stored} />
      {save.error !== null && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
      {/* The search adds to the stored members: it waits for them, or a pick made while they
          load would replace the whole collection on «Speichern». */}
      {stored.isSuccess && (
        <div className={styles.columns}>
          <div ref={searchRef}>
            <TitleSearch
              label={t('admin.collections.members.search.label')}
              hint={t('admin.collections.members.search.hint')}
              emptyText={t('admin.collections.members.search.empty')}
              resultsLabel={t('admin.collections.members.search.results')}
              actionText={t('admin.collections.members.add')}
              actionLabel={(hit) => t('admin.collections.members.addLabel', { title: hit.title })}
              onPick={(hit) => {
                if (save.error) save.reset();
                onPick(collection, hit);
              }}
              exclude={new Set(ids)}
              kinds={MEMBER_KINDS}
            />
          </div>
          <div className={styles.members}>
            <div className={styles.membersHead}>
              <h3 id={headingId} className={styles.membersTitle}>
                {t('admin.collections.members.list')}
              </h3>
              <span className={styles.muted}>
                {t('admin.collections.members.count', {
                  count: ids.length,
                  number: number.format(ids.length),
                })}
              </span>
            </div>
            {ids.length === 0 ? (
              <Text tone="secondary">{t('admin.collections.members.empty')}</Text>
            ) : (
              <>
                <RankHint id={hintId} />
                <ol className={styles.list} aria-labelledby={headingId}>
                  {members.map((member) => (
                    <li key={member.itemId} className={styles.entry}>
                      <RankSelect
                        name={member.title}
                        position={member.position}
                        count={ids.length}
                        describedBy={hintId}
                        onChange={(to) => change(rankTo(ids, member.position, to, 'move'))}
                      />
                      <span className={styles.thumb}>
                        <Poster member={member} />
                      </span>
                      <span className={styles.entryText}>
                        <span>{member.title}</span>
                        <span className={styles.muted}>{memberMeta(t, member)}</span>
                      </span>
                      <AriaButton
                        variant="ghost"
                        icon="trash"
                        aria-label={t('admin.collections.members.removeLabel', {
                          title: member.title,
                        })}
                        onPress={() => {
                          change(ids.filter((id) => id !== member.itemId));
                          focusSearch();
                        }}
                      >
                        {t('admin.collections.members.remove')}
                      </AriaButton>
                    </li>
                  ))}
                </ol>
              </>
            )}
          </div>
        </div>
      )}
      <div className={styles.actions}>
        {dirty && (
          <>
            <Text tone="secondary">{t('admin.collections.members.unsaved')}</Text>
            <AriaButton
              variant="ghost"
              isDisabled={save.isPending}
              onPress={() => {
                save.reset();
                onDiscard(collection);
                // The button goes with the draft; the focus stays with the members' buttons.
                saveButton.current?.focus();
              }}
            >
              {t('admin.collections.members.discard')}
            </AriaButton>
          </>
        )}
        <AriaButton
          ref={saveButton}
          variant="primary"
          isPending={save.isPending}
          isDisabled={!stored.isSuccess}
          onPress={() => save.mutate(ids)}
        >
          {save.isPending ? t('common.saving') : t('common.save')}
        </AriaButton>
        <span className={styles.spacer} />
        <AriaButton
          variant="secondary"
          aria-label={t('admin.collections.rename.label', { name: collection.name })}
          onPress={() => onRename(collection)}
        >
          {t('admin.collections.rename.action')}
        </AriaButton>
        <AriaButton
          variant="danger"
          icon="trash"
          aria-label={t('admin.collections.remove.label', { name: collection.name })}
          onPress={() => onRemove(collection)}
        >
          {t('admin.collections.remove.action')}
        </AriaButton>
      </div>
    </section>
  );
}

/**
 * The fold-out of a TMDB collection (SAM-3): its members as posters, only to read. The provider
 * owns it, so members, name and «Löschen» are locked (the server answers `not_editable`).
 */
export function TmdbMembers({ collection, hold }: { collection: AdminCollection; hold: boolean }) {
  const { t } = useTranslation();
  const headingId = useId();
  const stored = useQuery({
    ...adminCollectionQuery(collection.id),
    refetchOnWindowFocus: !hold,
  });
  const members = stored.data?.items ?? [];
  return (
    <section
      className={styles.detail}
      aria-label={t('admin.collections.members.title', { name: collection.name })}
    >
      <p className={styles.lockNote}>
        <Icon name="lock" size="sm" />
        {t('admin.collections.lockedNote')}
      </p>
      <StoredState stored={stored} />
      {stored.isSuccess && (
        <div className={styles.members}>
          <h3 id={headingId} className={styles.membersTitle}>
            {t('admin.collections.members.list')}
          </h3>
          {members.length === 0 ? (
            <Text tone="secondary">{t('admin.collections.members.empty')}</Text>
          ) : (
            <ol className={styles.tiles} aria-labelledby={headingId}>
              {members.map((member) => (
                <li key={member.itemId} className={styles.tile}>
                  <Poster member={member} />
                  <span>{member.title}</span>
                  <span className={styles.muted}>{memberMeta(t, member)}</span>
                </li>
              ))}
            </ol>
          )}
        </div>
      )}
    </section>
  );
}
