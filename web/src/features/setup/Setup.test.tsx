import type {
  AdminSettings,
  Library,
  MetadataSettings,
  SessionInfo,
  SetupProgress,
  SetupStep,
  SystemInfo,
} from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it } from 'vitest';
import i18n, { setLanguage } from '@/i18n';
import { dashboardFixture } from '@/test/admin';
import { ratingSystems } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, type Call, type Routes } from '@/test/api';
import { fill } from '@/test/fill';
import { SETUP_STEPS } from './setupState';

// The guided wizard of concept 22 part A: before an account only steps 1 and 2; the owner of an
// unfinished wizard is led back into it at every sign-in (F-11) and finishes it with a library.

const system: SystemInfo = {
  version: '1.35.0',
  startedAt: '2026-11-15T08:00:00Z',
  timeZone: 'Europe/Zurich',
  gpu: {
    backend: 'software',
    hardwareEncode: false,
    tonemapper: 'cpu',
    hwTonemap: false,
    devices: { dri: false, nvidia: false },
    candidates: [
      { backend: 'nvenc', ok: false },
      { backend: 'qsv', ok: false },
      { backend: 'vaapi', ok: false },
    ],
  },
  inotify: { maxUserWatches: 524288, watchedDirs: 0, usagePercent: 0, warn: false },
  configFs: 'ext4',
  cacheFs: 'ext4',
};

const progress = (
  states: Partial<Record<SetupStep, 'open' | 'done' | 'skipped'>> = {},
): SetupProgress => ({
  steps: SETUP_STEPS.map((step) => ({
    step,
    state: states[step] ?? (step === 'language' || step === 'account' ? 'done' : 'open'),
  })),
  trickplayDefault: false,
  finished: false,
});

const pending: SessionInfo = { ...session, setupPending: true };

