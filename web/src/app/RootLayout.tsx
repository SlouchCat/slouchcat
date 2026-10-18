import { Outlet, useRouter, type ErrorComponentProps } from '@tanstack/react-router';
import { RouterProvider } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Button, Notice, Page } from '@/ui';

/** Lets react-aria links (`href`) navigate through the app router instead of reloading. */
export function RootLayout() {
  const router = useRouter();
  return (
    <RouterProvider navigate={(href) => void router.navigate({ href })}>
      <Outlet />
    </RouterProvider>
  );
}

/** Shown when the guard cannot reach the server at all. */
export function RootError({ reset }: ErrorComponentProps) {
  const { t } = useTranslation();
  const router = useRouter();
  return (
    <Page layout="form">
      <Notice
        tone="danger"
        title={t('errors.page.title')}
        actions={
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
        }
      >
        {t('errors.page.body')}
      </Notice>
    </Page>
  );
}
