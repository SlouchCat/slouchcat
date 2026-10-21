import { Outlet, useLocation } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { PageNav } from '@/ui';
import { AdminNav, type AdminNavGroup } from '@/ui/aria';
import { adminGroups, adminSections, isSectionPath } from './sections';

/**
 * Parent of every /admin page (A-2): the admin navigation at the top of each page, for the main
 * account's primary profile. Anyone else gets the pages' own "only for the main account" notice
 * without it.
 */
export function AdminLayout() {
  const { t } = useTranslation();
  const session = useActiveSession();
  const { pathname } = useLocation();
  if (session.account.role !== 'owner' || !session.profile.isPrimary) return <Outlet />;

  const groups: AdminNavGroup[] = [
    {
      links: [
        { href: paths.admin, label: t('admin.nav.overview'), current: pathname === paths.admin },
      ],
    },
    ...adminGroups.map((group) => ({
      label: t(`admin.nav.groups.${group}`),
      links: adminSections
        .filter((section) => section.group === group && section.href.startsWith(`${paths.admin}/`))
        .map((section) => ({
          href: section.href,
          label: t(`admin.sections.${section.key}.title`),
          current: isSectionPath(section, pathname),
        })),
    })),
  ];

  return (
    <PageNav nav={<AdminNav label={t('admin.nav.label')} groups={groups} />}>
      <Outlet />
    </PageNav>
  );
}
