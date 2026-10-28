import { useTranslation } from 'react-i18next';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Text } from '@/ui';
import { useLibraryOptions } from '../shared/useLibraryOptions';
import { InvitationsSection } from './InvitationsSection';
import styles from './AccessPage.module.css';

/**
 * Access (K-6): the invitation links, for an administrator's primary profile. The accounts an
 * invitation creates are managed in the account list (ADR 0014); this page is only the door.
 */
export function AccessPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.access.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const libraries = useLibraryOptions(allowed);

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.access.title')}
        </Text>
        <Text tone="secondary">{t('admin.access.intro')}</Text>
      </div>
      {allowed ? (
        <div className={styles.sections}>
          <InvitationsSection libraries={libraries} />
        </div>
      ) : (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      )}
    </Page>
  );
}
