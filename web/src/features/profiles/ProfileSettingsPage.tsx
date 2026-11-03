import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Notice } from '@/ui';
import { AdminPage } from '@/ui/aria';
import { ProfileList } from './ProfileList';

/**
 * Profile management (K-2) of the account's primary profile: list, create, edit, PIN, delete.
 * Every account has it, administrators too (concept 12, KP-1: they also find their profiles in
 * «Konten»); another profile of the account gets a notice with the way to the picker (PRO-0).
 */
export function ProfileSettingsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('profiles.settings.title'));
  const session = useActiveSession();
  const titleRef = useRef<HTMLHeadingElement>(null);

  if (!session.profile.isPrimary) {
    return (
      <AdminPage title={t('profiles.settings.title')}>
        <Notice
          title={t('profiles.settings.onlyPrimary.title')}
          actions={
            <ButtonLink href={paths.profiles} variant="secondary">
              {t('profiles.settings.onlyPrimary.action')}
            </ButtonLink>
          }
        >
          {t('profiles.settings.onlyPrimary.body')}
        </Notice>
      </AdminPage>
    );
  }

  return (
    <ProfileList titleRef={titleRef}>
      {({ add, list }) => (
        <AdminPage
          title={t('profiles.settings.title')}
          help={t('profiles.settings.intro')}
          titleRef={titleRef}
          actions={add}
        >
          {list}
        </AdminPage>
      )}
    </ProfileList>
  );
}
