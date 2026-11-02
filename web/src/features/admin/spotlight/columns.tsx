import type { TFunction } from 'i18next';
import { StatusDot } from '@/ui';
import { AriaButton, type DataColumn } from '@/ui/aria';
import { PosterTitle } from '../PosterTitle';
import type { SpotlightEntry } from '../queries';
import styles from './Spotlight.module.css';

/** The name of an entry: its Trakt title, else its TMDB id. */
export const entryName = (t: TFunction, entry: SpotlightEntry) =>
  entry.title ?? t('admin.spotlight.entries.untitled', { id: entry.tmdbId });

/** The key of an entry in the tables and in the dialogs' state. */
export const entryKey = (entry: SpotlightEntry) => `${entry.media}-${entry.tmdbId}`;

const pinnedText = (t: TFunction, entry: SpotlightEntry) =>
  entry.pinnedRank === undefined
    ? null
    : t('admin.spotlight.entries.pinned', { rank: entry.pinnedRank });

const quoted = (t: TFunction, note: string) => t('admin.spotlight.entries.quoted', { note });

/** Title and «Film · 2021» in two lines. */
function TitleText({ t, entry }: { t: TFunction; entry: SpotlightEntry }) {
  return (
    <span className={styles.stack}>
      <span className={styles.title}>{entryName(t, entry)}</span>
      <span className={styles.muted}>
        {entry.year
          ? t('admin.spotlight.entries.meta', {
              media: t(`admin.spotlight.entries.media.${entry.media}`),
              year: entry.year,
            })
          : t(`admin.spotlight.entries.media.${entry.media}`)}
      </span>
    </span>
  );
}

export interface EntryActions {
  onPin: (entry: SpotlightEntry) => void;
  onNote: (entry: SpotlightEntry) => void;
  onChange: (entry: SpotlightEntry, body: { hidden?: boolean; pinnedRank?: null }) => void;
}

/**
 * The entries of the list (SPO-3): rank, poster and title, state, note and the actions. An entry
 * without a library title has no actions; it says why it never reaches the start page.
 */
export function entryColumns(
  t: TFunction,
  { onPin, onNote, onChange }: EntryActions,
): DataColumn<SpotlightEntry>[] {
  return [
    {
      key: 'rank',
      header: t('admin.spotlight.entries.columns.rank'),
      align: 'end',
      cell: (entry) => <span className={styles.rank}>{entry.position}</span>,
    },
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      // A hidden entry: poster and title at half strength (mockup Spo-A).
      cell: (entry) => (
        <PosterTitle posterUrl={entry.posterUrl} className={entry.hidden ? styles.dimmed : ''}>
          <TitleText t={t} entry={entry} />
        </PosterTitle>
      ),
    },
    {
      key: 'status',
      header: t('admin.spotlight.entries.columns.status'),
      cell: (entry) => {
        const [state, word] =
          entry.itemId === undefined
            ? (['warning', 'admin.spotlight.entries.state.missing'] as const)
            : entry.hidden
              ? (['pending', 'admin.spotlight.entries.state.hidden'] as const)
              : (['success', 'admin.spotlight.entries.state.visible'] as const);
        const pinned = pinnedText(t, entry);
        return (
          <span className={styles.stack}>
            <span className={styles.dot}>
              <StatusDot state={state} />
              {t(word)}
            </span>
            {pinned && <span className={styles.muted}>{pinned}</span>}
          </span>
        );
      },
    },
    {
      key: 'note',
      header: t('admin.spotlight.entries.columns.note'),
      // The whole note, wrapped where it has to: nothing hides behind a tooltip.
      cell: (entry) => (
        <span className={styles.noteCell}>
          <span className={entry.note === undefined ? styles.muted : styles.note}>
            {entry.note === undefined ? t('admin.spotlight.entries.none') : quoted(t, entry.note)}
          </span>
          {entry.itemId !== undefined && (
            <AriaButton
              variant="ghost"
              aria-label={t('admin.spotlight.note.label', { title: entryName(t, entry) })}
              onPress={() => onNote(entry)}
            >
              {t('admin.spotlight.note.action')}
            </AriaButton>
          )}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('admin.spotlight.entries.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => {
        if (entry.itemId === undefined) {
          return <span className={styles.muted}>{t('admin.spotlight.entries.notInLibrary')}</span>;
        }
        const title = entryName(t, entry);
        // Pin and unpin share their place: the button that had the focus keeps it, renamed.
        return (
          <span className={styles.rowActions}>
            {entry.pinnedRank === undefined ? (
              <AriaButton
                variant="ghost"
                aria-label={t('admin.spotlight.pin.label', { title })}
                onPress={() => onPin(entry)}
              >
                {t('admin.spotlight.pin.action')}
              </AriaButton>
            ) : (
              <AriaButton
                variant="ghost"
                aria-label={t('admin.spotlight.unpin.label', { title })}
                onPress={() => onChange(entry, { pinnedRank: null })}
              >
                {t('admin.spotlight.unpin.action')}
              </AriaButton>
            )}
            <AriaButton
              variant="ghost"
              aria-label={t(
                entry.hidden
                  ? 'admin.spotlight.entries.showLabel'
                  : 'admin.spotlight.entries.hideLabel',
                { title },
              )}
              onPress={() => onChange(entry, { hidden: !entry.hidden })}
            >
              {t(entry.hidden ? 'admin.spotlight.entries.show' : 'admin.spotlight.entries.hide')}
            </AriaButton>
          </span>
        );
      },
    },
  ];
}

/** What an orphaned override still holds: hidden, pinned, its note. */
export const overrideSummary = (t: TFunction, entry: SpotlightEntry) =>
  [
    entry.hidden ? t('admin.spotlight.entries.state.hidden') : null,
    pinnedText(t, entry),
    entry.note === undefined ? null : quoted(t, entry.note),
  ]
    .filter(Boolean)
    .join(t('admin.spotlight.state.separator'));

/** Overrides whose title left the list (SPO-4): what they hold and «Entfernen». */
export function orphanColumns(
  t: TFunction,
  onRemove: (entry: SpotlightEntry) => void,
): DataColumn<SpotlightEntry>[] {
  return [
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      cell: (entry) => (
        <span className={entry.hidden ? styles.dimmed : ''}>
          <TitleText t={t} entry={entry} />
        </span>
      ),
    },
    {
      key: 'override',
      header: t('admin.spotlight.orphans.override'),
      cell: (entry) => <span className={styles.note}>{overrideSummary(t, entry)}</span>,
    },
    {
      key: 'actions',
      header: t('admin.spotlight.entries.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => (
        <AriaButton
          variant="danger"
          icon="trash"
          aria-label={t('admin.spotlight.orphans.removeLabel', { title: entryName(t, entry) })}
          onPress={() => onRemove(entry)}
        >
          {t('admin.spotlight.orphans.remove')}
        </AriaButton>
      ),
    },
  ];
}
