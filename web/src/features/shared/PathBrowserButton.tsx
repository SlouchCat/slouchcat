import { useRef, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from '@/ui/aria';
import { PathBrowserDialog } from './PathBrowserDialog';

interface PathBrowserButtonProps {
  /** The folder field's current value; the dialog opens in that folder. */
  path: string;
  onPathChange: (path: string) => void;
  /** The folder field, focused after "Apply" so the path can be corrected right away. */
  fieldRef?: RefObject<HTMLElement | null>;
}

/**
 * "Browse …" next to the folder field (idea 02). The field stays an equal way in: the dialog is
 * a shortcut, not a replacement. The dialog stays mounted while it is closed -- its queries are
 * switched off with `enabled`, which cancels a running request through its signal, and only a
 * mounted dialog can hand the focus on after it closed.
 */
export function PathBrowserButton({ path, onPathChange, fieldRef }: PathBrowserButtonProps) {
  const { t } = useTranslation();
  const [isOpen, setOpen] = useState(false);
  // Which element takes the focus when the dialog closes: the field after "Apply", and nothing
  // after Escape or Cancel, where react-aria returns it to this button.
  const returnFocus = useRef<HTMLElement | null>(null);
  const applying = useRef(false);

  return (
    <>
      <AriaButton icon="folder" onPress={() => setOpen(true)}>
        {t('library.browse.open')}
      </AriaButton>
      <PathBrowserDialog
        isOpen={isOpen}
        onOpenChange={(open) => {
          if (!open && !applying.current) returnFocus.current = null;
          applying.current = false;
          setOpen(open);
        }}
        currentPath={path}
        returnFocus={returnFocus}
        onApply={(chosen) => {
          applying.current = true;
          returnFocus.current = fieldRef?.current ?? null;
          onPathChange(chosen);
        }}
      />
    </>
  );
}
