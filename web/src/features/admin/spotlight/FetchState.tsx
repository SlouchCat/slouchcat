import type { TraktStatus } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { codeMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, Skeleton, StatusDot, TextLink, type StatusDotState } from '@/ui';
import type { SpotlightConfig, SpotlightEntries } from '../queries';
import styles from './Spotlight.module.css';

/** Whether the row reaches the start page, and if not, why (SPO-1a, plan 7.1). */
export interface RowState {
  /** The sentence why the row is missing; null when it is shown. */
  missing: string | null;
  /** The key is missing or the last fetch failed: the danger tone, not the warning one. */
  broken: boolean;
  /** Library hits among the fetched entries; undefined until the entries are there. */
  hits: number | undefined;
}

export function rowState(
  t: TFunction,
  status: TraktStatus,
  config: SpotlightConfig,
  entries: SpotlightEntries | undefined,
): RowState {
  // Hits are system-wide (plan 5.5): a profile may see fewer of them and still lose the row.
  const hits = entries?.entries.filter((entry) => entry.itemId !== undefined).length;
  const missing = (() => {
    if (!status.configured) return t('admin.spotlight.missing.noKey');
    if (!config.enabled) return t('admin.spotlight.missing.disabled');
    if (status.lastError) return t('admin.spotlight.missing.failed');
    if (status.lastSuccess && status.entries === 0) return t('admin.spotlight.missing.empty');
    if (hits !== undefined && hits < config.minHits) {
      return t('admin.spotlight.missing.belowMinHits', { hits, min: config.minHits });
    }
    return null;
  })();
  return { missing, broken: !status.configured || Boolean(status.lastError), hits };
}

/** The band's state: «Auf der Startseite» or «Fehlt auf der Startseite». */
export function RowChip({ state }: { state: RowState }) {
  const { t } = useTranslation();
  if (!state.missing) return <Chip tone="success">{t('admin.spotlight.row.shown')}</Chip>;
  return <Chip tone={state.broken ? 'danger' : 'warning'}>{t('admin.spotlight.row.missing')}</Chip>;
}

/**
 * The source's last lines (mockup Spo-A): when the list was fetched or why the last fetch failed,
 * how many of its titles are in the library, whether the server has a Trakt key (a link to the
 * integrations page, never the key), and why the row is missing from the start page now. The
 * last sentence is a warning that stays visible (S-2).
 */
export function FetchState({
  status,
  statusFailed,
  config,
  entries,
}: {
  status?: TraktStatus;
  /** The state could not be loaded: the page shows the error with «Erneut versuchen». */
  statusFailed: boolean;
  config: SpotlightConfig;
  entries?: SpotlightEntries;
}) {
  const { t, i18n } = useTranslation();
  if (!status) return statusFailed ? <div className={styles.state} /> : <Skeleton shape="line" />;
  // `trakt_unavailable: HTTP 503`: the code in words when the UI knows it, else the server's text.
  const reason = (text: string) => {
    const [code = ''] = text.split(':');
    return codeMessage(t, code.trim()) ?? text;
  };
  const state = rowState(t, status, config, entries);
  const dot: StatusDotState = state.broken ? 'danger' : state.missing ? 'warning' : 'success';
  const fetched = status.lastError
    ? t('admin.spotlight.state.failed', { reason: reason(status.lastError) })
    : status.lastSuccess
      ? t('admin.spotlight.state.fetched', {
          date: formatDateTime(status.lastSuccess, i18n.language),
        })
      : t('admin.spotlight.state.never');
  const separator = <span aria-hidden="true">{t('admin.spotlight.state.separator')}</span>;

  return (
    <div className={styles.state}>
      <p className={styles.stateLine}>
        <StatusDot state={dot} />
        <span>{fetched}</span>
        {state.hits !== undefined && (
          <>
            {separator}
            <span>
              {t('admin.spotlight.state.hits', { hits: state.hits, entries: status.entries })}
            </span>
          </>
        )}
        {separator}
        <TextLink href={paths.adminIntegrations}>
          {status.configured ? t('admin.spotlight.state.keySet') : t('admin.spotlight.state.noKey')}
        </TextLink>
      </p>
      {state.missing && (
        <Notice tone={state.broken ? 'danger' : 'warning'} live={false}>
          {state.missing}
        </Notice>
      )}
    </div>
  );
}
