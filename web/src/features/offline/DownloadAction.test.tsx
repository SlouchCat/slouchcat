import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { formatBytes } from '@/lib/format';
import { json, mockApi, renderApp, signedIn } from '@/test/api';
import { alienDetail, episodes, ids, season1Detail, seriesIds } from '@/test/fixtures';
import { resetOfflineFiles } from '@/test/offlineFiles';
import { downloadIds, downloadOptions, downloadState } from '@/test/offlineFixtures';
import { resetOfflineStore, storedDownloads } from '@/test/offlineStore';

vi.mock('@/features/offline/store', () => import('@/test/offlineStore'));
vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'));
const engine = vi.hoisted(() => ({ kick: vi.fn() }));
vi.mock('@/features/offline/engine', () => ({
  startOfflineEngine: () => () => undefined,
  kickOfflineEngine: engine.kick,
  cancelFetch: vi.fn(() => Promise.resolve()),
  rememberDeleted: vi.fn(() => Promise.resolve()),
  flushEventsSoon: vi.fn(),
}));
vi.mock('@/features/offline/support', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./support')>()),
  offlineSupported: () => true,
}));

let persist: ReturnType<typeof vi.fn>;

beforeEach(() => {
  resetOfflineStore();
  resetOfflineFiles();
  engine.kick.mockClear();
  persist = vi.fn(() => Promise.resolve(true));
  Object.defineProperty(navigator, 'storage', {
    configurable: true,
    value: {
      persist,
      persisted: () => Promise.resolve(false),
      estimate: () => Promise.resolve({ usage: 0, quota: 1e10 }),
    },
  });
});

afterEach(() => vi.restoreAllMocks());

const size = (bytes: number) => formatBytes(bytes, 'de');

describe('download on the detail page', () => {
  it('offers the three profiles with their size and the audio track, and creates the download', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
      [`GET /items/${ids.alien}/download-options`]: () => json(downloadOptions),
      [`POST /items/${ids.alien}/downloads`]: () =>
        json({ downloads: [downloadState({ state: 'queued', sizeBytes: undefined })] }, 201),
    });
    renderApp(`/items/${ids.alien}`);
    await user.click(await screen.findByRole('button', { name: 'Herunterladen' }));

    const dialog = await screen.findByRole('dialog', { name: 'Alien herunterladen' });
    const quality = await within(dialog).findByRole('radiogroup', { name: 'Qualität' });
    expect(
      within(quality)
        .getAllByRole('radio')
        .map((radio) => radio.closest('label')?.textContent),
    ).toEqual([
      `Hoch · 1080p · ca. ${size(5_480_000_000)}`,
      `Mittel · 720p · ca. ${size(2_800_000_000)}`,
      `Niedrig · 480p · ca. ${size(1_140_000_000)}`,
    ]);
    expect(within(quality).getByRole('radio', { name: /^Mittel/ })).toBeChecked();
    const audio = within(dialog).getByRole('radiogroup', { name: 'Tonspur' });
    expect(within(audio).getAllByRole('radio')).toHaveLength(2);
    expect(within(dialog).getByText(/Untertitel: Deutsch \(SRT\)/)).toBeVisible();

    await user.click(within(audio).getByText(/Englisch/));
    await user.click(within(dialog).getByRole('button', { name: 'Herunterladen' }));

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      quality: 'medium',
      audioTrack: 2,
    });
    // The first download asks the browser to keep the storage.
    expect(persist).toHaveBeenCalledTimes(1);
    expect(engine.kick).toHaveBeenCalled();
    expect(storedDownloads()).toEqual([
      expect.objectContaining({
        id: downloadIds.alien,
        accountId: 1,
        state: 'queued',
        bytesWritten: 0,
      }),
    ]);
    // The title now shows its state and leads to the downloads.
    expect(await screen.findByRole('link', { name: 'Zu den Downloads' })).toHaveAttribute(
      'href',
      '/downloads',
    );
    expect(screen.getByText('Wartet auf den Server')).toBeVisible();
  });

  it('downloads a whole season in one action (O-2)', async () => {
    const user = userEvent.setup();
    const created = [episodes.e1, episodes.e2, episodes.e34].map((episode, index) =>
      downloadState({
        id: `01JB8Z6Q4M7N9P2R3S5T7V9E0${index}`,
        state: 'queued',
        item: episode,
      }),
    );
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
      [`GET /items/${seriesIds.season1}/download-options`]: () =>
        json({
          ...downloadOptions,
          mediaFileId: undefined,
          episodeCount: 3,
          audioTracks: [],
          subtitles: [],
        }),
      [`POST /items/${seriesIds.season1}/downloads`]: () => json({ downloads: created }, 201),
    });
    renderApp(`/items/${seriesIds.season1}`);
    await user.click(await screen.findByRole('button', { name: 'Staffel herunterladen' }));
    const dialog = await screen.findByRole('dialog', { name: 'Season 1 herunterladen' });
    expect(
      await within(dialog).findByText(
        'Alle 3 Folgen dieser Staffel werden auf diesem Gerät gespeichert.',
      ),
    ).toBeVisible();
    await within(dialog).findByRole('radiogroup', { name: 'Qualität' });
    expect(within(dialog).queryByRole('radiogroup', { name: 'Tonspur' })).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Herunterladen' }));
    await waitFor(() => expect(storedDownloads()).toHaveLength(3));
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({ quality: 'medium' });
    expect(await screen.findByText('0 von 3 Folgen auf diesem Gerät')).toBeVisible();
  });

  it('is not offered for a series itself', async () => {
    const { showDetail } = await import('@/test/fixtures');
    mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () => json(showDetail),
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
    });
    renderApp(`/items/${seriesIds.show}`);
    // The season of the series page has its own button below the episodes.
    expect(await screen.findByRole('button', { name: 'Staffel herunterladen' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Herunterladen' })).not.toBeInTheDocument();
  });
});
