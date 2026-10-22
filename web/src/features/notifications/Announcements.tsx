import { useQuery } from '@tanstack/react-query';
import { lazy, Suspense, useState, type ComponentProps, type ComponentType } from 'react';
import type { AnnouncementDialog as Dialog } from './AnnouncementDialog';
import { announcementsQuery } from './announcements';

type DialogModule = { default: ComponentType<ComponentProps<typeof Dialog>> };

// The dialog needs react-aria, which the start page does not load up front (P-10): it comes as
// its own chunk only when the profile has an announcement to read. A chunk that does not load
// (an old tab after an update) means no dialog, not an error page instead of the whole layout.
const AnnouncementDialog = lazy(() =>
  import('./AnnouncementDialog').then(
    (module): DialogModule => ({ default: module.AnnouncementDialog }),
    (): DialogModule => ({ default: () => null }),
  ),
);

/**
 * Announcements of the main user at the start (A-7): asked once per profile, the newest open one
 * shows in a dialog. "Verstanden" acknowledges it and the next one follows; Escape, Back or
 * "Später" close the dialog for this visit without acknowledging, so it comes back next start.
 */
export function Announcements({ profileId }: { profileId: number }) {
  const open = useQuery(announcementsQuery(profileId));
  const [closedFor, setClosedFor] = useState<number | null>(null);
  const list = open.data ?? [];
  if (list.length === 0 || closedFor === profileId) return null;
  return (
    <Suspense fallback={null}>
      <AnnouncementDialog
        profileId={profileId}
        announcements={list}
        onClose={() => setClosedFor(profileId)}
      />
    </Suspense>
  );
}
