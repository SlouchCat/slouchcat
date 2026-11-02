import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import styles from './SaveBar.module.css';
import { Text } from './Text';

interface SaveBarProps {
  /** Something differs from what the server holds. */
  dirty: boolean;
  /** A save is under way: both buttons wait. */
  saving: boolean;
  onSave: () => void;
  onDiscard: () => void;
}

/**
 * The save bar of a page whose settings save together (concept 12, S-7): «Ungespeicherte
 * Änderungen» or «Gespeichert», «Verwerfen» and «Speichern». It sticks to the bottom of the
 * viewport while the page scrolls, so the way to save stays in sight. Messages about the result
 * (affected titles, errors) stay with the page, below the bar.
 */
export function SaveBar({ dirty, saving, onSave, onDiscard }: SaveBarProps) {
  const { t } = useTranslation();
  return (
    <div className={styles.bar} role="region" aria-label={t('ui.saveBar.save')}>
      <Text tone={dirty ? 'primary' : 'secondary'}>
        {dirty ? t('ui.saveBar.dirty') : t('ui.saveBar.clean')}
      </Text>
      <div className={styles.actions}>
        <AriaButton variant="ghost" isDisabled={!dirty || saving} onPress={onDiscard}>
          {t('ui.saveBar.discard')}
        </AriaButton>
        <AriaButton variant="primary" isDisabled={!dirty} isPending={saving} onPress={onSave}>
          {saving ? t('ui.saveBar.saving') : t('ui.saveBar.save')}
        </AriaButton>
      </div>
    </div>
  );
}
