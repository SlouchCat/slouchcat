import type { TFunction } from 'i18next';
import { WishStateChip } from '@/features/wishes/state';
import { StatusDot } from '@/ui';
import { AriaButton, type DataColumn } from '@/ui/aria';
import { PosterTitle } from '../PosterTitle';
import type { SpotlightEntry } from '../queries';
import styles from './Spotlight.module.css';

/** The name of an entry: its Trakt title, else its TMDB id (an unmatched one its TVDB id). */
export const entryName = (t: TFunction, entry: SpotlightEntry) =>
  entry.title ??
  (entry.tmdbId === undefined
    ? t('admin.spotlight.ids.tvdb', { id: entry.tvdbId ?? t('admin.spotlight.ids.unknown') })
    : t('admin.spotlight.entries.untitled', { id: entry.tmdbId }));

/** An entry the library lacks that TMDB knows: missing, the band «Fehlende Einträge» lists it. */
export const isMissing = (entry: SpotlightEntry) => entry.itemId === undefined && !entry.unmatched;

/**
 * «TMDB 7001 · TVDB – · IMDb tt0123»: the ids Trakt names (N-60), the TVDB id for a series only;
 * an unknown one reads «–».
 */
export function idLine(t: TFunction, entry: SpotlightEntry): string {
  const unknown = t('admin.spotlight.ids.unknown');
  return [
    t('admin.spotlight.ids.tmdb', { id: entry.tmdbId ?? unknown }),
    ...(entry.media === 'show'
      ? [t('admin.spotlight.ids.tvdb', { id: entry.tvdbId ?? unknown })]
      : []),
    t('admin.spotlight.ids.imdb', { id: entry.imdbId ?? unknown }),
  ].join(t('admin.spotlight.state.separator'));
}

/**
 * How the library title was found (N-61): for a series always, for a film only when it was not
 * its TMDB id, the usual way.
 */
const matchText = (t: TFunction, entry: SpotlightEntry) =>
  entry.match && (entry.media === 'show' || entry.match !== 'tmdb')
    ? t(`admin.spotlight.ids.match.${entry.match}`)
    : null;

/** The key of an entry in the tables and in the dialogs' state (an unmatched one by its place). */
export const entryKey = (entry: SpotlightEntry) =>
  `${entry.media}-${entry.tmdbId ?? -entry.position}`;

const pinnedText = (t: TFunction, entry: SpotlightEntry) =>
  entry.pinnedRank === undefined
    ? null
    : t('admin.spotlight.entries.pinned', { rank: entry.pinnedRank });

const quoted = (t: TFunction, note: string) => t('admin.spotlight.entries.quoted', { note });

/** Title and «Film · 2021» in two lines; with `ids` the ids and the way it was matched below. */
function TitleText({
  t,
  entry,
  ids = false,
}: {
  t: TFunction;
  entry: SpotlightEntry;
  ids?: boolean;
}) {
  const matched = ids ? matchText(t, entry) : null;
  return (
    <span className={ids ? `${styles.stack} ${styles.named}` : styles.stack}>
      <span className={styles.title}>{entryName(t, entry)}</span>
      <span className={styles.muted}>
        {entry.year
          ? t('admin.spotlight.entries.meta', {
              media: t(`admin.spotlight.entries.media.${entry.media}`),
              year: entry.year,
            })
          : t(`admin.spotlight.entries.media.${entry.media}`)}
      </span>
      {ids && <span className={styles.ids}>{idLine(t, entry)}</span>}
      {matched && <span className={styles.muted}>{matched}</span>}
    </span>
  );
}

/** «Nicht zuordenbar» with its reason in one line below (N-62). */
function UnmatchedState({ t }: { t: TFunction }) {
  return (
    <span className={styles.stack}>
      <span className={styles.dot}>
        <StatusDot state="warning" />
        {t('admin.spotlight.entries.unmatched')}
      </span>
      <span className={styles.muted}>{t('admin.spotlight.entries.unmatchedReason')}</span>
    </span>
  );
}

/** A missing entry: its wish's state, else «Noch nicht in der Bibliothek». */
function MissingState({ t, entry }: { t: TFunction; entry: SpotlightEntry }) {
  return entry.wish ? (
    <span>
      <WishStateChip state={entry.wish.state} detail={entry.wish.stateDetail} />
    </span>
  ) : (
    <span className={styles.dot}>
      <StatusDot state="warning" />
      {t('admin.spotlight.entries.state.missing')}
    </span>
  );
}

/** «Anfordern» of a missing entry, as the active profile (concept 25, M-27). */
function RequestButton({
  t,
  entry,
  onRequest,
}: {
  t: TFunction;
  entry: SpotlightEntry;
  onRequest: (entry: SpotlightEntry) => void;
}) {
  return (
    <AriaButton
      icon="plus"
      size="sm"
      aria-label={t('admin.spotlight.request.label', { title: entryName(t, entry) })}
      onPress={() => onRequest(entry)}
    >
      {t('admin.spotlight.request.action')}
    </AriaButton>
  );
}

