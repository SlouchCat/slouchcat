import type { AdminSettings } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { AriaButton, ConfirmDialog, useStatusMessage } from '@/ui/aria';
import { rotatePushKeys, settingsQuery } from '../queries';
import styles from './Integrations.module.css';
import { Outcome } from './shared';

/** Web Push: whether a VAPID pair exists, its public key, and a new pair after a confirmation. */
export function PushBlock({ push }: { push: AdminSettings['push'] }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const rotate = useMutation({
    mutationFn: rotatePushKeys,
    onMutate: () => setMessage(null),
    onSuccess: (settings) => {
      queryClient.setQueryData(settingsQuery.queryKey, settings);
      setMessage(t('admin.integrations.push.rotated'));
    },
  });
  return (
    <>
      <Text tone="secondary">{t('admin.integrations.push.intro')}</Text>
      <Text tone="secondary">
        {push.configured
          ? t('admin.integrations.push.configured')
          : t('admin.integrations.push.notConfigured')}
      </Text>
      {push.publicKey && (
        <div className={styles.publicKey}>
          <Text as="span" variant="caption" tone="muted">
            {t('admin.integrations.push.publicKey')}
          </Text>
          <code className={styles.url}>{push.publicKey}</code>
        </div>
      )}
      <div className={styles.actions}>
        <ConfirmDialog
          trigger={
            <AriaButton variant="dangerQuiet" icon="refresh" isPending={rotate.isPending}>
              {t('admin.integrations.push.rotate.action')}
            </AriaButton>
          }
          title={t('admin.integrations.push.rotate.title')}
          body={t('admin.integrations.push.rotate.body')}
          confirmLabel={t('admin.integrations.push.rotate.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => rotate.mutate()}
        />
      </div>
      <Outcome error={rotate.error} message={message} />
    </>
  );
}