const metadata: MetadataSettings = {
  textLangs: ['de', 'en'],
  imageLangs: { poster: ['de', 'en', ''], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
  imageKinds: ['logo'],
  imageMinWidth: { poster: 0, backdrop: 0 },
  imageByVote: true,
  localImages: true,
  ratingCountry: 'DE',
  ratingSystems,
  tmdbIncludeAdult: false,
  fanartEnabled: false,
  knownLanguages: ['de', 'en', 'fr'],
  affectedTitles: 0,
  localImageTitles: 0,
};

const settings: AdminSettings = {
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  transcodeHw: 'auto',
  transcodeHwSource: 'none',
  transcodeHwRestart: false,
  keys: {
    tmdb: { source: 'none' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
    fanart: { source: 'none' },
    fanartClient: { source: 'none' },
    youtube: { source: 'none' },
    trakt: { source: 'none' },
    googleBooks: { source: 'none' },
  },
  email: { source: 'none' },
  telegram: { source: 'none' },
  push: { configured: false },
};

const library: Library = {
  id: 1,
  name: 'Filme',
  kind: 'video',
  roots: [],
  scan: { state: 'running', itemCount: 0, fileCount: 0, filesSeen: 3, filesTotal: 9 },
} as unknown as Library;

/** The owner of an unfinished wizard, with what the steps read. */
const owner = (extra: Routes = {}, states: Parameters<typeof progress>[0] = {}): Routes => ({
  'GET /auth/session': () => json(pending),
  'GET /admin/setup': () => json(progress(states)),
  'PUT /admin/setup': (call: Call) => {
    const body = call.body as { step: SetupStep; state: 'done' | 'skipped' };
    return json(progress({ ...states, [body.step]: body.state }));
  },
  'GET /admin/system': () => json(system),
  'GET /admin/settings/metadata': () => json(metadata),
  'GET /admin/settings': () => json(settings),
  'GET /admin/radio/offer': () =>
    json({ version: 3, hideBroken: false, totals: { offered: 0 }, countries: [] }),
  'GET /admin/libraries': () => json([]),
  'GET /admin/dashboard': () => json(dashboardFixture()),
  ...extra,
});

beforeEach(async () => {
  localStorage.removeItem('slouchcat.setupLater');
  await setLanguage('de');
});

describe('before an account', () => {
  const noAccount: Routes = {
    'GET /auth/session': () => problem(401, 'unauthenticated'),
    'GET /setup': () => json({ setupRequired: true, system }),
  };

  it('leads every path to the language step, which switches the interface at once', async () => {
    mockApi(noAccount);
    const { router } = renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: 'Sprache wählen' })).toHaveFocus();
    expect(router.state.location.pathname).toBe('/setup/language');
    const user = userEvent.setup();
    await user.click(screen.getByRole('radio', { name: /English/ }));
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Choose a language' }),
    ).toBeVisible();
    expect(document.documentElement.lang).toBe('en');
    // The search finds a language by its English name too.
    await fill(user, screen.getByRole('searchbox', { name: 'Search languages' }), 'germany');
    expect(screen.getAllByRole('radio')).toHaveLength(1);
    expect(screen.getByRole('radio', { name: /Deutsch \(Deutschland\)/ })).toBeVisible();
  });

  it('a later step leads to the account; «Weiter» on the account creates it without a library', async () => {
    const { calls } = mockApi({
      ...noAccount,
      'POST /setup': () =>
        json({ ...pending, profile: { ...session.profile, name: 'Marco' } }, 201),
      'GET /admin/setup': () => json(progress()),
      'GET /admin/settings/metadata': () => json(metadata),
      'GET /admin/system': () => json(system),
      'GET /admin/radio/offer': () =>
        json({ version: 0, hideBroken: false, totals: {}, countries: [] }),
    });
    const { router } = renderApp('/setup/region');
    await screen.findByRole('heading', { level: 1, name: 'Sprache wählen' });
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Weiter' }));
    expect(await screen.findByRole('heading', { level: 1, name: 'Konto anlegen' })).toHaveFocus();
    await fill(user, screen.getByLabelText(/^E-Mail-Adresse/), 'marco@example.test');
    await fill(user, screen.getByLabelText(/^Passwort$/), 'long-enough-password');
    await fill(user, screen.getByLabelText(/^Passwort wiederholen/), 'long-enough-password');
    await fill(user, screen.getByLabelText(/^Name des Profils/), 'Marco');
    await user.click(screen.getByRole('button', { name: 'Weiter' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/setup/region'));
    const setup = calls.find((call) => call.method === 'POST' && call.path === '/setup');
    expect(setup?.body).toEqual({
      email: 'marco@example.test',
      password: 'long-enough-password',
      profileName: 'Marco',
      accent: 'ember',
      language: 'de',
    });
  });

  it('says so when somebody else created the account first (3.1)', async () => {
    mockApi({ ...noAccount, 'POST /setup': () => problem(409, 'setup_done') });
    renderApp('/setup/account');
    const user = userEvent.setup();
    await screen.findByRole('heading', { level: 1, name: 'Konto anlegen' });
    await fill(user, screen.getByLabelText(/^E-Mail-Adresse/), 'marco@example.test');
    await fill(user, screen.getByLabelText(/^Passwort$/), 'long-enough-password');
    await fill(user, screen.getByLabelText(/^Passwort wiederholen/), 'long-enough-password');
    await fill(user, screen.getByLabelText(/^Name des Profils/), 'Marco');
    await user.click(screen.getByRole('button', { name: 'Weiter' }));
    expect(await screen.findByText('Slouchcat ist schon eingerichtet')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Anmeldung' })).toHaveAttribute('href', '/login');
  });
});

describe('the owner of an unfinished wizard', () => {
  it('is led to the first open step, and «Später fortsetzen» lets go for this sign-in', async () => {
    mockApi(owner({}, { region: 'done' }));
    const { router } = renderApp('/');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Dienste verbinden' }),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe('/setup/services');
    // The step list names the state of every step.
    const nav = screen.getByRole('navigation', { name: 'Einrichtung' });
    expect(within(nav).getByRole('link', { name: '3. Region, erledigt' })).toBeVisible();
    const user = userEvent.setup();
    // The cards are there: the page is the loaded one, not its placeholder.
    expect(await screen.findByRole('heading', { level: 2, name: 'Empfohlen' })).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Später fortsetzen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    await router.navigate({ to: '/admin' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
    expect(await screen.findByText('Einrichtung nicht abgeschlossen')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Einrichtung fortsetzen' })).toHaveAttribute(
      'href',
      '/setup/services',
    );
  });

  it('«Später fortsetzen» outlives a new tab or a reopened app until the next sign-in', async () => {
    localStorage.setItem('slouchcat.setupLater', String(session.account.id));
    mockApi(owner());
    const { router } = renderApp('/admin');
    expect(await screen.findByText('Einrichtung nicht abgeschlossen')).toBeVisible();
    expect(router.state.location.pathname).toBe('/admin');
  });

  it('is not led into the wizard before choosing the primary profile (review of stage 2)', async () => {
    const other = { ...session.profile, id: 2, name: 'Kind', isPrimary: false };
    const { calls } = mockApi(
      owner({
        'GET /auth/session': () => json({ account: pending.account, setupPending: true }),
        'GET /auth/profiles': () => json([session.profile, other]),
      }),
    );
    const { router } = renderApp('/');
    await waitFor(() => expect(router.state.location.pathname).toBe('/profiles'));
    expect(await screen.findByRole('button', { name: /^Kind/ })).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/setup')).toBe(false);
  });

  it('the overview asks for the wizard only for the owner (review of stage 2)', async () => {
    const admin = { ...pending, account: { ...pending.account, isOwner: false } };
    localStorage.removeItem('slouchcat.setupLater');
    const { calls } = mockApi(owner({ 'GET /auth/session': () => json(admin) }));
    renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    expect(screen.queryByText('Einrichtung nicht abgeschlossen')).toBeNull();
    expect(calls.some((call) => call.path === '/admin/setup')).toBe(false);
  });

  it('stores the region: rating country, languages, the country in the radio without stations', async () => {
    const { calls } = mockApi(
      owner({
        'PUT /admin/settings/metadata': (call: Call) =>
          json({ ...metadata, ...(call.body as object) }),
        'POST /admin/radio/offer/apply': () =>
          json({ offer: { version: 4, countries: [] }, queuedLogos: 0, demarkedStationIds: [] }),
      }),
    );
    const { router } = renderApp('/setup/region');
    await screen.findByRole('heading', { level: 1, name: 'Region und Metadaten' });
    // Swiss German suggests Switzerland, with its steps as the household step offers them.
    const ratingCountry = await screen.findByRole('combobox', { name: 'Altersfreigaben nach' });
    expect(ratingCountry).toHaveValue('CH');
    expect(
      screen.getByText(
        'Altersstufen der Profile: ab 0 · ab 6 · ab 7 · ab 8 · ab 10 · ab 12 · ab 14 · ab 16 · ab 18',
      ),
    ).toBeVisible();
    // Every country of the server's table (concept 22 stage 3), named with its rating body.
    const user = userEvent.setup();
    await user.selectOptions(ratingCountry, 'FR');
    expect(
      within(ratingCountry).getByRole('option', { name: 'Frankreich (CNC)' }),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Altersstufen der Profile: Tous publics · -10 · -12 · -16 · -18'),
    ).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Weiter' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/setup/services'));
    const update = calls.find(
      (call) => call.method === 'PUT' && call.path === '/admin/settings/metadata',
    );
    expect(update?.body).toMatchObject({
      ratingCountry: 'FR',
      textLangs: ['de', 'en'],
      imageLangs: { poster: ['de', 'en', ''], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
    });
    const apply = calls.find((call) => call.path === '/admin/radio/offer/apply');
    expect(apply?.body).toEqual({
      version: 3,
      steps: [
        { op: 'chooseCountry', country: 'CH' },
        { op: 'demarkAll', country: 'CH' },
      ],
    });
    expect(
      calls.find((call) => call.method === 'PUT' && call.path === '/admin/setup')?.body,
    ).toEqual({
      step: 'region',
      state: 'done',
    });
  });

  // Table 4.1: every language suggests its country and itself, then English, as metadata
  // languages; a country without a rating system (China) rates by Germany (F-12).
  it.each([
    ['fr', 'FR', 'FR', ['fr', 'en']],
    ['zh-CN', 'CN', 'DE', ['zh-CN', 'en']],
  ] as const)(
    'suggests by the language %s: %s, ratings of %s, %j',
    async (language, country, rating, textLangs) => {
      const { calls } = mockApi(
        owner({
          'GET /auth/session': () =>
            json({ ...pending, profile: { ...pending.profile, language } }),
          'PUT /admin/settings/metadata': (call: Call) =>
            json({ ...metadata, ...(call.body as object) }),
          'POST /admin/radio/offer/apply': () =>
            json({ offer: { version: 4, countries: [] }, queuedLogos: 0, demarkedStationIds: [] }),
        }),
      );
      renderApp('/setup/region');
      await waitFor(() => expect(document.documentElement.lang).toBe(language));
      const [countrySelect, ratingSelect] = await screen.findAllByRole('combobox');
      expect(countrySelect).toHaveValue(country);
      expect(ratingSelect).toHaveValue(rating);
      // The suggestion is what the step stores.
      await userEvent
        .setup()
        .click(screen.getByRole('button', { name: i18n.t('setup.actions.next') }));
      await waitFor(() =>
        expect(
          calls.find((call) => call.method === 'PUT' && call.path === '/admin/settings/metadata')
            ?.body,
        ).toMatchObject({ ratingCountry: rating, textLangs }),
      );
    },
  );

  it('cannot leave the libraries without one', async () => {
    const { calls } = mockApi(owner());
    renderApp('/setup/libraries');
    await screen.findByRole('heading', { level: 1, name: 'Bibliotheken' });
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Weiter' }));
    expect(screen.queryByRole('button', { name: 'Überspringen' })).toBeNull();
    expect(calls.some((call) => call.method === 'PUT' && call.path === '/admin/setup')).toBe(false);
    expect(screen.getByRole('button', { name: 'Bibliothek hinzufügen' })).toBeVisible();
  });

  it('offers only the probed backends and stores the choice (F-10)', async () => {
    const { calls } = mockApi(
      owner({
        'PUT /admin/settings': () =>
          json({ ...settings, transcodeHw: 'software', transcodeHwSource: 'stored' }),
      }),
    );
    renderApp('/setup/playback');
    await screen.findByRole('heading', { level: 1, name: 'Wiedergabe' });
    const group = await screen.findByRole('radiogroup', { name: 'Grafikchip für das Umwandeln' });
    expect(within(group).getByRole('radio', { name: 'Automatisch (empfohlen)' })).toBeChecked();
    expect(within(group).getByRole('radio', { name: /^VA-API – Probe/ })).toBeDisabled();
    const user = userEvent.setup();
    await user.click(within(group).getByRole('radio', { name: 'Nur Prozessor' }));
    await waitFor(() =>
      expect(calls.some((call) => call.path === '/admin/settings' && call.method !== 'GET')).toBe(
        true,
      ),
    );
    expect(
      calls.find((call) => call.path === '/admin/settings' && call.method !== 'GET')?.body,
    ).toEqual({
      transcodeHw: 'software',
    });
  });

  it('finishes from the summary and shows the first scans', async () => {
    const { calls } = mockApi(
      owner({
        'POST /admin/setup/finish': () => json({ restart: 'none' }),
        'GET /admin/libraries': () => json([library]),
        'GET /profiles': () => json([session.profile]),
      }),
    );
    renderApp('/setup/summary');
    await screen.findByRole('heading', { level: 1, name: 'Zusammenfassung' });
    expect(screen.getByRole('link', { name: 'Region ändern' })).toHaveAttribute(
      'href',
      '/setup/region',
    );
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));
    expect(
      await screen.findByRole('heading', { name: 'Slouchcat ist eingerichtet' }),
    ).toHaveFocus();
    expect(await screen.findByText(/von 9 Dateien/)).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/setup/finish')).toBe(true);
  });
});
