import type { Announcement } from '@slouchcat/api-client';
import { focusManager } from '@tanstack/react-query';
import { act, screen, waitFor, within } from '@testing-library/react';
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
// on the channels) and sees how many profiles read them. Concept 12, variant B: the list of
// drafts and published ones on the left, the selected one on the right with its form and the
// preview «So sehen es die Profile».

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
  body: 'Zwanzig neue Filme sind da.\n\nViel Spass!',
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

const listBox = () => screen.getByRole('listbox', { name: 'Ankündigungen' });
const option = (name: string | RegExp) => within(listBox()).getByRole('option', { name });
const detail = (name: string) => screen.getByRole('region', { name });
const preview = (region: HTMLElement) =>
  within(region).getByRole('heading', { level: 3, name: 'So sehen es die Profile' })
    .nextElementSibling as HTMLElement;
const sent = (calls: Call[], method: string) => calls.find((call) => call.method === method);

describe('admin announcements (A-7)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists drafts and published announcements; the selected one shows its read progress', async () => {
    mockApi(routes(() => [published, draft]));
    const { router } = renderApp('/admin/announcements');
    expect(await screen.findByRole('heading', { level: 1, name: 'Ankündigungen' })).toBeVisible();
    await screen.findByRole('listbox', { name: 'Ankündigungen' });
    // Drafts first, then the published ones, each under its heading.
    expect(
      within(listBox())
        .getAllByRole('option')
        .map((entry) => entry.getAttribute('aria-label') ?? entry.textContent),
    ).toEqual(['Wartung am SamstagErstellt am 21.10.2026', 'Neue Filme3 von 5 gelesen']);
    expect(
      within(listBox())
        .getAllByRole('group')
        .map((group) => group.textContent),
    ).toEqual([expect.stringMatching(/^Entwürfe/), expect.stringMatching(/^Veröffentlicht/)]);

    // The first one is selected without `?item=`: a draft, still to be sent.
    expect(option(/Wartung am Samstag/)).toHaveAttribute('aria-selected', 'true');
    const draftDetail = detail('Wartung am Samstag');
    expect(draftDetail).toHaveTextContent('Entwurf');
    expect(draftDetail).toHaveTextContent('Noch nicht verschickt');
    expect(within(draftDetail).queryByRole('progressbar')).toBeNull();
    expect(
      within(draftDetail)
        .getAllByRole('button')
        .map((button) => button.getAttribute('aria-label') ?? button.textContent),
    ).toEqual([
      'Als Entwurf speichern',
      '„Wartung am Samstag“ veröffentlichen',
      '„Wartung am Samstag“ löschen',
    ]);

    // A published one: its dates, how many read it as a bar and in words, only «Löschen».
    const user = userEvent.setup();
    await user.click(option(/Neue Filme/));
    expect(router.state.location.searchStr).toBe('?item=announcement-1');
    const publishedDetail = detail('Neue Filme');
    expect(
      within(publishedDetail)
        .getAllByRole('time')
        .map((time) => time.getAttribute('dateTime')),
    ).toEqual([published.createdAt, published.publishedAt]);
    const bar = within(publishedDetail).getByRole('progressbar', { name: 'Gelesen' });
    expect(bar).toHaveAttribute('aria-valuenow', '60');
    expect(publishedDetail).toHaveTextContent('3 von 5 Profilen');
    // A published announcement stays as it was sent: no form, no second publishing.
    expect(within(publishedDetail).queryByRole('form')).toBeNull();
    expect(within(publishedDetail).getAllByRole('button')).toHaveLength(1);
    expect(
      within(publishedDetail).getByRole('button', { name: '„Neue Filme“ löschen' }),
    ).toBeVisible();
    // The preview is the profiles' dialog without buttons, with the whole text.
    const shown = preview(publishedDetail);
    expect(within(shown).getByText('Ankündigung')).toBeVisible();
    expect(within(shown).getByText('Veröffentlicht am 19.10.2026')).toBeVisible();
    expect(within(shown).getByText('Zwanzig neue Filme sind da.')).toBeVisible();
    expect(within(shown).getByText('Viel Spass!')).toBeVisible();
    expect(within(shown).queryByRole('button')).toBeNull();
  });

  it('writes a draft in place; title and text are required', async () => {
    let list: Announcement[] = [];
    const { calls, router } = mockApiAndRender(() => list, {
      'POST /admin/announcements': (call) => {
        const created = { ...draft, id: 7, ...(call.body as Partial<Announcement>) };
        list = [created];
        return json(created, 201);
      },
    });
    const user = userEvent.setup();
    expect(await screen.findByText('Noch keine Ankündigungen.')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Neue Ankündigung' }));
    const form = await screen.findByRole('form', { name: 'Neue Ankündigung' });
    const title = within(form).getByRole('textbox', { name: 'Titel' });
    await waitFor(() => expect(title).toHaveFocus());
    expect(title).toHaveAttribute('maxLength', '120');
    const body = within(form).getByRole('textbox', { name: 'Text' });
    expect(body.tagName).toBe('TEXTAREA');
    expect(body).toHaveAttribute('maxLength', '4000');
    expect(
      within(form).getByRole('checkbox', { name: 'Sofort veröffentlichen' }),
    ).not.toBeChecked();
    // The new one has an address of its own and stands in the list as not saved.
    await waitFor(() => expect(router.state.location.search).toMatchObject({ item: 'new' }));
    expect(option(/Neue Ankündigung/)).toHaveTextContent('Nicht gespeichert');
    expect(within(form).getByText('0 / 120')).toBeVisible();

    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(within(form).getAllByText('Pflichtfeld.')).toHaveLength(2);
    expect(sent(calls, 'POST')).toBeUndefined();

    await user.type(title, 'Wartung am Samstag');
    expect(within(form).getByText('18 / 120')).toBeVisible();
    await user.type(body, 'Zeile eins{Enter}Zeile zwei');
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    await waitFor(() => expect(form).not.toBeInTheDocument());
    expect(sent(calls, 'POST')?.body).toEqual({
      title: 'Wartung am Samstag',
      body: 'Zeile eins\nZeile zwei',
      publish: false,
    });
    expect(
      await screen.findByText('„Wartung am Samstag“ ist als Entwurf gespeichert.'),
    ).toBeVisible();
    // The saved draft takes the new one's place, in the list and in the address.
    await waitFor(() =>
      expect(router.state.location.search).toMatchObject({ item: 'announcement-7' }),
    );
    expect(option(/Wartung am Samstag/)).toHaveAttribute('aria-selected', 'true');
    expect(within(listBox()).queryByRole('option', { name: /Nicht gespeichert/ })).toBeNull();
    // The form went with the new one: the same place in the saved draft has the focus.
    await waitFor(() =>
      expect(
        within(detail('Wartung am Samstag')).getByRole('button', {
          name: 'Als Entwurf speichern',
        }),
      ).toHaveFocus(),
    );
  });

  it('keeps what is typed while the new one is being saved', async () => {
    const created: Announcement = { ...draft, id: 7, title: 'Kinoabend', body: 'Am Freitag.' };
    let list: Announcement[] = [];
    // The answer to the POST waits until the test gives it.
    let answer: () => void = () => undefined;
    const { calls } = mockApiAndRender(() => list, {
      'POST /admin/announcements': () =>
        new Promise<Response>((resolve) => {
          answer = () => {
            list = [created];
            resolve(json(created, 201));
          };
        }),
    });
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neue Ankündigung' }));
    const form = await screen.findByRole('form', { name: 'Neue Ankündigung' });
    const title = within(form).getByRole('textbox', { name: 'Titel' });
    await user.type(title, 'Kinoabend');
    await user.type(within(form).getByRole('textbox', { name: 'Text' }), 'Am Freitag.');
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    await waitFor(() => expect(sent(calls, 'POST')).toBeDefined());
    // «Abbrechen» waits for the answer; typing goes on.
    expect(within(form).getByRole('button', { name: 'Abbrechen' })).toBeDisabled();
    await user.type(title, ' um acht');
    answer();

    const saved = await screen.findByRole('region', { name: 'Kinoabend' });
    expect(within(saved).getByRole('textbox', { name: 'Titel' })).toHaveValue('Kinoabend um acht');
    expect(option(/Kinoabend/)).toHaveTextContent('Ungespeicherte Änderungen');
    expect(sent(calls, 'POST')?.body).toEqual({
      title: 'Kinoabend',
      body: 'Am Freitag.',
      publish: false,
    });
  });

  it('«Neue Ankündigung» waits for the list', async () => {
    let answer: () => void = () => undefined;
    mockApi(
      routes(() => [], {
        'GET /admin/announcements': () =>
          new Promise<Response>((resolve) => {
            answer = () => resolve(json([]));
          }),
      }),
    );
    renderApp('/admin/announcements');
    expect(await screen.findByRole('button', { name: 'Neue Ankündigung' })).toBeDisabled();
    answer();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Neue Ankündigung' })).toBeEnabled(),
    );
  });

  it('publishes a new announcement at once when asked to', async () => {
    const { calls } = mockApiAndRender(() => [], {
      'POST /admin/announcements': () =>
        json({ ...published, title: 'Neu', publishedAt: '2026-10-21T10:00:00Z' }, 201),
    });
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neue Ankündigung' }));
    const form = await screen.findByRole('form', { name: 'Neue Ankündigung' });
    expect(form).toHaveTextContent('Gäste bekommen sie standardmässig per E-Mail.');
    await user.type(within(form).getByRole('textbox', { name: 'Titel' }), 'Neu');
    await user.type(within(form).getByRole('textbox', { name: 'Text' }), 'Hallo');
    await user.click(within(form).getByRole('checkbox', { name: 'Sofort veröffentlichen' }));
    await user.click(within(form).getByRole('button', { name: 'Veröffentlichen' }));
    await waitFor(() => expect(form).not.toBeInTheDocument());
    expect(sent(calls, 'POST')?.body).toEqual({ title: 'Neu', body: 'Hallo', publish: true });
    expect(await screen.findByText('„Neu“ ist veröffentlicht.')).toBeVisible();
  });

  it('the preview shows what is typed, before it is saved', async () => {
    mockApiAndRender(() => [draft]);
    const user = userEvent.setup();
    const region = await screen.findByRole('region', { name: 'Wartung am Samstag' });
    const shown = preview(region);
    expect(within(shown).getByText('Wartung am Samstag')).toBeVisible();
    // An unpublished one shows the day it would go out: today.
    expect(shown).toHaveTextContent(/Veröffentlicht am \d\d\.\d\d\.\d{4}/);

    const title = within(region).getByRole('textbox', { name: 'Titel' });
    await user.clear(title);
    expect(within(shown).getByText('Titel der Ankündigung')).toBeVisible();
    await user.type(title, 'Wartung verschoben');
    const body = within(region).getByRole('textbox', { name: 'Text' });
    await user.clear(body);
    await user.type(body, 'Erster Absatz{Enter}{Enter}Zweiter Absatz');
    expect(within(shown).getByText('Wartung verschoben')).toBeVisible();
    expect(within(shown).getByText('Erster Absatz')).toBeVisible();
    expect(within(shown).getByText('Zweiter Absatz')).toBeVisible();
    // The list marks the draft as changed; the heading still names what is saved.
    expect(option(/Wartung am Samstag/)).toHaveTextContent('Ungespeicherte Änderungen');
  });

  it('keeps what is typed while another announcement is selected; leaving asks first', async () => {
    const { router } = mockApiAndRender(() => [draft, published]);
    const user = userEvent.setup();
    const region = await screen.findByRole('region', { name: 'Wartung am Samstag' });
    await user.type(within(region).getByRole('textbox', { name: 'Titel' }), ' (verschoben)');

    await user.click(option(/Neue Filme/));
    expect(await screen.findByRole('region', { name: 'Neue Filme' })).toBeVisible();
    await user.click(option(/Wartung am Samstag/));
    const back = await screen.findByRole('region', { name: 'Wartung am Samstag' });
    expect(within(back).getByRole('textbox', { name: 'Titel' })).toHaveValue(
      'Wartung am Samstag (verschoben)',
    );

    // A new one keeps its text as well.
    await user.click(screen.getByRole('button', { name: 'Neue Ankündigung' }));
    const form = await screen.findByRole('form', { name: 'Neue Ankündigung' });
    await user.type(within(form).getByRole('textbox', { name: 'Titel' }), 'Kinoabend');
    await user.click(option(/Neue Filme/));
    expect(option(/Kinoabend/)).toHaveTextContent('Nicht gespeichert');
    await user.click(option(/Kinoabend/));
    expect(
      within(await screen.findByRole('form', { name: 'Neue Ankündigung' })).getByRole('textbox', {
        name: 'Titel',
      }),
    ).toHaveValue('Kinoabend');

    void router.navigate({ to: '/admin/jobs' });
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(router.state.location.pathname).toBe('/admin/announcements');
  });

  it('discards changes after a question', async () => {
    mockApiAndRender(() => [draft]);
    const user = userEvent.setup();
    const region = await screen.findByRole('region', { name: 'Wartung am Samstag' });
    const title = within(region).getByRole('textbox', { name: 'Titel' });
    // Blanks around the text are no change.
    await user.type(title, '  ');
    expect(within(region).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    await user.type(title, 'x');
    await user.click(within(region).getByRole('button', { name: 'Verwerfen' }));
    const question = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    expect(within(question).getByRole('button', { name: 'Abbrechen' })).toHaveFocus();
    await user.click(within(question).getByRole('button', { name: 'Verwerfen' }));
    await waitFor(() => expect(title).toHaveValue(draft.title));
    // «Verwerfen» went with the change: the title has the focus.
    await waitFor(() => expect(title).toHaveFocus());
  });

  it('saves a changed draft; a field the server refuses is marked in the form', async () => {
    let list = [draft];
    let answer = problem(422, 'validation_failed', {
      errors: [{ field: 'title', code: 'too_long' }],
    });
    const { calls } = mockApiAndRender(() => list, {
      'PUT /admin/announcements/2': (call) => {
        if (answer.ok) list = [{ ...draft, ...(call.body as Partial<Announcement>) }];
        return answer;
      },
    });
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    const title = within(form).getByRole('textbox', { name: 'Titel' });
    expect(title).toHaveValue(draft.title);
    expect(within(form).getByRole('textbox', { name: 'Text' })).toHaveValue(draft.body);
    expect(within(form).queryByRole('checkbox')).toBeNull();
    expect(form).toHaveTextContent('Danach lässt sie sich nicht mehr ändern.');
    await user.type(title, ' (verschoben)');
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(await within(form).findByText('Zu lang.')).toBeVisible();
    expect(title).toHaveAttribute('aria-invalid', 'true');

    // The mark goes as soon as the field changes; the next save goes through.
    await user.type(title, '!');
    expect(within(form).queryByText('Zu lang.')).toBeNull();
    answer = json({ ...draft, title: 'Wartung am Samstag (verschoben)!' });
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(
      await screen.findByText('„Wartung am Samstag (verschoben)!“ ist als Entwurf gespeichert.'),
    ).toBeVisible();
    expect(calls.filter((call) => call.method === 'PUT').at(-1)?.body).toEqual({
      title: 'Wartung am Samstag (verschoben)!',
      body: draft.body,
    });
  });

  it('a draft published meanwhile: the page says so, drops the change and shows it as sent', async () => {
    let list = [draft];
    mockApiAndRender(() => list, {
      'PUT /admin/announcements/2': () => {
        list = [{ ...draft, publishedAt: '2026-10-21T10:00:00Z' }];
        return problem(409, 'announcement_published');
      },
    });
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    await user.type(within(form).getByRole('textbox', { name: 'Titel' }), ' (verschoben)');
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Die Ankündigung ist schon veröffentlicht und lässt sich nicht mehr ändern.',
    );
    const sentOne = await screen.findByRole('region', { name: 'Wartung am Samstag' });
    await waitFor(() => expect(within(sentOne).queryByRole('form')).toBeNull());
    expect(sentOne).toHaveTextContent('0 von 5 Profilen');
    expect(option(/Wartung am Samstag/)).toHaveTextContent('0 von 5 gelesen');
  });

  it('keeps what is typed while a draft is being saved', async () => {
    let list = [draft];
    let answer: () => void = () => undefined;
    mockApiAndRender(() => list, {
      'PUT /admin/announcements/2': (call) =>
        new Promise<Response>((resolve) => {
          answer = () => {
            list = [{ ...draft, ...(call.body as Partial<Announcement>) }];
            resolve(json(list[0]));
          };
        }),
    });
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    const title = within(form).getByRole('textbox', { name: 'Titel' });
    await user.type(title, ' (verschoben)');
    await user.click(within(form).getByRole('button', { name: 'Als Entwurf speichern' }));
    await user.type(title, ' auf Sonntag');
    answer();
    expect(
      await screen.findByText('„Wartung am Samstag (verschoben)“ ist als Entwurf gespeichert.'),
    ).toBeVisible();
    expect(title).toHaveValue('Wartung am Samstag (verschoben) auf Sonntag');
    expect(option(/Wartung am Samstag/)).toHaveTextContent('Ungespeicherte Änderungen');
  });

  it('saves nothing without a change; the same message again is announced again', async () => {
    let list = [draft];
    const { calls } = mockApiAndRender(() => list, {
      'PUT /admin/announcements/2': (call) => {
        list = [{ ...draft, ...(call.body as Partial<Announcement>) }];
        return json(list[0]);
      },
    });
    const puts = () => calls.filter((call) => call.method === 'PUT').length;
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    const save = within(form).getByRole('button', { name: 'Als Entwurf speichern' });
    const body = within(form).getByRole('textbox', { name: 'Text' });
    await user.click(save);
    expect(puts()).toBe(0);
    expect(save).toHaveFocus();

    await user.type(body, ' A');
    await user.click(save);
    const first = await screen.findByText('„Wartung am Samstag“ ist als Entwurf gespeichert.');
    const status = first.closest<HTMLElement>('[role="status"]')!;
    await user.type(body, 'B');
    await user.click(save);
    await waitFor(() => expect(puts()).toBe(2));
    // The same words, a new element: a screen reader announces them again.
    await waitFor(() =>
      expect(
        within(status).getByText('„Wartung am Samstag“ ist als Entwurf gespeichert.'),
      ).not.toBe(first),
    );
  });

  it('does not refetch on window focus while a draft is changed', async () => {
    const { calls } = mockApiAndRender(() => [draft]);
    const lists = () => calls.filter((call) => call.path === '/admin/announcements').length;
    const refocus = () =>
      act(() => {
        focusManager.setFocused(false);
        focusManager.setFocused(true);
      });
    const user = userEvent.setup();
    try {
      const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
      await user.type(within(form).getByRole('textbox', { name: 'Titel' }), '!');
      const before = lists();
      refocus();
      // Give a refetch the chance it must not take.
      await act(() => new Promise((resolve) => window.setTimeout(resolve, 50)));
      expect(lists()).toBe(before);

      await user.type(within(form).getByRole('textbox', { name: 'Titel' }), '{Backspace}');
      refocus();
      await waitFor(() => expect(lists()).toBeGreaterThan(before));
    } finally {
      focusManager.setFocused(undefined);
    }
  });

  it('an empty title is not published: no question, the form marks it and takes the focus', async () => {
    const { calls } = mockApiAndRender(() => [draft]);
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    const title = within(form).getByRole('textbox', { name: 'Titel' });
    await user.clear(title);
    await user.click(screen.getByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    expect(within(form).getByText('Pflichtfeld.')).toBeVisible();
    expect(title).toHaveFocus();
    expect(calls.some((call) => call.method !== 'GET')).toBe(false);
  });

  it('a refused save before the publication is marked in the form', async () => {
    const { calls } = mockApiAndRender(() => [draft], {
      'PUT /admin/announcements/2': () =>
        problem(422, 'validation_failed', { errors: [{ field: 'body', code: 'too_long' }] }),
    });
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    await user.type(within(form).getByRole('textbox', { name: 'Text' }), ' Mehr.');
    await user.click(screen.getByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Veröffentlichen' }));
    expect(await within(form).findByText('Zu lang.')).toBeVisible();
    expect(within(form).getByRole('textbox', { name: 'Text' })).toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(sent(calls, 'POST')).toBeUndefined();
    expect(screen.queryByRole('alert', { name: /Zu lang/ })).toBeNull();
  });

  it('publishes a draft after the confirmation with what is typed; the focus goes to the heading', async () => {
    let list = [draft];
    // The publication waits until the test answers it.
    let answer: () => void = () => undefined;
    const { calls } = mockApiAndRender(() => list, {
      'PUT /admin/announcements/2': (call) => {
        list = [{ ...draft, ...(call.body as Partial<Announcement>) }];
        return json(list[0]);
      },
      'POST /admin/announcements/2/publish': () =>
        new Promise<Response>((resolve) => {
          answer = () => {
            list = list.map((entry) => ({ ...entry, publishedAt: '2026-10-21T10:00:00Z' }));
            resolve(json(list[0]));
          };
        }),
    });
    const user = userEvent.setup();
    const form = await screen.findByRole('form', { name: 'Entwurf bearbeiten' });
    await user.type(within(form).getByRole('textbox', { name: 'Text' }), ' Danke!');
    await user.click(screen.getByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: '„Wartung am Samstag“ veröffentlichen?',
    });
    expect(dialog).toHaveTextContent('Gäste bekommen sie standardmässig per E-Mail.');
    await user.click(within(dialog).getByRole('button', { name: 'Veröffentlichen' }));
    await waitFor(() => expect(sent(calls, 'POST')?.path).toBe('/admin/announcements/2/publish'));
    // The changed text was saved first.
    expect(sent(calls, 'PUT')?.body).toEqual({ title: draft.title, body: `${draft.body} Danke!` });
    // While it goes out, the text takes no input and «Veröffentlichen» waits.
    expect(within(form).getByRole('textbox', { name: 'Titel' })).toHaveAttribute('readonly');
    expect(within(form).getByRole('textbox', { name: 'Text' })).toHaveAttribute('readonly');
    expect(
      screen.getByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }),
    ).toHaveAttribute('data-pending');
    answer();
    expect(await screen.findByText('„Wartung am Samstag“ ist veröffentlicht.')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('region', { name: 'Wartung am Samstag' })).toHaveTextContent(
        '0 von 5 Profilen',
      ),
    );
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Ankündigungen' })).toHaveFocus(),
    );
  });

  it('deletes an announcement after the confirmation; an older refusal goes away', async () => {
    let list = [draft, published];
    const { calls, router } = mockApiAndRender(() => list, {
      'POST /admin/announcements/2/publish': () => problem(409, 'announcement_published'),
      'DELETE /admin/announcements/1': () => {
        list = [draft];
        return new Response(null, { status: 204 });
      },
    });
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: '„Wartung am Samstag“ veröffentlichen' }),
    );
    const refused = await screen.findByRole('alertdialog', {
      name: '„Wartung am Samstag“ veröffentlichen?',
    });
    await user.click(within(refused).getByRole('button', { name: 'Veröffentlichen' }));
    expect(await screen.findByRole('alert')).toBeVisible();

    await user.click(option(/Neue Filme/));
    await user.click(await screen.findByRole('button', { name: '„Neue Filme“ löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: '„Neue Filme“ löschen?' });
    const confirm = within(dialog).getByRole('button', { name: 'Ankündigung löschen' });
    expect(confirm.className).toMatch(/danger/);
    await user.click(confirm);
    await waitFor(() => expect(sent(calls, 'DELETE')?.path).toBe('/admin/announcements/1'));
    expect(await screen.findByText('„Neue Filme“ ist gelöscht.')).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    await waitFor(() =>
      expect(within(listBox()).queryByRole('option', { name: /Neue Filme/ })).toBeNull(),
    );
    // The address drops the deleted one; the heading has the focus.
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('item'));
    expect(screen.getByRole('region', { name: 'Wartung am Samstag' })).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Ankündigungen' })).toHaveFocus(),
    );
  });

  it('an unknown ?item= shows the first one and leaves the address', async () => {
    mockApi(routes(() => [draft, published]));
    const { router } = renderApp('/admin/announcements?item=announcement-99');
    expect(await screen.findByRole('region', { name: 'Wartung am Samstag' })).toBeVisible();
    await waitFor(() => expect(router.state.location.search).not.toHaveProperty('item'));
  });

  it('a linked ?item= selects that announcement', async () => {
    mockApi(routes(() => [draft, published]));
    renderApp('/admin/announcements?item=announcement-1');
    expect(await screen.findByRole('region', { name: 'Neue Filme' })).toBeVisible();
    expect(option(/Neue Filme/)).toHaveAttribute('aria-selected', 'true');
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'user', isOwner: false } }),
    });
    renderApp('/admin/announcements');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });

  it('the list fails to load: the error offers «Erneut versuchen»', async () => {
    let fail = true;
    mockApi(
      routes(() => [published], {
        'GET /admin/announcements': () => (fail ? problem(500, 'internal') : json([published])),
      }),
    );
    renderApp('/admin/announcements');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('listbox', { name: 'Ankündigungen' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Erneut versuchen' })).not.toBeInTheDocument();
  });
});

/** Mocks the API with `list` and opens the page. */
function mockApiAndRender(list: () => Announcement[], extra: Routes = {}) {
  const { calls } = mockApi(routes(list, extra));
  const { router } = renderApp('/admin/announcements');
  return { calls, router };
}
