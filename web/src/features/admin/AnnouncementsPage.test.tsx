import type { Announcement } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  json,
  mockApi,
  problem,
  renderApp,
  session,
  signedIn,
  type Call,
  type Routes,
} from '@/test/api';

// A-7: the main user writes announcements, changes drafts, publishes them (the server sends them
// on the channels) and sees how many profiles read them.

const draft: Announcement = {
  id: 2,
  title: 'Wartung am Samstag',
  body: 'Der Server ist von 8 bis 10 Uhr nicht erreichbar.',
  createdAt: '2026-10-21T09:00:00Z',
  createdBy: 'owner@example.test',
  acknowledgedCount: 0,
  profileCount: 5,
};

const published: Announcement = {
  id: 1,
  title: 'Neue Filme',
  body: 'Zwanzig neue Filme sind da.',
  createdAt: '2026-10-19T09:00:00Z',
  publishedAt: '2026-10-19T09:05:00Z',
  createdBy: 'owner@example.test',
  acknowledgedCount: 3,
  profileCount: 5,
};

const routes = (list: () => Announcement[], extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/announcements': () => json(list()),
  ...extra,
});

const rowOf = (title: string) => screen.getByRole('rowheader', { name: title }).closest('tr')!;
const sent = (calls: Call[], method: string) => calls.find((call) => call.method === method);

