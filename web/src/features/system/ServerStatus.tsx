import { useTranslation } from 'react-i18next';
import { Stack, StatusDot, Text } from '@/ui';
import styles from './ServerStatus.module.css';
import { useHealth } from './useHealth';

export function ServerStatus() {
  const { t } = useTranslation();
  const health = useHealth();

  let state: 'pending' | 'success' | 'danger' = 'pending';
  let label = t('home.server.checking');
  if (health.isSuccess) {
    state = 'success';
    label = t('home.server.online');
  } else if (health.isError) {
    state = 'danger';
    label = t('home.server.offline');
  }

  return (
    <div role="status" aria-live="polite" className={styles.status}>
      <Stack direction="row" gap={2}>
        <StatusDot state={state} />
        <Text as="span" variant="caption" tone="secondary">
          {label}
        </Text>
        {health.data && (
          <Text as="span" variant="caption" tone="muted">
            {t('home.server.version', { version: health.data.version })}
          </Text>
        )}
      </Stack>
    </div>
  );
}