export interface EntryActions {
  onPin: (entry: SpotlightEntry) => void;
  onNote: (entry: SpotlightEntry) => void;
  onChange: (entry: SpotlightEntry, body: { hidden?: boolean; pinnedRank?: null }) => void;
  onRequest: (entry: SpotlightEntry) => void;
}

/**
 * The entries of the list (SPO-3): rank, poster, title and ids, state, note and the actions. A
 * missing entry takes its overrides too (they act once the title is in the library) and
 * «Anfordern» where it can be requested; an unmatched one (N-62) takes nothing and says why. The
 * widths keep the columns in place when an entry is hidden or pinned (K12-9); the note takes the
 * rest.
 */
export function entryColumns(
  t: TFunction,
  { onPin, onNote, onChange, onRequest }: EntryActions,
): DataColumn<SpotlightEntry>[] {
  return [
    {
      key: 'rank',
      width: '5rem',
      header: t('admin.spotlight.entries.columns.rank'),
      align: 'end',
      cell: (entry) => <span className={styles.rank}>{entry.position}</span>,
    },
    {
      key: 'title',
      // Leaves the note column room for «Bearbeiten» at the table's least width (48rem).
      width: '21%',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      // A hidden entry: the title muted, the poster at half strength (mockup Spo-A, Z-5).
      cell: (entry) => (
        <PosterTitle posterUrl={entry.posterUrl} className={entry.hidden ? styles.dimmed : ''}>
          <TitleText t={t} entry={entry} ids />
        </PosterTitle>
      ),
    },
    {
      key: 'status',
      width: '14%',
      header: t('admin.spotlight.entries.columns.status'),
      cell: (entry) => {
        if (entry.unmatched) return <UnmatchedState t={t} />;
        const pinned = pinnedText(t, entry);
        const hidden = (
          <span className={styles.dot}>
            <StatusDot state="pending" />
            {t('admin.spotlight.entries.state.hidden')}
          </span>
        );
        // A missing entry says so first; hidden is its second line then.
        return (
          <span className={styles.stack}>
            {entry.itemId === undefined && <MissingState t={t} entry={entry} />}
            {entry.hidden && hidden}
            {entry.itemId !== undefined && !entry.hidden && (
              <span className={styles.dot}>
                <StatusDot state="success" />
                {t('admin.spotlight.entries.state.visible')}
              </span>
            )}
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
          {entry.canOverride && (
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
      // Both buttons in full whichever words they carry (Anheften/Lösen, Ausblenden/Einblenden).
      width: '18.5rem',
      header: t('admin.spotlight.entries.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => {
        if (!entry.canOverride) {
          return <span className={styles.muted}>{t('admin.spotlight.entries.noAction')}</span>;
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
            {entry.canRequest && <RequestButton t={t} entry={entry} onRequest={onRequest} />}
          </span>
        );
      },
    },
  ];
}

/**
 * The entries the library lacks (band «Fehlende Einträge», N-71): poster, title and ids, the state
 * of the title's wish and «Anfordern» where it can be requested. Display only, nothing to set.
 */
export function missingColumns(
  t: TFunction,
  onRequest: (entry: SpotlightEntry) => void,
): DataColumn<SpotlightEntry>[] {
  return [
    {
      key: 'rank',
      width: '5rem',
      header: t('admin.spotlight.entries.columns.rank'),
      align: 'end',
      cell: (entry) => <span className={styles.rank}>{entry.position}</span>,
    },
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      cell: (entry) => (
        <PosterTitle posterUrl={entry.posterUrl}>
          <TitleText t={t} entry={entry} ids />
        </PosterTitle>
      ),
    },
    {
      key: 'status',
      header: t('admin.spotlight.entries.columns.status'),
      cell: (entry) => <MissingState t={t} entry={entry} />,
    },
    {
      key: 'actions',
      header: t('admin.spotlight.entries.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) =>
        entry.canRequest ? <RequestButton t={t} entry={entry} onRequest={onRequest} /> : null,
    },
  ];
}

/** The entries TMDB could not resolve (N-62), in the band «Abgleich über TMDB, TVDB und IMDb». */
export function unmatchedColumns(t: TFunction): DataColumn<SpotlightEntry>[] {
  return [
    {
      key: 'rank',
      width: '5rem',
      header: t('admin.spotlight.entries.columns.rank'),
      align: 'end',
      cell: (entry) => <span className={styles.rank}>{entry.position}</span>,
    },
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      cell: (entry) => <TitleText t={t} entry={entry} ids />,
    },
    {
      key: 'status',
      header: t('admin.spotlight.entries.columns.status'),
      cell: () => <UnmatchedState t={t} />,
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
          variant="dangerQuiet"
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
