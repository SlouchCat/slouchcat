import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

/** Sets the tab title to "<title> · Slouchcat" while the page is mounted. */
export function useDocumentTitle(title: string | undefined): void {
  const { t } = useTranslation();
  const appName = t('app.name');
  useEffect(() => {
    document.title = title ? `${title} · ${appName}` : appName;
  }, [title, appName]);
}
