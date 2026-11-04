import { useTranslation } from 'react-i18next';
import { PlayerPanel } from '@/ui/aria';
import { PANEL_IDS } from './panels/panelIds';
import styles from './ShortcutHelp.module.css';

/** The rows of the help in the order of the key map (concept 13, 11.2). */
export const SHORTCUT_ROWS = [
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

export type ShortcutRow = (typeof SHORTCUT_ROWS)[number];

/**
 * The shortcut help (V-12, key ?): every key of the video player in a panel beside the video,
 * which keeps playing. Escape, Backspace and the close button close it (PlayerPanel), which gives
 * the focus back to the «?» button when it opened the help. `rows` leaves out what a player cannot
 * do (the offline player has no quality, stream info or next episode); `backAction` says where
 * Escape leads when nothing is open, if not to the detail page (offline: the downloads).
 */
export function ShortcutHelp({
  onClose,
  rows = SHORTCUT_ROWS,
  backAction,
}: {
  onClose: () => void;
  rows?: readonly ShortcutRow[];
  backAction?: string;
}) {
  const { t } = useTranslation();
  return (
    <PlayerPanel id={PANEL_IDS.help} title={t('player.help.title')} onClose={onClose}>
      {/* The list is the panel's focus target: text that scrolls must take the focus (axe
          «scrollable-region-focusable»), the arrows then scroll it (the panel keeps them from
          the player), and a remote reaches every row. */}
      {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex */}
      <dl className={styles.list} tabIndex={0} aria-label={t('player.help.title')}>
        {rows.map((row) => (
          <div key={row} className={styles.row}>
            <dt className={styles.keys}>
              <kbd>{t(`player.help.rows.${row}.keys`)}</kbd>
            </dt>
            <dd className={styles.action}>
              {row === 'back' && backAction !== undefined
                ? backAction
                : t(`player.help.rows.${row}.action`)}
            </dd>
          </div>
        ))}
      </dl>
    </PlayerPanel>
  );
}
