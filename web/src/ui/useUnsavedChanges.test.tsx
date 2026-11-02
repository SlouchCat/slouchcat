import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { adminSearch } from '@/app/router';
import { ConfirmDialog } from './Dialog';
import { Tabs } from './Tabs';
import { useDrafts } from './useDrafts';
import { useSearchSelection } from './useSearchSelection';
import { useUnsavedChanges } from './useUnsavedChanges';

const TABS = ['library', 'failed'];

/** A page with tabs bound to ?tab=, a draft and the guard (concept 12, S-13, S-14, S-17). */
function JobsPage() {
  const [tab, setTab] = useSearchSelection('tab', TABS, 'library');
  const drafts = useDrafts<string>(() => 'alle 6 h');
  const unsaved = useUnsavedChanges(drafts.dirty);
  return (
    <>
      <h1>Hintergrundjobs</h1>
      <p>{unsaved.pausePolling ? 'Abfragen pausiert' : 'Abfragen läuft'}</p>
      <Link to="/admin/backup">Sicherung</Link>
      <Tabs
        label="Gruppen"
        selectedKey={tab}
        onSelectionChange={setTab}
        tabs={[
          {
            id: 'library',
            label: 'Bibliothek',
            content: (
              <input
                aria-label="Zeitplan"
                value={drafts.valueOf('scan') ?? ''}
                onChange={(event) => drafts.set('scan', event.target.value)}
              />
            ),
          },
          { id: 'failed', label: 'Fehlschläge', content: 'keine' },
        ]}
      />
      <ConfirmDialog {...unsaved.dialog} />
    </>
  );
}

function setup(path = '/admin/jobs') {
  const root = createRootRoute({ component: Outlet });
  const jobs = createRoute({
    getParentRoute: () => root,
    path: '/admin/jobs',
    validateSearch: adminSearch,
    component: JobsPage,
  });
  const backup = createRoute({
    getParentRoute: () => root,
    path: '/admin/backup',
    component: () => <h1>Sicherung</h1>,
  });
  const router = createRouter({
    routeTree: root.addChildren([jobs, backup]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  render(<RouterProvider router={router as never} />);
  return router;
}

describe('useSearchSelection (concept 12, S-14)', () => {
  it('reads the tab from ?tab= and writes it back, the default without a parameter', async () => {
    const user = userEvent.setup();
    const router = setup('/admin/jobs?tab=failed');
    expect(await screen.findByRole('tab', { name: 'Fehlschläge' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await user.click(screen.getByRole('tab', { name: 'Bibliothek' }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    await user.click(screen.getByRole('tab', { name: 'Fehlschläge' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'failed' }));
  });

  it('falls back to the default for an unknown tab', async () => {
    setup('/admin/jobs?tab=nothing');
    expect(await screen.findByRole('tab', { name: 'Bibliothek' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });
});

describe('useUnsavedChanges (concept 12, S-13)', () => {
  it('lets a clean page go without asking', async () => {
    const user = userEvent.setup();
    const router = setup();
    expect(await screen.findByText('Abfragen läuft')).toBeVisible();
    await user.click(screen.getByRole('link', { name: 'Sicherung' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/backup'));
  });

  it('asks before leaving with unsaved input, not when only ?tab= changes', async () => {
    const user = userEvent.setup();
    const router = setup();
    const field = await screen.findByRole('textbox', { name: 'Zeitplan' });
    await user.clear(field);
    await user.type(field, 'alle 2 h');
    expect(screen.getByText('Abfragen pausiert')).toBeVisible();

    // Another tab of the same page: no question, the draft stays.
    await user.click(screen.getByRole('tab', { name: 'Fehlschläge' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'failed' }));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    await user.click(screen.getByRole('tab', { name: 'Bibliothek' }));
    expect(await screen.findByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');

    // Leaving: the question; staying keeps page and draft.
    await user.click(screen.getByRole('link', { name: 'Sicherung' }));
    const question = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(screen.getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(question).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe('/admin/jobs');
    expect(screen.getByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');

    // Leaving again and confirming.
    await user.click(screen.getByRole('link', { name: 'Sicherung' }));
    await user.click(await screen.findByRole('button', { name: 'Verwerfen und verlassen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/backup'));
    expect(await screen.findByRole('heading', { name: 'Sicherung' })).toBeVisible();
  });
});
