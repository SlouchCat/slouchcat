import type { AdminItem } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

const item: AdminItem = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  libraryId: 1,
  metadataState: 'unmatched',
  metadataError: 'no_match',
  lockedFields: ['rating'],
  ownRating: 16,
  effectiveRating: 16,
  ratingSource: 'manual',
  trailerState: 'none',
  trailers: [
    {
      id: 5,
      lang: 'de',
      youtubeId: 'abcdefghijk',
      embedUrl: 'https://www.youtube-nocookie.com/embed/abcdefghijk',
      source: 'manual',
      official: false,
    },
  ],
  mediaFiles: [
    {
      id: 'mf-1',
      path: 'Alien (1979)/Alien.mkv',
      sizeBytes: 4_200_000_000,
      available: true,
      versionLabel: '1080p',
      probedAt: '2026-10-18T18:00:00Z',
    },
    {
      id: 'mf-2',
      path: 'Alien (1979)/Alien - broken.mkv',
      sizeBytes: 12,
      available: true,
      probedAt: '2026-10-18T18:00:00Z',
      probeError: 'invalid data found when processing input',
    },
  ],
  children: { seasons: 0, episodes: 0, extras: 2 },
};

const base = `/admin/items/${ids.alien}`;

describe('admin item page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows state, ids, rating, files and trailers of the title', async () => {
    mockApi({ ...signedIn, [`GET ${base}`]: () => json(item) });
    renderApp(base);

    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(screen.getByText('Nicht zugeordnet')).toBeVisible();
    expect(screen.getByText('no_match')).toBeVisible();
    expect(screen.getByText('Staffeln: 0 · Folgen: 0 · Extras: 2')).toBeVisible();
    const files = screen.getByRole('region', { name: 'Dateien' });
    expect(within(files).getByText('Alien (1979)/Alien.mkv')).toBeVisible();
    expect(within(files).getByText('invalid data found when processing input')).toBeVisible();
    const trailers = screen.getByRole('region', { name: 'Trailer' });
    expect(within(trailers).getByText('Keiner gefunden')).toBeVisible();
    expect(within(trailers).getByText('abcdefghijk')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Detailseite' })).toHaveAttribute(
      'href',
      `/items/${ids.alien}`,
    );
  });

  it('refreshes the title', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      [`POST ${base}/refresh`]: () => json({ ...item, metadataState: 'pending' }, 202),
    });
    renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Aktualisieren' }));
    await waitFor(() => expect(calls.some((call) => call.path === `${base}/refresh`)).toBe(true));
    expect(await screen.findByText('Aktualisierung eingereiht.')).toBeVisible();
  });

  it('sets the own rating back to the metadata rating and confirms it', async () => {
    const user = userEvent.setup();
    let stored: AdminItem = item;
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(stored),
      [`PUT ${base}/rating`]: () => {
        stored = { ...item, ownRating: undefined, effectiveRating: 18, ratingSource: 'missing' };
        return json({ id: item.id, kind: 'movie', title: 'Alien', effectiveRating: 18 });
      },
    });
    renderApp(base);
    const rating = await screen.findByRole('region', { name: 'Freigabe' });
    expect(within(rating).getByRole('radio', { name: 'FSK 16' })).toBeChecked();
    await user.click(within(rating).getByRole('radio', { name: 'Aus Metadaten' }));
    await user.click(within(rating).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/rating`)?.body).toEqual({ rating: null }),
    );
    // The panel starts anew from the stored value (source "missing"); the confirmation stays.
    await waitFor(() =>
      expect(
        within(screen.getByRole('region', { name: 'Freigabe' })).getByText('missing'),
      ).toBeVisible(),
    );
    expect(screen.getByText('Freigabe gespeichert.')).toBeVisible();
  });

  it('adds a manual trailer from a URL and deletes one after confirmation', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      [`POST ${base}/trailers`]: () => json(item.trailers[0], 201),
      [`DELETE ${base}/trailers/5`]: () => new Response(null, { status: 204 }),
    });
    renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Trailer hinzufügen' });
    const source = within(dialog).getByLabelText('YouTube-Link oder Video-ID');
    expect(source).toHaveFocus();
    await user.type(source, 'https://youtu.be/dQw4w9WgXcQ');
    await user.click(within(dialog).getByRole('radio', { name: 'Andere' }));
    await user.type(within(dialog).getByLabelText('Sprachcode'), 'fr');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/trailers`)?.body).toEqual({
        lang: 'fr',
        url: 'https://youtu.be/dQw4w9WgXcQ',
      }),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    await user.click(screen.getByRole('button', { name: 'Löschen' }));
    const confirm = await screen.findByRole('alertdialog', { name: 'Trailer löschen?' });
    await user.click(within(confirm).getByRole('button', { name: 'Löschen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === `${base}/trailers/5`),
      ).toBe(true),
    );
  });

  it('sends a bare video id as youtubeId and shows the field error of the server', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      [`POST ${base}/trailers`]: () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'youtubeId', code: 'invalid_value' }],
        }),
    });
    renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Trailer hinzufügen' });
    await user.type(within(dialog).getByLabelText('YouTube-Link oder Video-ID'), 'dQw4w9WgXcQ');
    await user.click(within(dialog).getByRole('button', { name: 'Hinzufügen' }));
    await waitFor(() =>
      expect(calls.find((call) => call.path === `${base}/trailers`)?.body).toEqual({
        lang: 'de',
        youtubeId: 'dQw4w9WgXcQ',
      }),
    );
    expect(await within(dialog).findByText('Ungültiger Wert.')).toBeVisible();
  });

  it('opens the match dialog for films and series only', async () => {
    const user = userEvent.setup();
    mockApi({
      ...signedIn,
      [`GET ${base}`]: () => json(item),
      'GET /admin/metadata/search': () => problem(422, 'provider_not_configured'),
    });
    const { unmount } = renderApp(base);
    await user.click(await screen.findByRole('button', { name: 'Zuordnen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Titel zuordnen' });
    await user.click(within(dialog).getByRole('radio', { name: 'TVDB' }));
    // TVDB knows series only.
    expect(within(dialog).getByRole('radio', { name: 'Serie' })).toBeChecked();
    expect(within(dialog).getByRole('radio', { name: 'Film' })).toBeDisabled();
    await user.click(within(dialog).getByRole('button', { name: 'Suchen' }));
    expect(await within(dialog).findByText(/Dieser Anbieter ist nicht eingerichtet/)).toBeVisible();
    unmount();

    mockApi({ ...signedIn, [`GET ${base}`]: () => json({ ...item, kind: 'episode' }) });
    renderApp(base);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(screen.queryByRole('button', { name: 'Zuordnen' })).not.toBeInTheDocument();
  });

  it('answers an unknown title and a profile without rights', async () => {
    mockApi({ ...signedIn, [`GET ${base}`]: () => problem(404, 'not_found') });
    const { unmount } = renderApp(base);
    expect(await screen.findByText('Diesen Titel gibt es nicht oder nicht mehr.')).toBeVisible();
    unmount();

    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
    });
    renderApp(base);
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
  });
});
