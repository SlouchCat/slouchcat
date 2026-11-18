import { useTranslation } from 'react-i18next';
import { formatDuration } from '@/lib/format';
import { Icon, ProgressBar } from '@/ui';
import type { WishProgress } from './queries';
import styles from './Wishes.module.css';

/** The mark of each stage, beside its words. */
const STAGE_ICON = {
  search: 'search',
  download: 'download',
  import: 'refresh',
  stalled: 'warning',
} as const;

/**
 * The download progress of a wish in procurement as the viewer sees it (concept 25, 5.3, E-7,
 * architecture 22.2): «Wird gesucht», «Wird heruntergeladen · 45 % · noch 12 min» with a bar,
 * «Wird eingelesen» or «Download verzögert», for a series «3 von 8 Folgen da». Only the server's
 * stage and numbers, never a message, a service or a file. A display, not a focus stop; nothing
 * without progress in the answer. «Meine Wünsche» and the wish card of the preview page.
 */
export function WishProgressLine({ progress }: { progress?: WishProgress }) {
  const { t } = useTranslation();
  if (!progress) return null;
  const parts = [t(`wishes.progress.${progress.stage}`)];
  const downloading = progress.stage === 'download' && progress.percent !== undefined;
  if (downloading) parts.push(t('wishes.progress.percent', { percent: progress.percent }));
  if (progress.episodes) {
    parts.push(
      t('wishes.progress.episodes', {
        count: progress.episodes.total,
        done: progress.episodes.done,
      }),
    );
  }
  if (progress.stage === 'download' && progress.etaSeconds !== undefined) {
    parts.push(t('wishes.progress.eta', { time: formatDuration(t, progress.etaSeconds * 1000) }));
  }
  const text = parts.join(t('format.separator'));
  return (
    <div className={styles.progress} data-stage={progress.stage}>
      <p className={styles.progressLine}>
        <Icon name={STAGE_ICON[progress.stage]} size="sm" />
        <span>{text}</span>
      </p>
      {downloading && <ProgressBar value={progress.percent ?? 0} label={text} />}
    </div>
  );
}
