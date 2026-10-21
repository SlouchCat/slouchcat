import { useRef, useState } from 'react';
import { AriaButton } from './AriaButton';
import styles from './CodeBlock.module.css';

interface CodeBlockProps {
  /** The command, shown and copied verbatim. */
  code: string;
  /** Accessible name of the block ("Compose-Befehl für NVIDIA"). */
  label: string;
  /** Text of the copy button ("Kopieren"). */
  copyLabel: string;
  /** Announced after a successful copy ("Kopiert."). */
  copiedMessage: string;
  /** Shown when the browser has no clipboard (plain HTTP); the command is then selected. */
  copyFailedMessage: string;
}

/**
 * A command to copy (setup wizard, dashboard): monospace text that wraps instead of scrolling and
 * a copy button. Without a clipboard (plain-HTTP origins) the command is selected for Ctrl+C.
 */
export function CodeBlock({
  code,
  label,
  copyLabel,
  copiedMessage,
  copyFailedMessage,
}: CodeBlockProps) {
  const text = useRef<HTMLElement>(null);
  const [status, setStatus] = useState<'copied' | 'failed' | null>(null);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(code);
      setStatus('copied');
    } catch {
      const node = text.current;
      const selection = window.getSelection();
      if (node && selection) {
        const range = document.createRange();
        range.selectNodeContents(node);
        selection.removeAllRanges();
        selection.addRange(range);
      }
      setStatus('failed');
    }
  };

  return (
    <div role="group" aria-label={label} className={styles.block}>
      <pre className={styles.code}>
        <code ref={text}>{code}</code>
      </pre>
      <div className={styles.footer}>
        <AriaButton variant="secondary" icon="copy" onPress={() => void copy()}>
          {copyLabel}
        </AriaButton>
        <p role="status" className={styles.status}>
          {status === 'copied' && copiedMessage}
          {status === 'failed' && copyFailedMessage}
        </p>
      </div>
    </div>
  );
}
