import type { AdminUserPick } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { AriaButton, type DataColumn } from '@/ui/aria';
import { titleMeta } from '../CurationParts';
import { PosterTitle } from '../PosterTitle';
import styles from './Picks.module.css';

/**
 * The day of a pick as the server names it (`YYYY-MM-DD` in its zone): read as a calendar day,
 * so no time zone of the browser moves it to the day before.
 */
export function pickDay(day: string, language: string): string {
  return new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeZone: 'UTC' }).format(
    new Date(`${day}T00:00:00Z`),
  );
}

/** «läuft ab in 12 Tagen», «heute». */
export function expiresIn(t: TFunction, days: number): string {
  return days <= 0 ? t('admin.picks.expiresToday') : t('admin.picks.expiresIn', { count: days });
}

/**
 * The columns of the user picks (concept 18 part B, 14.7, mockup «EmpfehlungenAdmin»): the title
 * with its poster, the day of the newest pick, the days left, the number of picks and
 * «Entfernen». Anonymous: no column names a profile.
 */
export function pickColumns(
  t: TFunction,
  language: string,
  onRemove: (pick: AdminUserPick) => void,
): DataColumn<AdminUserPick>[] {
  return [
    {
      key: 'title',
      width: '40%',
      header: t('admin.picks.columns.title'),
      rowHeader: true,
      cell: (pick) => (
        <PosterTitle posterUrl={pick.posterUrl}>
          <span className={styles.stack}>
            <span>{pick.title}</span>
            <span className={styles.muted}>{titleMeta(t, pick)}</span>
          </span>
        </PosterTitle>
      ),
    },
    {
      key: 'picked',
      header: t('admin.picks.columns.picked'),
      cell: (pick) => pickDay(pick.lastPickedOn, language),
    },
    {
      key: 'expires',
      header: t('admin.picks.columns.expires'),
      cell: (pick) => expiresIn(t, pick.expiresInDays),
    },
    {
      key: 'count',
      width: '7rem',
      header: t('admin.picks.columns.count'),
      align: 'end',
      cell: (pick) => (
        <span className={styles.numbers}>{t('admin.picks.countValue', { count: pick.count })}</span>
      ),
    },
    {
      key: 'actions',
      width: '9rem',
      header: t('admin.picks.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (pick) => (
        <AriaButton
          variant="dangerQuiet"
          icon="trash"
          aria-label={t('admin.picks.remove.label', { title: pick.title })}
          onPress={() => onRemove(pick)}
        >
          {t('admin.picks.remove.action')}
        </AriaButton>
      ),
    },
  ];
}
