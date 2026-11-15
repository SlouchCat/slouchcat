import { Outlet } from '@tanstack/react-router';
import { useRef } from 'react';
import { useActiveSession } from '@/app/session';
import { useWideScreen } from '@/lib/useWideScreen';
import { PageNav } from '@/ui';
import { AdminFrame } from '@/ui/aria';
import { AdminShellMenu, useAdminNav } from './AdminShellNav';

/**
 * Parent of every /admin page (A-2). From 768 px the administration's sidebar stands in the
 * shell's column (AppLayout, concept 11); on a phone every page starts with the button
 * «Verwaltung › area / page» that opens every page as a sheet. Every AdminPage below gets the
 * frame of the redesign (build plan 2.5, 2.6): its area's tabs and the line over its title.
 * Anyone but an administrator's primary profile gets the pages' own "only for administrators"
 * notice without any of it.
 */
export function AdminLayout() {
  const session = useActiveSession();
  if (session.account.role !== 'admin' || !session.profile.isPrimary) return <Outlet />;
  return <AdminFramedOutlet />;
}

function AdminFramedOutlet() {
  const wide = useWideScreen();
  const nav = useAdminNav();
  // The focused tab, carried over to the next page's tabs (AreaTabs).
  const tabFocus = useRef<number | null>(null);
  // The same tree in both widths, so turning a tablet keeps the page and its state.
  return (
    <AdminFrame
      value={{
        eyebrow: nav.trail,
        tabs: nav.tabs && { ...nav.tabs, focusCarry: tabFocus },
      }}
    >
      <PageNav nav={wide ? null : <AdminShellMenu nav={nav} />}>
        <Outlet />
      </PageNav>
    </AdminFrame>
  );
}
