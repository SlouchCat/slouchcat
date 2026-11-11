import type { AdminPodcast } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { matchesFilter, matchesSearch, sortPodcasts } from './listView';

const podcast = (over: Partial<AdminPodcast>): AdminPodcast => ({
  id: over.title ?? 'x',
  libraryId: 1,
  title: 'Podcast',
  episodeCount: 10,
  localEpisodeCount: 0,
  explicit: false,
  createdAt: '2026-11-01T00:00:00Z',
  storedBytes: 0,
  downloads: { queued: 0, running: 0, failed: 0 },
  unclearEpisodeCount: 0,
  savable: true,
  ...over,
});

describe('the list of podcasts (concept 19 part A, 12.2)', () => {
  it('filters by what the server sent', () => {
    const saving = podcast({ effectiveMode: 'save' });
    const linking = podcast({ effectiveMode: 'link' });
    const folder = podcast({});
    const loading = podcast({ downloads: { queued: 1, running: 0, failed: 0 } });
    const failed = podcast({ downloads: { queued: 0, running: 0, failed: 2 } });
    const feedError = podcast({ lastError: 'feed_unreachable: HTTP 503' });
    expect(matchesFilter(saving, 'save')).toBe(true);
    expect(matchesFilter(linking, 'save')).toBe(false);
    expect(matchesFilter(linking, 'link')).toBe(true);
    expect(matchesFilter(folder, 'link')).toBe(false);
    expect(matchesFilter(folder, 'all')).toBe(true);
    expect(matchesFilter(loading, 'loading')).toBe(true);
    expect(matchesFilter(failed, 'error')).toBe(true);
    expect(matchesFilter(feedError, 'error')).toBe(true);
    expect(matchesFilter(saving, 'error')).toBe(false);
  });

  it('searches title and author, case and accents aside', () => {
    const p = podcast({ title: 'Tagesgespräch', author: 'SRF' });
    expect(matchesSearch(p, 'gesprach', 'de')).toBe(true);
    expect(matchesSearch(p, 'srf', 'de')).toBe(true);
    expect(matchesSearch(p, '  ', 'de')).toBe(true);
    expect(matchesSearch(p, 'nzz', 'de')).toBe(false);
  });

  it('sorts stably, titles without leading signs', () => {
    const collator = new Intl.Collator('de');
    const a = podcast({
      id: 'a',
      title: '{ungeskriptet}',
      storedBytes: 5,
      latestEpisodeAt: '2026-11-02T00:00:00Z',
    });
    const b = podcast({ id: 'b', title: 'Echo der Zeit', storedBytes: 9, episodeCount: 3 });
    const c = podcast({ id: 'c', title: 'Echo der Zeit', latestEpisodeAt: '2026-11-03T00:00:00Z' });
    const ids = (list: AdminPodcast[]) => list.map((p) => p.id);
    expect(ids(sortPodcasts([a, b, c], 'title', collator))).toEqual(['b', 'c', 'a']);
    expect(ids(sortPodcasts([a, b, c], 'newest', collator))).toEqual(['c', 'a', 'b']);
    expect(ids(sortPodcasts([a, b, c], 'size', collator))).toEqual(['b', 'a', 'c']);
    expect(ids(sortPodcasts([a, b, c], 'episodes', collator))).toEqual(['a', 'c', 'b']);
  });
});
