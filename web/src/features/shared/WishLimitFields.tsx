import type { components } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { RadioGroup, TextField } from '@/ui/aria';
import styles from './WishLimitFields.module.css';

type WishLimits = components['schemas']['WishLimits'];
type WishLimitRequest = components['schemas']['AccountWishLimitRequest'];
export type WishLimitMode = WishLimitRequest['mode'];

/** The modes of a wish limit, in the order the form offers them. */
const MODES = ['default', 'custom', 'unlimited'] as const satisfies readonly WishLimitMode[];

/** The bounds of the contract (`WishLimit`): a count 0 to 100 (0 = no limit), days 1 to 90. */
const BOUNDS = {
  movieLimit: [0, 100],
  movieDays: [1, 90],
  tvLimit: [0, 100],
  tvDays: [1, 90],
} as const;

/** The four numbers of a wish limit as typed: texts, checked against the contract's bounds. */
export interface WishLimitNumbers {
  movieLimit: string;
  movieDays: string;
  tvLimit: string;
  tvDays: string;
}

/** A wish limit as a form holds it: the mode and, for «Eigenes», the numbers. */
export interface WishLimitDraft extends WishLimitNumbers {
  mode: WishLimitMode;
}

export const wishNumbersOf = (limits: WishLimits): WishLimitNumbers => ({
  movieLimit: String(limits.movie.limit),
  movieDays: String(limits.movie.days),
  tvLimit: String(limits.tv.limit),
  tvDays: String(limits.tv.days),
});

/**
 * The draft of a stored limit; the numbers of «Eigenes» start from the account's own ones, else
 * from the default (so switching to «Eigenes» shows something to change).
 */
export function wishLimitDraftOf(
  limit: { mode: WishLimitMode; movie?: WishLimits['movie']; tv?: WishLimits['tv'] } | undefined,
  defaults: WishLimits,
): WishLimitDraft {
  const own = limit?.movie && limit.tv ? { movie: limit.movie, tv: limit.tv } : defaults;
  return { mode: limit?.mode ?? 'default', ...wishNumbersOf(own) };
}

const within = (text: string, [low, high]: readonly [number, number]) =>
  /^\d{1,3}$/.test(text.trim()) && Number(text) >= low && Number(text) <= high;

/** All four numbers lie within the contract's bounds. */
export const wishNumbersValid = (value: WishLimitNumbers) =>
  (Object.keys(BOUNDS) as (keyof WishLimitNumbers)[]).every((key) =>
    within(value[key], BOUNDS[key]),
  );

export const wishLimitsOf = (value: WishLimitNumbers): WishLimits => ({
  movie: { limit: Number(value.movieLimit), days: Number(value.movieDays) },
  tv: { limit: Number(value.tvLimit), days: Number(value.tvDays) },
});

/**
 * The server's names of the four numbers in a 422 (`<server>movie.limit`, …) as the names of the
 * fields (`<name>.movieLimit`, …), for `formErrors`: so a refused number stands at its field.
 */
export const wishLimitFieldNames = (server: string, name: string): Record<string, string> => ({
  [`${server}movie.limit`]: `${name}.movieLimit`,
  [`${server}movie.days`]: `${name}.movieDays`,
  [`${server}tv.limit`]: `${name}.tvLimit`,
  [`${server}tv.days`]: `${name}.tvDays`,
});

/** The request of a draft: the numbers only with «Eigenes». */
export const wishLimitRequestOf = (draft: WishLimitDraft): WishLimitRequest =>
  draft.mode === 'custom' ? { mode: 'custom', ...wishLimitsOf(draft) } : { mode: draft.mode };

/** «10 Filme / 7 Tage» or «5 Staffeln / 7 Tage»; a limit of 0 is «ohne Limit». */
export function limitPart(t: TFunction, limits: WishLimits, kind: 'movie' | 'tv'): string {
  const { limit, days } = limits[kind];
  return limit === 0
    ? t(`admin.wishes.limit.${kind}None`)
    : t(`admin.wishes.limit.${kind}`, { count: limit, days });
}

