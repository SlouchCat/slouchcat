import { QueryObserver } from '@tanstack/react-query';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries, profiles } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Routes } from '@/test/api';
import { myPlaybackPrefsQuery } from './playbackPrefs';

// The playback preferences in the profile form of the primary profile (concept 13 11.4): the same
// fields as «Einstellungen», for any profile of the account.

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /profiles': () => json(profiles),
  'GET /libraries': () => json(accountLibraries),
  ...extra,
});

describe('playback preferences in the profile form', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('edits the lists and the subtitle rule of another profile', async () => {
    const { calls } = mockApi(routes({ 'PATCH /profiles/2': () => json(profiles[1]) }));
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    const lena = await screen.findByRole('article', { name: 'Lena' });
    await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });

    const audio = within(dialog).getByRole('group', { name: 'Tonsprachen' });
    expect(within(audio).getByLabelText('1. Tonsprache')).toHaveValue('en');
    await user.click(within(audio).getByRole('button', { name: 'Weitere Tonsprache' }));
    await user.selectOptions(within(audio).getByLabelText('2. Tonsprache'), 'ja');
    const rule = within(dialog).getByRole('radiogroup', { name: 'Untertitel' });
    await user.click(within(rule).getByRole('radio', { name: 'Bei fremder Sprache' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
      audioLanguages: ['en', 'ja'],
      subtitleMode: 'foreign',
      subtitleLanguages: ['en'],
    });
  });

  it('reads the active profile’s preferences again after its own form saved (final review stage 2, G3-m5)', async () => {
    const { calls } = mockApi(
      routes({
        'PATCH /profiles/1': () => json(profiles[0]),
        'GET /me/playback-prefs': () =>
          json({ audioLanguages: ['en'], subtitleMode: 'forced', subtitleLanguages: ['de'] }),
      }),
    );
    const { queryClient } = renderApp('/settings/profiles');
    // «Einstellungen» or the sound panel holds the lists of before.
    queryClient.setQueryData(myPlaybackPrefsQuery.queryKey, {
      audioLanguages: ['de'],
      subtitleMode: 'forced',
      subtitleLanguages: ['de'],
    });
    const observer = new QueryObserver(queryClient, {
      ...myPlaybackPrefsQuery,
      staleTime: Infinity,
    });
    const unsubscribe = observer.subscribe(() => undefined);
    const user = userEvent.setup();
    const marco = await screen.findByRole('article', { name: 'Marco' });
    await user.click(within(marco).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'GET' && call.path === '/me/playback-prefs'),
      ).toBe(true),
    );
    unsubscribe();
  });

  it('leaves a new profile’s preferences to the server until they change; the fields follow the language', async () => {
    const { calls } = mockApi(routes({ 'POST /profiles': () => json(profiles[1], 201) }));
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Profil hinzufügen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Neues Profil' });
    await user.type(within(dialog).getByLabelText('Name'), 'Gast');
    await user.click(within(dialog).getByRole('radio', { name: 'English' }));
    const audio = within(dialog).getByRole('group', { name: 'Tonsprachen' });
    // The server starts both lists from the profile's language (F-12); the form shows that.
    expect(within(audio).getByLabelText('1. Tonsprache')).toHaveValue('en');
    await user.click(within(dialog).getByRole('button', { name: 'Profil anlegen' }));

    await waitFor(() => expect(calls.some((call) => call.method === 'POST')).toBe(true));
    const body = calls.find((call) => call.method === 'POST')?.body as Record<string, unknown>;
    expect(body.language).toBe('en');
    expect(body).not.toHaveProperty('audioLanguages');
    expect(body).not.toHaveProperty('subtitleMode');
  });
});
