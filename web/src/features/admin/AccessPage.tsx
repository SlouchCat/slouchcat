import { useTranslation } from 'react-i18next';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Text } from '@/ui';
import { useLibraryOptions } from '../shared/useLibraryOptions';
import { GuestsSection } from './GuestsSection';
import { InvitationsSection } from './InvitationsSection';
import styles from './AccessPage.module.css';

/** Access (K-6, K-7): invitation links and guest accounts, for the main account's primary profile. */
export function AccessPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.access.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
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
          <GuestsSection libraries={libraries} />
        </div>
      ) : (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      )}
    </Page>
  );
}