/** «10 Filme / 7 Tage · 5 Staffeln / 7 Tage». */
export const limitSummary = (t: TFunction, limits: WishLimits): string =>
  `${limitPart(t, limits, 'movie')}${t('admin.wishes.separator')}${limitPart(t, limits, 'tv')}`;

/**
 * The four numbers of a wish limit (N-31): films and seasons, each a count and a period side by
 * side, each with its bounds below (as LimitFields) and the reason under a wrong one. Native
 * validation as everywhere in the app: a wrong number stops the form's submit and takes the
 * focus. `onBlur` lets a form save when a field is left.
 */
export function WishLimitNumberFields({
  value,
  onChange,
  onBlur,
  name = 'wishLimit',
}: {
  value: WishLimitNumbers;
  onChange: (value: WishLimitNumbers) => void;
  onBlur?: () => void;
  /** The prefix of the fields' names (two forms on one page stay apart). */
  name?: string;
}) {
  const { t } = useTranslation();
  const field = (key: keyof WishLimitNumbers) => {
    const [low, high] = BOUNDS[key];
    return (
      <TextField
        name={`${name}.${key}`}
        label={t(`admin.wishes.settings.${key}`)}
        inputMode="numeric"
        autoComplete="off"
        value={value[key]}
        onChange={(text) => onChange({ ...value, [key]: text })}
        onBlur={onBlur}
        description={t(
          key.endsWith('Days')
            ? 'admin.wishes.settings.limitHintDays'
            : 'admin.wishes.settings.limitHintCount',
          { low, high },
        )}
        validate={(text) =>
          within(text, BOUNDS[key]) ? null : t('admin.wishes.settings.limitRange', { low, high })
        }
      />
    );
  };
  return (
    <div className={styles.limits}>
      <div className={styles.row}>
        {field('movieLimit')}
        {field('movieDays')}
      </div>
      <div className={styles.row}>
        {field('tvLimit')}
        {field('tvDays')}
      </div>
    </div>
  );
}

/**
 * The wish limit of an account or an invitation (concept 25, administration 8): «Standard» (the
 * default of the wish settings, named), «Eigenes» (the four numbers) or «Unbegrenzt». `locked`
 * replaces the choice with its reason (a Seerr administrator has no limit, N-34).
 */
export function WishLimitFields({
  value,
  onChange,
  defaults,
  hint,
  locked,
  name = 'wishLimit',
}: {
  value: WishLimitDraft;
  onChange: (value: WishLimitDraft) => void;
  /** The default limit of the wish settings, to name «Standard»; unknown while it loads. */
  defaults: WishLimits | undefined;
  /** A line under the choice (what happens without a Seerr user). */
  hint?: ReactNode;
  locked?: ReactNode;
  name?: string;
}) {
  const { t } = useTranslation();
  if (locked) {
    return (
      <div className={styles.limits}>
        <Text>{t('admin.wishes.users.noLimitAdmin')}</Text>
        {locked}
      </div>
    );
  }
  return (
    <div className={styles.limits}>
      <RadioGroup
        name={`${name}.mode`}
        label={t('admin.wishes.limitField.label')}
        value={value.mode}
        onChange={(mode) => {
          const next = MODES.find((candidate) => candidate === mode);
          if (next) onChange({ ...value, mode: next });
        }}
        description={
          value.mode === 'default'
            ? defaults
              ? t('admin.wishes.limitField.help.default', { limit: limitSummary(t, defaults) })
              : undefined
            : t(`admin.wishes.limitField.help.${value.mode}`)
        }
        options={MODES.map((mode) => ({
          value: mode,
          label: t(`admin.wishes.users.mode.${mode}`),
        }))}
      />
      {value.mode === 'custom' && (
        <WishLimitNumberFields
          value={value}
          onChange={(numbers) => onChange({ ...value, ...numbers })}
          name={name}
        />
      )}
      {hint}
    </div>
  );
}
