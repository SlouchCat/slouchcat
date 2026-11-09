import type {
  LibraryTrickplay,
  LibraryTrickplayEstimate,
  LibraryTrickplayKindEstimate,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatBytes } from '@/lib/format';
import { RadioGroup, Switch } from '@/ui/aria';
import styles from './Libraries.module.css';

/**
 * What a new video library starts with: both kinds «Aus» (since 1.23.1, before «Im Voraus» by
 * F-11), «Speichern» and the whole series kept for a later change of the mode.
 */
export const DEFAULT_TRICKPLAY: LibraryTrickplay = {
  movies: { mode: 'off', keep: true },
  series: { mode: 'off', keep: true, scope: 'series' },
};

type Kind = keyof LibraryTrickplay;
type Mode = LibraryTrickplay['series']['mode'];
type Scope = LibraryTrickplay['series']['scope'];

/** The choices in the order of the dialog: «Aus» first (1.23.1). */
const MOVIE_MODES = ['off', 'ahead', 'on_start'] as const;
const SERIES_MODES = ['off', 'ahead', 'on_start', 'series_start'] as const;
const SCOPES = ['series', 'season_rest', 'next_episode'] as const;

/** Whether a setting keeps its previews (6.3): `ahead`, `series_start`, `on_start` with «Speichern». */
export const trickplayStores = (setting: { mode: Mode; keep: boolean }): boolean =>
  setting.mode === 'ahead' ||
  setting.mode === 'series_start' ||
  (setting.mode === 'on_start' && setting.keep);

/** Whether two settings ask the server for the same thing (the draft differs or not). */
export function sameTrickplay(a: LibraryTrickplay, b: LibraryTrickplay): boolean {
  return (
    a.movies.mode === b.movies.mode &&
    a.movies.keep === b.movies.keep &&
    a.series.mode === b.series.mode &&
    a.series.keep === b.series.keep &&
    a.series.scope === b.series.scope
  );
}

/** The sentence under a group: when the previews come and what they cost (concept 18, 9). */
function factText(t: TFunction, kind: Kind, setting: { mode: Mode; keep: boolean }, scope: Scope) {
  const noun = kind === 'movies' ? 'movie' : 'episode';
  switch (setting.mode) {
    case 'ahead':
      return t(`admin.libraries.trickplay.facts.ahead.${noun}`);
    case 'on_start':
      return setting.keep
        ? t('admin.libraries.trickplay.facts.onStartKeep')
        : t('admin.libraries.trickplay.facts.onStart');
    case 'series_start':
      return t(`admin.libraries.trickplay.facts.seriesStart.${scope}`);
    case 'off':
      return t('admin.libraries.trickplay.facts.off');
  }
}

interface EstimateRow {
  label: string;
  value: string;
  total?: boolean;
}

/**
 * The size of one title: «1,5 MB» below 10 MB, because `formatBytes` rounds megabytes to whole
 * numbers and a title of 1.5 MB would read «2 MB».
 */
function formatBytesSmall(bytes: number, language: string): string {
  if (bytes < 1_000_000 || bytes >= 10_000_000) return formatBytes(bytes, language);
  return new Intl.NumberFormat(language, {
    style: 'unit',
    unit: 'megabyte',
    unitDisplay: 'short',
    maximumFractionDigits: 1,
  }).format(bytes / 1_000_000);
}

/**
 * The estimate of one kind for its setting (concept 18, 9): per film or episode and the whole
 * library; «höchstens» where only started titles get one, «nur während der Wiedergabe» and 0 for
 * the library without «Speichern», 0 for «Aus».
 */
export function estimateRows(
  t: TFunction,
  language: string,
  kind: Kind,
  setting: { mode: Mode; keep: boolean },
  estimate: LibraryTrickplayKindEstimate,
): EstimateRow[] {
  const size = (bytes: number, oneDecimal = false) =>
    bytes === 0
      ? t('admin.libraries.trickplay.estimate.zero')
      : t('admin.libraries.trickplay.estimate.approx', {
          size: oneDecimal ? formatBytesSmall(bytes, language) : formatBytes(bytes, language),
        });
  const per = kind === 'movies' ? 'perMovie' : 'perEpisode';
  const library = (bytes: number, atMost = false): EstimateRow => ({
    label: t(`admin.libraries.trickplay.estimate.${atMost ? 'libraryAtMost' : 'library'}`),
    value: size(bytes),
    total: true,
  });
  if (setting.mode === 'off') {
    return [{ label: t(`admin.libraries.trickplay.estimate.${per}`), value: size(0) }, library(0)];
  }
  if (setting.mode === 'on_start' && !setting.keep) {
    return [
      {
        label: t(`admin.libraries.trickplay.estimate.${per}Playing`),
        value: size(estimate.perTitleBytes, true),
      },
      library(0),
    ];
  }
  return [
    {
      label: t(`admin.libraries.trickplay.estimate.${per}`),
      value: size(estimate.perTitleBytes, true),
    },
    library(estimate.libraryBytes, setting.mode !== 'ahead'),
  ];
}

