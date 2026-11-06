import type { AdminItem, ImageCandidate } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { ApiError } from '@/api/client';
import { FROM_METADATA } from '../item/MetadataTab';
import {
  canToggleLock,
  candidateFacets,
  editField,
  emptyDraft,
  filterCandidates,
  saveOf,
  serverValues,
  shownLocked,
  toggleLock,
  uploadErrorKey,
  valueOf,
  withGenre,
  type AdminItemMetadata,
} from './editorModel';

const meta: AdminItemMetadata = {
  lang: 'de',
  title: 'Alien',
  sortTitle: 'Alien',
  year: 1979,
  overview: 'Im Weltall hört dich niemand schreien.',
  genres: ['Horror'],
  locked: ['overview'],
  sources: [{ field: 'title', provider: 'tmdb' }],
};

const item = {
  id: 'alien',
  kind: 'movie',
  title: 'Alien',
  lockedFields: ['overview:de'],
  fieldSources: [],
  ownRating: undefined,
  metadata: meta,
} as unknown as AdminItem;

const server = serverValues(item, meta);

describe('metadata editor: the draft', () => {
  it('reads the server values; a missing text is empty, the rating «Aus Metadaten»', () => {
    expect(server).toMatchObject({
      title: 'Alien',
      originalTitle: '',
      year: '1979',
      releaseDate: '',
      genres: ['Horror'],
      rating: FROM_METADATA,
      language: FROM_METADATA,
    });
  });

  it('keeps only real changes; going back to the server value drops the change', () => {
    let draft = editField(server, emptyDraft, 'title', 'Alien – Das unheimliche Wesen');
    expect(valueOf(server, draft, 'title')).toBe('Alien – Das unheimliche Wesen');
    draft = editField(server, draft, 'title', 'Alien');
    expect(draft.changed).toEqual({});
    draft = editField(server, draft, 'genres', ['Horror']);
    expect(draft.changed).toEqual({});
  });

  it('a change locks the field, the lock reverts the change', () => {
    expect(shownLocked(item, meta, server, emptyDraft, 'title')).toBe(false);
    expect(canToggleLock(item, meta, server, emptyDraft, 'title')).toBe(false);
    const changed = editField(server, emptyDraft, 'title', 'Alien 2');
    expect(shownLocked(item, meta, server, changed, 'title')).toBe(true);
    const reverted = toggleLock(item, meta, server, changed, 'title');
    expect(reverted).toEqual({ changed: {}, unlock: [] });
  });

  it('the lock of a locked field lifts it, a second press keeps it; a change drops the unlock', () => {
    expect(shownLocked(item, meta, server, emptyDraft, 'overview')).toBe(true);
    const lifted = toggleLock(item, meta, server, emptyDraft, 'overview');
    expect(lifted.unlock).toEqual(['overview']);
    expect(shownLocked(item, meta, server, lifted, 'overview')).toBe(false);
    expect(toggleLock(item, meta, server, lifted, 'overview')).toEqual(emptyDraft);
    const edited = editField(server, lifted, 'overview', 'Neu');
    expect(edited.unlock).toEqual([]);
    expect(shownLocked(item, meta, server, edited, 'overview')).toBe(true);
  });

  it('the rating and the language unlock by going back to «Aus Metadaten»', () => {
    const own = { ...item, ownRating: 16 } as AdminItem;
    const ownServer = serverValues(own, meta);
    expect(shownLocked(own, meta, ownServer, emptyDraft, 'rating')).toBe(true);
    const lifted = toggleLock(own, meta, ownServer, emptyDraft, 'rating');
    expect(lifted.changed).toEqual({ rating: FROM_METADATA });
    expect(toggleLock(own, meta, ownServer, lifted, 'rating')).toEqual(emptyDraft);
  });
});

