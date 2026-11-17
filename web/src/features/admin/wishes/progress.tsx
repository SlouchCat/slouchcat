import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Chip } from '@/ui';
import type { AdminWishProgress, WishRequesterTag } from '../queries';

// What Radarr and Sonarr say about a wish in the administration (concept 25 stage 4, M-23): the
// download progress in one line, the profile tag as a chip, and the class of an error in words.
// The message of the service stands only in the fold-out (N-83).

type Service = AdminWishProgress['service'];

/** «Radarr», «Sonarr». */
export const serviceName = (t: TFunction, service: Service) =>
  t(`admin.integrations.webhooks.kind.${service}`);

/** «noch 12 min», «noch 2 h 5 min». */
function etaText(t: TFunction, seconds: number): string {
  const minutes = Math.max(1, Math.ceil(seconds / 60));
  if (minutes < 60) return t('admin.wishes.requests.progress.etaMinutes', { minutes });
  return t('admin.wishes.requests.progress.etaHours', {
    hours: Math.floor(minutes / 60),
    minutes: minutes % 60,
  });
}

/** The progress can be shown: read recently and the service answered. */
const readable = (progress: AdminWishProgress) =>
  !progress.stale && progress.error !== 'unreachable';

/**
 * The progress in one line: «Wird heruntergeladen · 45 % · noch 12 min», «Download verzögert»,
 * «8 von 10 Folgen da» added for a series; «Fortschritt nicht lesbar» with the time of the last read
 * when the read is stale or the service did not answer; empty when nothing is known.
 */
export function progressText(
  t: TFunction,
  progress: AdminWishProgress | undefined,
  language: string,
): string | undefined {
  if (!progress) return undefined;
  const separator = t('format.separator');
  if (!readable(progress)) {
    return progress.readAt
      ? t('admin.wishes.requests.progress.unreadableAt', {
          time: formatDateTime(progress.readAt, language),
        })
      : t('admin.wishes.requests.progress.unreadable');
  }
  if (!progress.stage) {
    return progress.error === 'not_found'
      ? t('admin.wishes.requests.progress.notInService', {
          service: serviceName(t, progress.service),
        })
      : undefined;
  }
  const parts = [t(`admin.wishes.requests.progress.${progress.stage}`)];
  if (progress.stage === 'download') {
    if (progress.percent !== undefined) {
      parts.push(t('admin.wishes.requests.progress.percent', { percent: progress.percent }));
    }
    if (progress.etaSeconds !== undefined) parts.push(etaText(t, progress.etaSeconds));
  }
  if (progress.episodes) {
    parts.push(t('admin.wishes.requests.progress.episodes', progress.episodes));
  }
  return parts.join(separator);
}

/** Whether a wish's download is stalled by a read that is fresh. */
export const isStalled = (progress: AdminWishProgress | undefined) =>
  progress?.stage === 'stalled' && readable(progress);

const tagTone = {
  set: 'success',
  pending: 'outline',
  failed: 'danger',
} as const satisfies Record<WishRequesterTag['state'], string>;

/** «sc-noah», «sc-noah · ausstehend», «sc-noah · fehlgeschlagen». */
export function TagChip({ tag }: { tag: WishRequesterTag }) {
  const { t } = useTranslation();
  return (
    <Chip tone={tagTone[tag.state]}>
      {t(`admin.wishes.requests.tags.${tag.state}`, { name: tag.name })}
    </Chip>
  );
}

/** The error classes the server names (arrErrorClass, the tag job's own two). */
const ERROR_CLASSES = [
  'not_in_arr',
  'tvdb_id_missing',
  'not_confirmed',
  'not_in_seerr',
  'seerr_off',
  'redirect',
  'timeout',
  'unreachable',
  'key_invalid',
  'not_found',
  'validation',
  'server',
  'bad_request',
  'invalid',
] as const;

const isErrorClass = (error: string): error is (typeof ERROR_CLASSES)[number] =>
  ERROR_CLASSES.some((known) => known === error);

/** The class of an error of Radarr or Sonarr in words; never an address or a key. */
export function arrErrorText(t: TFunction, error: string, service: Service): string {
  const name = serviceName(t, service);
  if (isErrorClass(error)) return t(`admin.wishes.arrError.${error}`, { service: name });
  if (error.startsWith('seerr_')) return t('admin.wishes.arrError.seerr', { service: name });
  return t('admin.wishes.arrError.other', { error });
}