interface TrickplayFieldsProps {
  value: LibraryTrickplay;
  onChange: (value: LibraryTrickplay) => void;
  /** `getLibrary`'s estimate; absent when adding a library (nothing to count yet). */
  estimate?: LibraryTrickplayEstimate;
}

/**
 * «Vorschau beim Spulen» of a video library (concept 18, 6.1 and 9): one group for films and one
 * for series, each with its choice, «Speichern» at «Beim Start», the scope at «Beim Start alle
 * Folgen», a sentence about the chosen setting and, when the library exists, the estimate.
 */
export function TrickplayFields({ value, onChange, estimate }: TrickplayFieldsProps) {
  const { t, i18n } = useTranslation();
  const hours = (ms: number) =>
    new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 0 }).format(ms / 3_600_000);
  const modeOptions = (modes: readonly Mode[]) =>
    modes.map((mode) => ({ value: mode, label: t(`admin.libraries.trickplay.mode.${mode}`) }));

  const group = (kind: Kind) => {
    const setting = value[kind];
    const kindEstimate = estimate?.[kind];
    const rows = kindEstimate
      ? estimateRows(t, i18n.language, kind, setting, kindEstimate)
      : undefined;
    return (
      <div className={styles.trickplayGroup} data-trickplay-group={kind}>
        <RadioGroup
          name={`trickplay.${kind}.mode`}
          label={t(`admin.libraries.trickplay.${kind}`)}
          orientation="vertical"
          value={setting.mode}
          onChange={(mode) =>
            onChange(
              kind === 'movies'
                ? { ...value, movies: { ...value.movies, mode: mode as typeof value.movies.mode } }
                : { ...value, series: { ...value.series, mode: mode as Mode } },
            )
          }
          options={modeOptions(kind === 'movies' ? MOVIE_MODES : SERIES_MODES)}
        />
        {setting.mode === 'on_start' && (
          <div className={styles.trickplayKeep}>
            <Switch
              isSelected={setting.keep}
              onChange={(keep) =>
                onChange(
                  kind === 'movies'
                    ? { ...value, movies: { ...value.movies, keep } }
                    : { ...value, series: { ...value.series, keep } },
                )
              }
            >
              {t('admin.libraries.trickplay.keep')}
            </Switch>
            <span className={styles.muted}>{t('admin.libraries.trickplay.keepHint')}</span>
          </div>
        )}
        {kind === 'series' && setting.mode === 'series_start' && (
          <RadioGroup
            name="trickplay.series.scope"
            label={t('admin.libraries.trickplay.scope.label')}
            orientation="vertical"
            value={value.series.scope}
            onChange={(scope) =>
              onChange({ ...value, series: { ...value.series, scope: scope as Scope } })
            }
            options={SCOPES.map((scope) => ({
              value: scope,
              label: t(`admin.libraries.trickplay.scope.${scope}`),
            }))}
          />
        )}
        <p className={styles.muted} data-trickplay-facts="">
          {factText(t, kind, setting, value.series.scope)}
        </p>
        {kindEstimate && rows && (
          <div className={styles.trickplayEstimate}>
            <span className={styles.muted}>
              {t(`admin.libraries.trickplay.estimate.counts.${kind}`, {
                count: kindEstimate.titles,
                hours: hours(kindEstimate.durationMs),
              })}
            </span>
            <dl className={styles.trickplayRows}>
              {rows.map((row) => (
                <div key={row.label} data-total={row.total || undefined}>
                  <dt>{row.label}</dt>
                  <dd>{row.value}</dd>
                </div>
              ))}
            </dl>
          </div>
        )}
      </div>
    );
  };

  const storedSize = (bytes: number) =>
    bytes === 0 ? t('admin.libraries.trickplay.estimate.zero') : formatBytes(bytes, i18n.language);

  return (
    <div className={styles.trickplay}>
      {group('movies')}
      {group('series')}
      {estimate && (
        <div className={styles.trickplayFooter}>
          <span className={styles.muted}>{t('admin.libraries.trickplay.estimate.basis')}</span>
          <span className={styles.muted}>
            {t('admin.libraries.trickplay.estimate.stored', {
              movies: storedSize(estimate.movies.storedBytes),
              series: storedSize(estimate.series.storedBytes),
            })}
          </span>
          {estimate.cacheFreeBytes !== undefined && (
            <span className={styles.muted}>
              {t('admin.libraries.trickplay.estimate.free', {
                size: formatBytes(estimate.cacheFreeBytes, i18n.language),
              })}
            </span>
          )}
        </div>
      )}
    </div>
  );
}
