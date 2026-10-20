import { onlineManager } from '@tanstack/react-query';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mockApi, renderApp } from '@/test/api';
import { resetOfflineFiles } from '@/test/offlineFiles';
import {
  downloadIds,
  episodeDownload,
  heatDownload,
  offlineProfile,
  storedDownload,
  storedProfile,
} from '@/test/offlineFixtures';
import { putDownloads, putProfiles, resetOfflineStore } from '@/test/offlineStore';
import { leaveOfflineProfile } from './offlineSession';
import { deriveLocalVerifier } from './pin';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: vi.fn(),
  cancelFetch: vi.fn(),
  flushEventsSoon: vi.fn(),
}));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

const SALT = '0123456789abcdef0123456789abcdef';

beforeEach(async () => {
  resetOfflineStore();
  resetOfflineFiles();
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
  onlineManager.setOnline(false);
  await putDownloads([
    storedDownload(),
    heatDownload(),
    episodeDownload('e1', { complete: false, bytesWritten: 4500, allowedProfileIds: [1, 2] }),
  ]);
  await putProfiles([
    storedProfile(),
    storedProfile({
      id: 2,
      snapshot: offlineProfile({
        id: 2,
        name: 'Lea',
        isPrimary: false,
        hasPin: true,
        pinSalt: SALT,
        pinIterations: 10,
      }),
      pinVerifier: await deriveLocalVerifier('4711', SALT, 10),
    }),
    storedProfile({
      id: 3,
      snapshot: offlineProfile({ id: 3, name: 'Ohne Downloads', isPrimary: false }),
    }),
  ]);
});

afterEach(() => {
  leaveOfflineProfile();
  vi.restoreAllMocks();
  onlineManager.setOnline(true);
});

describe('offline library', () => {
  it('opens without the server and lists the profiles that may play a download here', async () => {
    const { calls } = mockApi({});
    renderApp('/offline');
    const profiles = await screen.findByRole('list', { name: 'Profile' });
    expect(
      within(profiles)
        .getAllByRole('button')
        .map((tile) => tile.textContent),
    ).toEqual([expect.stringContaining('Marco'), expect.stringContaining('Lea')]);
    expect(screen.getByText('Offline')).toBeVisible();
    expect(calls).toEqual([]);
  });

  it('a profile without PIN opens at once and shows only its downloads', async () => {
    const user = userEvent.setup();
    mockApi({});
    renderApp('/offline');
    await user.click(await screen.findByRole('button', { name: /Marco/ }));
    expect(await screen.findByRole('heading', { name: 'Downloads von Marco' })).toBeVisible();
    const list = screen.getByRole('list', { name: 'Heruntergeladene Titel' });
    const cards = within(list).getAllByRole('link');
    expect(cards).toHaveLength(1);
    expect(cards[0]).toHaveAttribute('href', `/offline/play/${downloadIds.alien}`);
    expect(cards[0]).toHaveTextContent('Alien');
    // Heat is only allowed for Lea; the episode is not complete yet.
    expect(screen.queryByText('Heat')).not.toBeInTheDocument();
    const pending = screen.getByRole('region', { name: 'Noch nicht vollständig' });
    expect(within(pending).getByText('S01E01 · Pilot · Slouchcat Test Show')).toBeVisible();
    // The first title takes the focus.
    await waitFor(() => expect(cards[0]).toHaveFocus());

    await user.click(screen.getByRole('button', { name: 'Profil wechseln' }));
    expect(await screen.findByRole('list', { name: 'Profile' })).toBeVisible();
  });

  it('checks a PIN against the device’s verifier and brakes wrong ones', async () => {
    const user = userEvent.setup();
    mockApi({});
    renderApp('/offline');
    await user.click(await screen.findByRole('button', { name: /Lea/ }));
    const dialog = await screen.findByRole('dialog', { name: 'PIN für Lea' });
    const field = within(dialog).getByLabelText(/^PIN/);
    await user.type(field, '0000');
    await user.click(within(dialog).getByRole('button', { name: 'Öffnen' }));
    expect(
      await within(dialog).findByText('Die PIN stimmt nicht. Versuche es noch einmal.'),
    ).toBeVisible();
    await user.type(field, '4711');
    await user.click(within(dialog).getByRole('button', { name: 'Öffnen' }));
    expect(await screen.findByRole('heading', { name: 'Downloads von Lea' })).toBeVisible();
    const list = screen.getByRole('list', { name: 'Heruntergeladene Titel' });
    expect(within(list).getByRole('link')).toHaveTextContent('Heat');
  });

  it('locks a profile with PIN but without a verifier on this device', async () => {
    const user = userEvent.setup();
    await putProfiles([
      storedProfile({
        snapshot: offlineProfile({ hasPin: true, pinSalt: SALT }),
      }),
    ]);
    mockApi({});
    renderApp('/offline');
    await user.click(await screen.findByRole('button', { name: /Marco/ }));
    const dialog = await screen.findByRole('dialog', { name: 'PIN für Marco' });
    expect(within(dialog).getByText('Offline gesperrt')).toBeVisible();
    expect(within(dialog).queryByLabelText(/^PIN/)).not.toBeInTheDocument();
  });

  it('Escape leads back from the downloads to the profiles', async () => {
    const user = userEvent.setup();
    mockApi({});
    renderApp('/offline');
    await user.click(await screen.findByRole('button', { name: /Marco/ }));
    await screen.findByRole('heading', { name: 'Downloads von Marco' });
    await user.keyboard('{Escape}');
    expect(await screen.findByRole('list', { name: 'Profile' })).toBeVisible();
  });

  it('explains an empty device', async () => {
    resetOfflineStore();
    mockApi({});
    renderApp('/offline');
    expect(await screen.findByText('Keine Downloads auf diesem Gerät')).toBeVisible();
  });
});
