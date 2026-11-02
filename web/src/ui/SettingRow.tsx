import { useId, type ReactNode } from 'react';
import styles from './SettingRow.module.css';

interface SettingRowProps {
  /** What the setting is ("Bildsprachen"). */
  label: string;
  /** A short line under the label: what the setting does (it may hold a link). */
  help?: ReactNode;
  /**
   * The control, on the right. It keeps its own accessible name (a switch's label may be
   * visually hidden); the row names the group and describes it with `help`.
   */
  children: ReactNode;
}

/**
 * One setting per line (concept 12, S-7): label and help on the left, the control on the right;
 * on narrow screens the control moves below. The row is a group named by the label and described
 * by the help, so a screen reader hears both when it enters the control.
 */
export function SettingRow({ label, help, children }: SettingRowProps) {
  const labelId = useId();
  const helpId = useId();
  return (
    <div
      role="group"
      aria-labelledby={labelId}
      aria-describedby={help ? helpId : undefined}
      className={styles.row}
    >
      <div className={styles.text}>
        <span id={labelId} className={styles.label}>
          {label}
        </span>
        {help && (
          <span id={helpId} className={styles.help}>
            {help}
          </span>
        )}
      </div>
      <div className={styles.control}>{children}</div>
    </div>
  );
}
