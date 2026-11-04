import { useTranslation } from 'react-i18next';
import { TitleOrLogo } from '@/ui';
import styles from './Player.module.css';

/** The pause view shows after this long paused without input (V-10). */
export const PAUSE_VIEW_MS = 5000;

interface PauseViewProps {
  title: string;
  logoUrl?: string;
  /** «S01E04 · Wahrheit», or the year, runtime and genres of a film. */
  detail?: string;
  overview?: string;
  /** The chapter the player paused in. */
  chapter?: string;
}

/**
 * The pause view (V-10): after five seconds of pause the picture darkens and the title shows on
 * the left with its logo, the year or episode, a short description and the chapter. Any key or
 * movement hides it (VideoPlayer). It takes no focus and lets the pointer through: the controls
 * stay where they are, so a key acts on them as before.
 */
export function PauseView({ title, logoUrl, detail, overview, chapter }: PauseViewProps) {
  const { t } = useTranslation();
  return (
    <section className={styles.pauseView} aria-label={t('player.pauseView.label')}>
      <div className={styles.pauseInfo}>
        <p className={styles.pauseEyebrow}>{t('player.pauseView.paused')}</p>
        <h2 className={styles.pauseTitle}>
          <TitleOrLogo title={title} logoUrl={logoUrl} className={styles.pauseLogo} />
        </h2>
        {detail && <p className={styles.pauseDetail}>{detail}</p>}
        {overview && <p className={styles.pauseOverview}>{overview}</p>}
        {chapter && (
          <p className={styles.pauseDetail}>{t('player.pauseView.chapter', { chapter })}</p>
        )}
      </div>
    </section>
  );
}
