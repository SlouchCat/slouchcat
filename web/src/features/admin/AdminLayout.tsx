import { Outlet } from '@tanstack/react-router';
import { useActiveSession } from '@/app/session';
import { useWideScreen } from '@/lib/useWideScreen';
import { PageNav } from '@/ui';
import { AdminShellMenu } from './AdminShellNav';

/**
 * Parent of every /admin page (A-2). From 768 px the administration's sidebar stands in the
 * shell's column (AppLayout, concept 11); on a phone every page starts with the button
 * «Verwaltung · page» that opens the same entries as a sheet. Anyone but an administrator's
 * primary profile gets the pages' own "only for administrators" notice without either.
 */
export function AdminLayout() {
  const session = useActiveSession();
  const wide = useWideScreen();
  if (session.account.role !== 'admin' || !session.profile.isPrimary) return <Outlet />;
  // The same tree in both widths, so turning a tablet keeps the page and its state.
  return (
    <PageNav nav={wide ? null : <AdminShellMenu />}>
      <Outlet />
    </PageNav>
  );
}
