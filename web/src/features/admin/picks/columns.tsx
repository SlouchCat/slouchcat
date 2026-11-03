import type { AdminPick } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatDateTime } from '@/lib/format';
import { Chip, StatusDot, VisuallyHidden, type StatusDotState } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import { titleMeta } from '../CurationParts';
import { PosterTitle } from '../PosterTitle';
import styles from './Picks.module.css';

/**
 * Where a pick stands in time (the filter tabs, PIC-2): `expired` once its end has passed,
 * `scheduled` while its start lies ahead, `active` otherwise. Only the time window counts: whether
 * a profile may see the title is the server's decision (rule 12), so «Aktiv» is not «sichtbar».
 */
export type PickPhase = 'active' | 'scheduled' | 'expired';

export function phaseOf(pick: AdminPick, now: number): PickPhase {
  if (pick.endsAt && Date.parse(pick.endsAt) <= now) return 'expired';
  if (pick.startsAt && Date.parse(pick.startsAt) > now) return 'scheduled';
  return 'active';
}

/** «01.12.2026, 08:00 bis 26.12.2026, 23:59», «Ab …», «Bis …» or «Immer». */
export function timeWindow(t: TFunction, language: string, pick: AdminPick): string {
  const date = (iso: string) => formatDateTime(iso, language);
  if (pick.startsAt && pick.endsAt) {
    return t('admin.picks.window.range', { from: date(pick.startsAt), to: date(pick.endsAt) });
  }
  if (pick.startsAt) return t('admin.picks.window.from', { date: date(pick.startsAt) });
  if (pick.endsAt) return t('admin.picks.window.until', { date: date(pick.endsAt) });
  return t('admin.picks.window.always');
}

/**
 * The dot of the window and the word for the two states in which the row is missing on the
 * start page: expired, scheduled; a limited window that runs now has the dot only.
 */
function windowState(
  t: TFunction,
  pick: AdminPick,
  now: number,
): { dot: StatusDotState; word?: string } | null {
  const phase = phaseOf(pick, now);
  if (phase === 'expired') return { dot: 'warning', word: t('admin.picks.window.expired') };
  if (phase === 'scheduled') return { dot: 'pending', word: t('admin.picks.window.scheduled') };
  if (pick.endsAt) return { dot: 'success' };
  return null;
}

/**
 * The columns of the picks table (PIC-2, mockup Pic-C): the place in the row, the title with its
 * poster (it folds the row out, «geändert» while the pick has unsaved changes), the target, the
 * note in full and the time window. `place` is the 1-based place in the whole row, also in a
 * filtered tab; the list comes in carousel order, whose `position` may have gaps. The widths keep
 * the columns in place when «geändert» appears and across the tabs (K12-9); the note takes the rest.
 */
export function pickColumns(
  t: TFunction,
  language: string,
  now: number,
  place: (pick: AdminPick) => number,
  isChanged: (pick: AdminPick) => boolean,
): DataColumn<AdminPick>[] {
  return [
    {
      key: 'position',
      width: '4rem',
      header: t('admin.picks.columns.position'),
      align: 'end',
      cell: (pick) => <span className={styles.numbers}>{place(pick)}</span>,
    },
    {
      key: 'title',
      width: '34%',
      header: t('admin.picks.columns.title'),
      rowHeader: true,
      cell: (pick) => (
        <PosterTitle posterUrl={pick.posterUrl}>
          <span className={styles.stack}>
            <span className={styles.title}>
              <span>{pick.title}</span>
              {isChanged(pick) && <Chip tone="warning">{t('admin.picks.changed')}</Chip>}
            </span>
            <span className={styles.muted}>{titleMeta(t, pick)}</span>
          </span>
        </PosterTitle>
      ),
    },
    {
      key: 'target',
      width: '14%',
      header: t('admin.picks.columns.target'),
      cell: (pick) => pick.profileName ?? t('admin.picks.allProfiles'),
    },
    {
      key: 'note',
      header: t('admin.picks.columns.note'),
      // In full, wrapped: a note has at most 280 characters, and no hover is needed to read it.
      cell: (pick) =>
        pick.note === undefined ? (
          <>
            <span aria-hidden="true">{t('admin.picks.noNote')}</span>
            <VisuallyHidden>{t('admin.picks.noNoteSpoken')}</VisuallyHidden>
          </>
        ) : (
          <span className={styles.note}>{t('admin.picks.quoted', { note: pick.note })}</span>
        ),
    },
    {
      key: 'window',
      width: '20%',
      header: t('admin.picks.columns.window'),
      cell: (pick) => {
        const state = windowState(t, pick, now);
        return (
          <span className={styles.stack}>
            <span className={styles.dot}>
              {state && <StatusDot state={state.dot} />}
              {timeWindow(t, language, pick)}
            </span>
            {state?.word && <span className={styles.muted}>{state.word}</span>}
          </span>
        );
      },
    },
  ];
}
