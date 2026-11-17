import type { components } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { intlLocale } from '@/i18n';
import { formatDate } from '@/lib/format';
import { limitPart } from './WishLimitFields';

// The words of a wish limit (concept 25, 4.10), the same in the administration (Seerr-Benutzer)
// and for the viewer (the request dialog, «Meine Wünsche»).

type WishLimits = components['schemas']['WishLimits'];
type WishQuota = components['schemas']['WishQuota'];
type WishQuotaPart = components['schemas']['WishQuotaPart'];
type Kind = 'movie' | 'tv';

/**
 * One half of a limit state as Seerr last reported it: «Filme: 7 von 10 in 7 Tagen · noch 3 frei
 * · nächster Platz frei am 20.11.2026». Without a state from Seerr the limit itself (`limits`, the
 * administration's); a half with limit 0 is absent from the state and reads «ohne Limit».
 */
export function quotaText(
  t: TFunction,
  language: string,
  kind: Kind,
  quota: WishQuota | undefined,
  limits?: WishLimits,
): string {
  if (quota === undefined) return limits ? limitPart(t, limits, kind) : '';
  const part: WishQuotaPart | undefined = quota[kind];
  if (part === undefined) return t(`wishes.quota.${kind}None`);
  const parts = [
    t(`wishes.quota.${kind}`, { used: part.used, limit: part.limit, days: part.days }),
    t('wishes.quota.remaining', { count: part.remaining }),
  ];
  if (part.nextFreeAt) {
    parts.push(t('wishes.quota.nextFree', { date: formatDate(part.nextFreeAt, language) }));
  }
  return parts.join(t('format.separator'));
}

/**
 * «Filme 7/10», «Staffeln 2/5»: the table's cell, one half per line; empty without a state or
 * without a limit.
 */
export function quotaShort(t: TFunction, quota: WishQuota | undefined): string[] {
  if (quota === undefined || quota.unlimited) return [];
  return (['movie', 'tv'] as const).flatMap((kind) => {
    const part = quota[kind];
    return part ? [t(`wishes.quota.short.${kind}`, { used: part.used, limit: part.limit })] : [];
  });
}

/** The two halves of a limit state, one line each (or one line with `separator`). */
export function QuotaLines({
  quota,
  limits,
  separator = false,
}: {
  quota: WishQuota | undefined;
  limits?: WishLimits;
  separator?: boolean;
}) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const movie = quotaText(t, language, 'movie', quota, limits);
  const tv = quotaText(t, language, 'tv', quota, limits);
  if (separator) return <>{`${movie}${t('format.separator')}${tv}`}</>;
  return (
    <>
      <span>{movie}</span>
      <span>{tv}</span>
    </>
  );
}