describe('metadata editor: «Speichern»', () => {
  it('sends every changed field at once, trimmed, and the lifted locks', () => {
    let draft = editField(server, emptyDraft, 'title', '  Alien 2 ');
    draft = editField(server, draft, 'year', '1986');
    draft = editField(server, draft, 'genres', ['Horror', 'Action']);
    draft = editField(server, draft, 'rating', '16');
    draft = toggleLock(item, meta, server, draft, 'overview');
    expect(saveOf(draft)).toEqual({
      metadata: {
        title: 'Alien 2',
        year: 1986,
        genres: ['Horror', 'Action'],
        unlock: ['overview'],
      },
      rating: '16',
      language: undefined,
      invalid: {},
    });
  });

  it('sends "" to clear a text; nothing changed is no request', () => {
    expect(saveOf(editField(server, emptyDraft, 'overview', '')).metadata).toEqual({
      overview: '',
    });
    expect(saveOf(emptyDraft)).toEqual({
      metadata: undefined,
      rating: undefined,
      language: undefined,
      invalid: {},
    });
  });

  it('refuses a year outside 1800 to 2200 or an empty one before sending', () => {
    expect(saveOf(editField(server, emptyDraft, 'year', '79')).invalid).toEqual({
      year: 'invalid_value',
    });
    expect(saveOf(editField(server, emptyDraft, 'year', '2300')).invalid).toEqual({
      year: 'invalid_value',
    });
    const empty = saveOf(editField(server, emptyDraft, 'year', ' '));
    expect(empty.invalid).toEqual({ year: 'required' });
    expect(empty.metadata).toBeUndefined();
  });

  it('adds a genre once, whatever its case', () => {
    expect(withGenre(['Horror'], ' Drama ')).toEqual(['Horror', 'Drama']);
    expect(withGenre(['Horror'], 'horror')).toEqual(['Horror']);
    expect(withGenre(['Horror'], '  ')).toEqual(['Horror']);
  });
});

const candidate = (provider: string, lang: string, source: string): ImageCandidate => ({
  provider,
  lang,
  source,
  width: 2000,
  height: 3000,
  score: 5,
  voteCount: 3,
  previewUrl: source,
  chosen: false,
});

describe('metadata editor: images', () => {
  const list = [
    candidate('tmdb', 'de', 'a'),
    candidate('tmdb', '', 'b'),
    candidate('fanart', 'en', 'c'),
    candidate('tmdb', 'en', 'd'),
  ];

  it('filters the candidates by provider and language ("" is without text)', () => {
    expect(filterCandidates(list, { provider: null, lang: null })).toHaveLength(4);
    expect(filterCandidates(list, { provider: 'tmdb', lang: null }).map((c) => c.source)).toEqual([
      'a',
      'b',
      'd',
    ]);
    expect(filterCandidates(list, { provider: 'tmdb', lang: 'en' }).map((c) => c.source)).toEqual([
      'd',
    ]);
    expect(filterCandidates(list, { provider: null, lang: '' }).map((c) => c.source)).toEqual([
      'b',
    ]);
    expect(candidateFacets(list)).toEqual({
      providers: ['tmdb', 'fanart'],
      langs: ['de', '', 'en'],
    });
  });

  it('names every 422 code of uploadItemImage', () => {
    const failed = (field: string, code: string) =>
      new ApiError(422, {
        type: 'about:blank',
        title: 'validation_failed',
        status: 422,
        code: 'validation_failed',
        errors: [{ field, code }],
      });
    const key = 'admin.editor.images.upload.error';
    expect(uploadErrorKey(failed('file', 'too_large'))).toBe(`${key}.tooLarge`);
    expect(uploadErrorKey(failed('file', 'unsupported_format'))).toBe(`${key}.format`);
    expect(uploadErrorKey(failed('file', 'too_many_pixels'))).toBe(`${key}.pixels`);
    expect(uploadErrorKey(failed('file', 'required'))).toBe(`${key}.missing`);
    expect(uploadErrorKey(failed('file', 'invalid_value'))).toBe(`${key}.missing`);
    expect(uploadErrorKey(failed('kind', 'invalid_value'))).toBe(`${key}.kind`);
    // Anything else is the general problem message of the page.
    expect(uploadErrorKey(new ApiError(404, null))).toBeUndefined();
    expect(uploadErrorKey(new Error('network'))).toBeUndefined();
  });
});
