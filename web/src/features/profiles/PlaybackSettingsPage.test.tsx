import type { PlaybackPrefs } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

const stored: PlaybackPrefs = {
  audioLanguages: ['de'],
  subtitleMode: 'forced',
  subtitleLanguages: ['de'],
};

const audioList = () => screen.getByRole('group', { name: 'Tonsprachen' });
const subtitleList = () => screen.getByRole('group', { name: 'Untertitelsprachen' });

describe('«Einstellungen» (concept 13 11.4; concept 20, P-13)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('changes the audio languages, the subtitle rule and the subtitle languages and saves them together', async () => {
    const user = userEvent.setup();
    let prefs = stored;
    const { calls } = mockApi({
      ...signedIn,
      'GET /me/playback-prefs': () => json(prefs),
      'PUT /me/playback-prefs': (call) => {
        prefs = call.body as PlaybackPrefs;
        return json(prefs);
      },
    });
    renderApp('/settings/playback');
    expect(await screen.findByRole('heading', { level: 1, name: 'Einstellungen' })).toBeVisible();
    await screen.findByRole('group', { name: 'Tonsprachen' });
    expect(within(audioList()).getByLabelText('1. Tonsprache')).toHaveValue('de');

    // A second audio language: the add button appends a place, which gets the focus.
    await user.click(within(audioList()).getByRole('button', { name: 'Weitere Tonsprache' }));
    const second = within(audioList()).getByLabelText('2. Tonsprache');
    expect(second).toHaveFocus();
    await user.selectOptions(second, 'en');
    // Back on the place later, picking German there swaps the two places.
    await user.tab();
    await user.selectOptions(second, 'de');
    expect(within(audioList()).getByLabelText('1. Tonsprache')).toHaveValue('en');
    expect(second).toHaveValue('de');

    // The rule explains itself; «Immer» shows its own text.
    const rule = screen.getByRole('radiogroup', { name: 'Untertitel' });
    expect(within(rule).getByRole('radio', { name: 'Nur erzwungene (Standard)' })).toBeChecked();
    expect(screen.getByText(/Nur Stellen in einer anderen Sprache/)).toBeVisible();
    await user.click(within(rule).getByRole('radio', { name: 'Immer' }));
    expect(
      screen.getByText(/Immer volle Untertitel in der ersten Untertitelsprache/),
    ).toBeVisible();

    await user.click(
      within(subtitleList()).getByRole('button', { name: 'Weitere Untertitelsprache' }),
    );
    await user.selectOptions(within(subtitleList()).getByLabelText('2. Untertitelsprache'), 'fr');

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        audioLanguages: ['en', 'de'],
        subtitleMode: 'always',
        subtitleLanguages: ['de', 'fr'],
      }),
    );
    expect(await screen.findByText('Gespeichert.')).toBeVisible();
    // The stored lists stay on the page.
    expect(within(audioList()).getByLabelText('1. Tonsprache')).toHaveValue('en');
  });

  it('removes a language and says when a list is empty or full', async () => {
    const user = userEvent.setup();
    const ten = ['de', 'en', 'fr', 'it', 'es', 'pt', 'nl', 'ja', 'ko', 'zh'];
    mockApi({
      ...signedIn,
      'GET /me/playback-prefs': () =>
        json({ audioLanguages: ten, subtitleMode: 'off', subtitleLanguages: [] }),
    });
    renderApp('/settings/playback');
    const list = await screen.findByRole('group', { name: 'Tonsprachen' });
    // Ten languages: no new one until a place is free; the places can still swap.
    expect(within(list).getByRole('button', { name: 'Weitere Tonsprache' })).toBeDisabled();
    expect(within(list).getByText(/Höchstens 10 Sprachen/)).toBeVisible();
    expect(
      within(within(list).getByLabelText('1. Tonsprache')).getAllByRole('option'),
    ).toHaveLength(10);
    await user.click(
      within(list).getByRole('button', { name: '10. Tonsprache (Chinesisch) entfernen' }),
    );
    expect(within(list).getByRole('button', { name: 'Weitere Tonsprache' })).toBeEnabled();
    expect(within(subtitleList()).getByText(/Es kommen nur erzwungene Untertitel/)).toBeVisible();
    expect(screen.getByRole('radio', { name: 'Aus' })).toBeChecked();
  });

  it('empties the audio languages: «Originalton immer» (final review stage 2, G3-M1)', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      'GET /me/playback-prefs': () =>
        json({ audioLanguages: ['de'], subtitleMode: 'forced', subtitleLanguages: ['de'] }),
      'PUT /me/playback-prefs': (call) => json(call.body),
    });
    renderApp('/settings/playback');
    const list = await screen.findByRole('group', { name: 'Tonsprachen' });
    await user.click(
      within(list).getByRole('button', { name: '1. Tonsprache (Deutsch) entfernen' }),
    );
    expect(within(list).queryByRole('combobox')).toBeNull();
    expect(within(list).getByRole('button', { name: 'Weitere Tonsprache' })).toHaveFocus();
    expect(within(list).getByText(/Es spielt die Originalsprache/)).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
        audioLanguages: [],
        subtitleMode: 'forced',
        subtitleLanguages: ['de'],
      }),
    );
  });

  it('shows what the server refuses', async () => {
    const user = userEvent.setup();
    mockApi({
      ...signedIn,
      'GET /me/playback-prefs': () => json(stored),
      'PUT /me/playback-prefs': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'audioLanguages', code: 'invalid_value' }],
        }),
    });
    renderApp('/settings/playback');
    await user.click(await screen.findByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Bitte prüfe die markierten Felder.')).toBeVisible();
    // The message stands at the list it is about.
    expect(within(audioList()).getByText('Ungültiger Wert.')).toBeVisible();
    expect(screen.queryByText('Gespeichert.')).toBeNull();
  });

  it('resets the series memory after a confirmation', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      'GET /me/playback-prefs': () => json(stored),
      'DELETE /me/playback-prefs/series': () => new Response(null, { status: 204 }),
    });
    renderApp('/settings/playback');
    await user.click(await screen.findByRole('button', { name: 'Serienmerker zurücksetzen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Serienmerker zurücksetzen?' });
    // Cancel sends nothing.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    await user.click(screen.getByRole('button', { name: 'Serienmerker zurücksetzen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Zurücksetzen' }),
    );
    expect(await screen.findByText('Serienmerker zurückgesetzt.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'DELETE').map((call) => call.path)).toEqual([
      '/me/playback-prefs/series',
    ]);
  });

  it('links to the notification channels in a third field (concept 20, O-8)', async () => {
    mockApi({ ...signedIn, 'GET /me/playback-prefs': () => json(stored) });
    renderApp('/settings/playback');
    const field = await screen.findByRole('region', { name: 'Benachrichtigungen' });
    expect(within(field).getByText(/für das ganze Konto auf einer eigenen Seite/)).toBeVisible();
    expect(
      within(field).getByRole('link', { name: 'Zu den Benachrichtigungskanälen' }),
    ).toHaveAttribute('href', '/settings/notifications');
  });

  it('is in the sidebar of every profile, not only the primary one', async () => {
    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, isPrimary: false } }),
      'GET /me/playback-prefs': () => json(stored),
    });
    renderApp('/settings/playback');
    const link = await screen.findByRole('link', { name: 'Einstellungen' });
    expect(link).toHaveAttribute('aria-current', 'page');
    expect(screen.queryByRole('link', { name: 'Profile' })).toBeNull();
  });
});
