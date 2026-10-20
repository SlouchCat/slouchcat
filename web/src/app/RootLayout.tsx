import { Outlet, useRouter, type ErrorComponentProps } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { offlineSupported } from '@/features/offline/support';
import { Button, ButtonLink, LinkRouterProvider, Notice, Page } from '@/ui';
import { paths } from './paths';
import { useOfflineEngine } from './useOfflineEngine';

/**
 * Lets the app's links (`href`) navigate through the app router instead of reloading, and runs
 * the offline engine of the page (M5).
 */
export function RootLayout() {
  const router = useRouter();
  useOfflineEngine();
  return (
    <LinkRouterProvider navigate={(href) => void router.navigate({ href })}>
      <Outlet />
    </LinkRouterProvider>
  );
}

/**
 * Shown when the guard cannot reach the server at all. A server error (a proxy in front of a
 * stopped Slouchcat answers 502) leaves the way to the offline library open (W10).
 */
export function RootError({ error, reset }: ErrorComponentProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const toLibrary = offlineSupported() && isApiError(error) && error.status >= 500;
  return (
    <Page layout="form">
      <Notice
        tone="danger"
        title={t('errors.page.title')}
        actions={
          <>
            <Button
              variant="primary"
              icon="refresh"
              onPress={() => {
                reset();
                void router.invalidate();
              }}
            >
              {t('common.retry')}
            </Button>
            {toLibrary && (
              <ButtonLink href={paths.offline} variant="secondary">
                {t('errors.page.offline')}
              </ButtonLink>
            )}
          </>
        }
      >
        {t('errors.page.body')}
      </Notice>
    </Page>
  );
}
