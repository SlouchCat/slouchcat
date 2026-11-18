import { useTranslation } from 'react-i18next';
import { useSpatialHold } from '@/ui';
import { ConfirmDialog } from '@/ui/aria';

interface WithdrawDialogProps {
  title: string;
  /** A joined wish: only the profile's own part goes (concept 25, 4.8). */
  joined: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/**
 * «Wunsch zurückziehen?» (concept 25, 4.8, F-17): loaded on demand from a browsing page, it holds
 * the page's arrow keys while open; «Abbrechen» is the default.
 */
export function WithdrawDialog({ title, joined, onConfirm, onClose }: WithdrawDialogProps) {
  const { t } = useTranslation();
  useSpatialHold(true);
  return (
    <ConfirmDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t(joined ? 'wishes.withdraw.titleJoined' : 'wishes.withdraw.title')}
      body={t(joined ? 'wishes.withdraw.bodyJoined' : 'wishes.withdraw.body', { title })}
      confirmLabel={t(joined ? 'wishes.withdraw.confirmJoined' : 'wishes.withdraw.confirm')}
      cancelLabel={t('common.cancel')}
      tone="danger"
      defaultFocus="cancel"
      onConfirm={onConfirm}
    />
  );
}