describe('admin announcements (A-7)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists drafts and published announcements with the read counter', async () => {
    mockApi(routes(() => [draft, published]));
    renderApp('/admin/announcements');
    expect(await screen.findByRole('heading', { level: 1, name: 'Ankündigungen' })).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Ankündigungen' });
    expect(
      within(table)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Titel', 'Erstellt', 'Veröffentlicht', 'Gelesen', 'Aktionen']);
    const draftRow = rowOf(draft.title);
    expect(draftRow).toHaveTextContent('Entwurf');
    expect(draftRow).toHaveTextContent('Noch nicht verschickt');
    expect(
      within(draftRow)
        .getAllByRole('button')
        .map((button) => button.getAttribute('aria-label')),
    ).toEqual([
      '„Wartung am Samstag“ bearbeiten',
      '„Wartung am Samstag“ veröffentlichen',
      '„Wartung am Samstag“ löschen',
    ]);
    const publishedRow = rowOf(published.title);
    expect(publishedRow).toHaveTextContent('3 von 5 Profilen');
    expect(
      within(publishedRow)
        .getAllByRole('time')
        .map((time) => time.getAttribute('dateTime')),
    ).toEqual([published.createdAt, published.publishedAt]);
    // A published announcement stays as it was sent: no editing, no second publishing.
    expect(within(publishedRow).getAllByRole('button')).toHaveLength(1);
  });

  it('writes a draft; title and text are required', async () => {
    let list: Announcement[] = [];
    const { calls } = mockApi(
      routes(() => list, {
        'POST /admin/announcements': (call) => {
          const created = { ...draft, ...(call.body as Partial<Announcement>) };
          list = [created];
          return json(created, 201);
        },
      }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Ankündigungen.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Neue Ankündigung' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neue Ankündigung' });
    const title = within(dialog).getByRole('textbox', { name: 'Titel' });
    expect(title).toHaveFocus();
    expect(title).toHaveAttribute('maxLength', '120');
    const body = within(dialog).getByRole('textbox', { name: 'Text' });
    expect(body.tagName).toBe('TEXTAREA');
    expect(body).toHaveAttribute('maxLength', '4000');
    expect(
      within(dialog).getByRole('checkbox', { name: 'Sofort veröffentlichen' }),
    ).not.toBeChecked();

    await user.click(within(dialog).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(within(dialog).getAllByText('Pflichtfeld.')).toHaveLength(2);
    expect(sent(calls, 'POST')).toBeUndefined();

    await user.type(title, 'Wartung am Samstag');
    await user.type(body, 'Zeile eins{Enter}Zeile zwei');
    await user.click(within(dialog).getByRole('button', { name: 'Als Entwurf speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(sent(calls, 'POST')?.body).toEqual({
      title: 'Wartung am Samstag',
      body: 'Zeile eins\nZeile zwei',
      publish: false,
    });
    expect(
      await screen.findByText('„Wartung am Samstag“ ist als Entwurf gespeichert.'),
    ).toBeVisible();
    expect(await screen.findByRole('rowheader', { name: 'Wartung am Samstag' })).toBeVisible();
  });

  it('publishes a new announcement at once when asked to', async () => {
    const { calls } = mockApi(
      routes(() => [], {
        'POST /admin/announcements': () =>
          json({ ...published, title: 'Neu', publishedAt: '2026-10-21T10:00:00Z' }, 201),
      }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neue Ankündigung' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neue Ankündigung' });
    expect(dialog).toHaveTextContent('Gäste bekommen sie standardmässig per E-Mail.');
    await user.type(within(dialog).getByRole('textbox', { name: 'Titel' }), 'Neu');
    await user.type(within(dialog).getByRole('textbox', { name: 'Text' }), 'Hallo');
    await user.click(within(dialog).getByRole('checkbox', { name: 'Sofort veröffentlichen' }));
    await user.click(within(dialog).getByRole('button', { name: 'Veröffentlichen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(sent(calls, 'POST')?.body).toEqual({ title: 'Neu', body: 'Hallo', publish: true });
    expect(await screen.findByText('„Neu“ ist veröffentlicht.')).toBeVisible();
  });

  it('changes a draft; one published meanwhile is refused', async () => {
    let answer = problem(409, 'announcement_published');
    const { calls } = mockApi(
      routes(() => [draft], { 'PUT /admin/announcements/2': () => answer }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: '„Wartung am Samstag“ bearbeiten' }),
    );
    const dialog = await screen.findByRole('dialog', { name: 'Entwurf bearbeiten' });
    const title = within(dialog).getByRole('textbox', { name: 'Titel' });
    expect(title).toHaveValue(draft.title);
    expect(within(dialog).getByRole('textbox', { name: 'Text' })).toHaveValue(draft.body);
    expect(within(dialog).queryByRole('checkbox')).toBeNull();
    await user.type(title, ' (verschoben)');
    await user.click(within(dialog).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(
      await within(dialog).findByText(
        'Die Ankündigung ist schon veröffentlicht und lässt sich nicht mehr ändern.',
      ),
    ).toBeVisible();

    answer = json({ ...draft, title: 'Wartung am Samstag (verschoben)' });
    await user.click(within(dialog).getByRole('button', { name: 'Als Entwurf speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(sent(calls, 'PUT')?.body).toEqual({
      title: 'Wartung am Samstag (verschoben)',
      body: draft.body,
    });
  });

  it('publishes a draft after the confirmation; the focus goes to the heading', async () => {
    let list = [draft];
    const { calls } = mockApi(
      routes(() => list, {
        'POST /admin/announcements/2/publish': () => {
          list = [{ ...draft, publishedAt: '2026-10-21T10:00:00Z' }];
          return json(list[0]);
        },
      }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }),
    );
    const dialog = await screen.findByRole('alertdialog', {
      name: '„Wartung am Samstag“ veröffentlichen?',
    });
    expect(dialog).toHaveTextContent('Gäste bekommen sie standardmässig per E-Mail.');
    await user.click(within(dialog).getByRole('button', { name: 'Veröffentlichen' }));
    await waitFor(() => expect(sent(calls, 'POST')?.path).toBe('/admin/announcements/2/publish'));
    expect(await screen.findByText('„Wartung am Samstag“ ist veröffentlicht.')).toBeVisible();
    await waitFor(() => expect(rowOf(draft.title)).toHaveTextContent('0 von 5 Profilen'));
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: 'Ankündigungen' }).parentElement,
      ).toHaveFocus(),
    );
  });

  it('deletes an announcement after the confirmation; an older refusal goes away', async () => {
    let list = [draft, published];
    const { calls } = mockApi(
      routes(() => list, {
        'POST /admin/announcements/2/publish': () => problem(409, 'announcement_published'),
        'DELETE /admin/announcements/1': () => {
          list = [draft];
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }),
    );
    const refused = await screen.findByRole('alertdialog', {
      name: '„Wartung am Samstag“ veröffentlichen?',
    });
    await user.click(within(refused).getByRole('button', { name: 'Veröffentlichen' }));
    expect(await screen.findByRole('alert')).toBeVisible();

    await user.click(await screen.findByRole('button', { name: '„Neue Filme“ löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: '„Neue Filme“ löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Ankündigung löschen' }));
    await waitFor(() => expect(sent(calls, 'DELETE')?.path).toBe('/admin/announcements/1'));
    expect(await screen.findByText('„Neue Filme“ ist gelöscht.')).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole('rowheader', { name: 'Neue Filme' })).toBeNull());
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/announcements');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
