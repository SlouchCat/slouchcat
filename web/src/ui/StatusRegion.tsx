import { useCallback, useState, type ReactNode } from 'react';
import { Notice } from './Notice';
import styles from './StatusRegion.module.css';

/** One message of a status region. `key` makes every message a new node (see StatusRegion). */
export interface StatusMessage {
  text: ReactNode;
  tone?: 'info' | 'warning' | 'danger';
  key: number;
}

interface StatusRegionProps {
  /** The message shown, or none. */
  message?: StatusMessage | null;
  /** `status` (default) is announced politely, `alert` at once. */
  role?: 'status' | 'alert';
  /** Several messages at once (one per saved scope): `Notice`s with `live={false}`. */
  children?: ReactNode;
}

/**
 * The outcome line of a page or block (concept 12, S-12): a live region that is always there, so
 * even its first message is announced, and that stays in the accessibility tree while empty
 * (never `display: none`). Empty, it takes no room and no gap in a flex or grid parent. Every
 * message is its own node (keyed), so the same text twice in a row is announced twice.
 */
export function StatusRegion({ message, role = 'status', children }: StatusRegionProps) {
  return (
    <div role={role} className={styles.region}>
      {message && (
        <Notice key={message.key} tone={message.tone} live={false}>
          {message.text}
        </Notice>
      )}
      {children}
    </div>
  );
}

/**
 * The message state of a StatusRegion: `say(text)` shows a message as a new node (also the same
 * text again), `say(null)` empties the region.
 */
export function useStatusMessage(): [
  message: StatusMessage | null,
  say: (text: ReactNode | null, tone?: StatusMessage['tone']) => void,
] {
  const [message, setMessage] = useState<StatusMessage | null>(null);
  const say = useCallback(
    (text: ReactNode | null, tone?: StatusMessage['tone']) =>
      setMessage((current) =>
        text === null ? null : { text, tone, key: (current?.key ?? 0) + 1 },
      ),
    [],
  );
  return [message, say];
}
