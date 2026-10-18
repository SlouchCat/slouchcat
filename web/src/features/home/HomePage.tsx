import { useTranslation } from 'react-i18next';
import { Page, Stack, Text } from '@/ui';
import { ServerStatus } from './ServerStatus';

export function HomePage() {
  const { t } = useTranslation();

  return (
    <Page>
      <Stack gap={6}>
        <Stack gap={2}>
          <Text as="h1" variant="display">
            {t('app.name')}
          </Text>
          <Text tone="secondary">{t('app.tagline')}</Text>
        </Stack>
        <ServerStatus />
      </Stack>
    </Page>
  );
}
