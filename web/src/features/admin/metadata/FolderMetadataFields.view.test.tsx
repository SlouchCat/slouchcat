import type { MetadataProviderInfo, MetadataSettings } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, type Routes } from '@/test/api';
import { ratingSystems } from '@/test/fixtures';
import { FolderMetadataFields, type FolderMetadataDraft } from './FolderMetadataFields';

// «Sprachen und Bilder eines Ordners» (concept 24 4.2) as the folder card shows it: the default
// beside «Wie Standard», the hint which source does not know a language of the folder's own list.

const defaults: MetadataSettings = {
  textLangs: ['de', 'en'],
  imageLangs: { poster: ['de', 'en', ''], backdrop: ['', 'de', 'en'], logo: ['de', 'en'] },
  imageKinds: ['logo', 'season_poster', 'episode_still'],
  imageMinWidth: { poster: 500, backdrop: 1280 },
  imageByVote: true,
  localImages: true,
  ratingCountry: 'DE',
  ratingSystems,
  tmdbIncludeAdult: false,
  fanartEnabled: false,
  knownLanguages: ['de', 'en', 'es', 'fr', 'ja'],
  affectedTitles: 1234,
  localImageTitles: 211,
};

const tmdb = (languages: string[]): MetadataProviderInfo => ({
  id: 'tmdb',
  configured: true,
  roles: ['identify', 'fill'],
  contents: ['movie', 'series'],
  languages,
});

function renderFields(value: FolderMetadataDraft) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = (draft: FolderMetadataDraft) => (
    <QueryClientProvider client={queryClient}>
      <FolderMetadataFields value={draft} onChange={() => undefined} name="folder" />
    </QueryClientProvider>
  );
  const result = render(view(value));
  return { ...result, rerenderWith: (draft: FolderMetadataDraft) => result.rerender(view(draft)) };
}

const settingsRoute = (): Routes => ({
  'GET /admin/settings/metadata': () => json(defaults),
});

describe('FolderMetadataFields (concept 24 4.2)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('holds the hint of the previous list while the sources answer about the new one', async () => {
    // TMDB knows Deutsch and Englisch, AniList Englisch; each answer names the languages of the
    // list it was asked about.
    const knows: Record<string, string[]> = { tmdb: ['de', 'en'], anilist: ['en'] };
    const answer = (langs: string[]) =>
      json(
        Object.entries(knows).map(([id, own]) => ({
          ...tmdb(langs.filter((code) => own.includes(code))),
          id,
        })),
      );
    let release = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    let asked = 0;
    const { fetchMock } = mockApi({
      ...settingsRoute(),
      'GET /admin/metadata/providers': async () => {
        asked += 1;
        const request = fetchMock.mock.calls.at(-1)?.[0] as Request;
        const langs = new URL(request.url).searchParams.getAll('textLangs');
        if (asked > 1) await held;
        return answer(langs);
      },
    });
    const { rerenderWith } = renderFields({
      ownLangs: true,
      textLangs: ['de', 'ja'],
      ownImages: false,
    });
    // One line per hint: what a source does not know, then the cost.
    expect(await screen.findByText('TMDB kennt nicht: Japanisch')).toBeVisible();
    expect(screen.getByText(/^Jede weitere Sprache kostet/)).toBeVisible();
    rerenderWith({ ownLangs: true, textLangs: ['de', 'ja', 'en'], ownImages: false });
    await waitFor(() => expect(asked).toBe(2));
    // The answer about ['de', 'ja'] would call Englisch unknown to TMDB.
    expect(screen.queryByText(/kennt nicht/)).toBeNull();
    release();
    expect(await screen.findByText('TMDB kennt nicht: Japanisch')).toBeVisible();
    expect(screen.getByText('ANILIST kennt nicht: Deutsch und Japanisch')).toBeVisible();
  });

  it('names the default beside «Wie Standard»: the languages, the image kinds and their languages', async () => {
    mockApi({
      'GET /admin/settings/metadata': () =>
        json({ ...defaults, imageKinds: ['season_poster'] } satisfies MetadataSettings),
    });
    renderFields({ ownLangs: false, ownImages: false });
    const langs = await screen.findByRole('radiogroup', { name: 'Sprachen' });
    expect(langs).toHaveAccessibleDescription('Standard: Deutsch › Englisch');
    // Without the logo, the default fetches no logo, so its language is not named.
    expect(screen.getByRole('radiogroup', { name: 'Bilder' })).toHaveAccessibleDescription(
      'Standard: Poster, Hintergrund und Staffelposter. Sprache je Bildart: ' +
        'Poster: Deutsch › Englisch › Ohne Text und Hintergrund: Ohne Text › Deutsch › Englisch.',
    );
  });

  it('shows a placeholder while the default loads', () => {
    mockApi({ 'GET /admin/settings/metadata': () => new Promise<Response>(() => undefined) });
    const { container } = renderFields({ ownLangs: false, ownImages: false });
    expect(screen.queryByRole('radiogroup')).toBeNull();
    expect(container.querySelector('[class*="skeleton"]')).not.toBeNull();
  });

  it('says so when the default cannot be loaded and loads it again on «Erneut versuchen»', async () => {
    let fail = true;
    mockApi({
      'GET /admin/settings/metadata': () =>
        fail ? problem(500, 'internal') : json(defaults satisfies MetadataSettings),
    });
    renderFields({ ownLangs: false, ownImages: false });
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    expect(screen.queryByRole('radiogroup')).toBeNull();
    fail = false;
    await userEvent.setup().click(retry);
    expect(await screen.findByRole('radiogroup', { name: 'Sprachen' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Erneut versuchen' })).toBeNull();
  });
});
