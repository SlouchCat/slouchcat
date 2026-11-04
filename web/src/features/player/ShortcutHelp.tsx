import { useTranslation } from 'react-i18next';
import { PlayerPanel } from '@/ui/aria';
import styles from './ShortcutHelp.module.css';

/** The rows of the help in the order of the key map (concept 13, 11.2). */
const ROWS = [
  'play',
  'seek',
  'chapter',
  'volume',
  'zones',
  'mute',
  'jump',
  'speed',
  'tracks',
  'subtitles',
  'chapters',
  'quality',
  'fullscreen',
  'info',
  'skip',
  'next',
  'help',
  'back',
] as const;

/**
 * The shortcut help (V-12, key ?): every key of the video player in a panel beside the video,
 * which keeps playing. Escape, Backspace and the close button close it (PlayerPanel), which gives
 * the focus back to the «?» button when it opened the help.
 */
export function ShortcutHelp({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  return (
    <PlayerPanel title={t('player.help.title')} onClose={onClose}>
      {/* The list is the panel's focus target: text that scrolls must take the focus (axe
          «scrollable-region-focusable»), the arrows then scroll it (the panel keeps them from
          the player), and a remote reaches every row. */}
      {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex */}
      <dl className={styles.list} tabIndex={0} aria-label={t('player.help.title')}>
        {ROWS.map((row) => (
          <div key={row} className={styles.row}>
            <dt className={styles.keys}>
              <kbd>{t(`player.help.rows.${row}.keys`)}</kbd>
            </dt>
            <dd className={styles.action}>{t(`player.help.rows.${row}.action`)}</dd>
          </div>
        ))}
      </dl>
    </PlayerPanel>
  );
}
