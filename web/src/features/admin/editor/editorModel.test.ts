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

  it('a free sort title is empty; anything typed sets and locks it, clearing keeps it free', () => {
    expect(server.sortTitle).toBe('');
    expect(shownLocked(item, meta, server, emptyDraft, 'sortTitle')).toBe(false);
    // The title's own text is a value of its own: it locks the order against a new title.
    const same = editField(server, emptyDraft, 'sortTitle', 'Alien');
    expect(same.changed).toEqual({ sortTitle: 'Alien' });
    expect(shownLocked(item, meta, server, same, 'sortTitle')).toBe(true);
    expect(saveOf(same, server).metadata).toEqual({ sortTitle: 'Alien' });
    const cleared = editField(server, same, 'sortTitle', '');
    expect(cleared).toEqual(emptyDraft);
    expect(shownLocked(item, meta, server, cleared, 'sortTitle')).toBe(false);
    expect(saveOf(cleared, server).metadata).toBeUndefined();
  });

  it('a locked sort title shows its text; its lock lifts it', () => {
    const lockedMeta: AdminItemMetadata = { ...meta, sortTitle: 'Alien 1', locked: ['sortTitle'] };
    const lockedServer = serverValues(item, lockedMeta);
    expect(lockedServer.sortTitle).toBe('Alien 1');
    expect(shownLocked(item, lockedMeta, lockedServer, emptyDraft, 'sortTitle')).toBe(true);
    const lifted = toggleLock(item, lockedMeta, lockedServer, emptyDraft, 'sortTitle');
    expect(saveOf(lifted, server).metadata).toEqual({ unlock: ['sortTitle'] });
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
    expect(saveOf(draft, server)).toEqual({
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
    expect(saveOf(editField(server, emptyDraft, 'overview', ''), server).metadata).toEqual({
      overview: '',
    });
    expect(saveOf(emptyDraft, server)).toEqual({
      metadata: undefined,
      rating: undefined,
      language: undefined,
      invalid: {},
    });
  });

  it('refuses a year outside 1800 to 2200 or an empty one before sending', () => {
    expect(saveOf(editField(server, emptyDraft, 'year', '79'), server).invalid).toEqual({
      year: 'invalid_value',
    });
    expect(saveOf(editField(server, emptyDraft, 'year', '2300'), server).invalid).toEqual({
      year: 'invalid_value',
    });
    const empty = saveOf(editField(server, emptyDraft, 'year', ' '), server);
    expect(empty.invalid).toEqual({ year: 'required' });
    expect(empty.metadata).toBeUndefined();
  });

  it('adds a genre once, whatever its case', () => {
    expect(withGenre(['Horror'], ' Drama ')).toEqual(['Horror', 'Drama']);
    expect(withGenre(['Horror'], 'horror')).toEqual(['Horror']);
    expect(withGenre(['Horror'], '  ')).toEqual(['Horror']);
  });
});

const bookMeta: AdminItemMetadata = {
  lang: 'de',
  title: 'Die Gefährten',
  genres: ['Fantasy'],
  author: 'J. R. R. Tolkien',
  narrator: 'Achim Höppner',
  seriesName: 'Der Herr der Ringe',
  seriesIndex: 1,
  isbn: '9783000000000',
  locked: ['narrator'],
  sources: [{ field: 'author', provider: 'local' }],
};
const book = {
  id: 'book',
  kind: 'audiobook',
  title: 'Die Gefährten',
  lockedFields: ['narrator'],
  fieldSources: [],
  metadata: bookMeta,
} as unknown as AdminItem;
const bookServer = serverValues(book, bookMeta);
/** «Speichern» of a book after `changes`, one field after the other. */
const bookSave = (changes: [keyof typeof bookServer, string][]) =>
  saveOf(
    changes.reduce(
      (draft, [field, value]) => editField(bookServer, draft, field, value),
      emptyDraft,
    ),
    bookServer,
  );

describe('metadata editor: an audiobook (concept 19 D5.2)', () => {
  it('reads the book fields; a missing one is empty, the volume as text', () => {
    expect(bookServer).toMatchObject({
      author: 'J. R. R. Tolkien',
      narrator: 'Achim Höppner',
      seriesName: 'Der Herr der Ringe',
      seriesIndex: '1',
      publisher: '',
      isbn: '9783000000000',
      asin: '',
    });
    expect(shownLocked(book, bookMeta, bookServer, emptyDraft, 'narrator')).toBe(true);
    expect(shownLocked(book, bookMeta, bookServer, emptyDraft, 'author')).toBe(false);
    const changed = editField(bookServer, emptyDraft, 'author', 'Tolkien');
    expect(shownLocked(book, bookMeta, bookServer, changed, 'author')).toBe(true);
  });

  it('sends the book fields trimmed; the volume with a point or a comma as a number', () => {
    expect(
      bookSave([
        ['author', ' J.R.R. Tolkien '],
        ['narrator', 'Gert Heidenreich'],
        ['seriesName', 'Herr der Ringe'],
        ['seriesIndex', '2,5'],
        ['asin', 'b0example1'],
      ]),
    ).toEqual({
      metadata: {
        author: 'J.R.R. Tolkien',
        narrator: 'Gert Heidenreich',
        seriesName: 'Herr der Ringe',
        seriesIndex: 2.5,
        asin: 'b0example1',
      },
      rating: undefined,
      language: undefined,
      invalid: {},
    });
    expect(bookSave([['seriesIndex', '2']]).metadata).toEqual({ seriesIndex: 2 });
    expect(bookSave([['seriesIndex', '0.5']]).metadata).toEqual({ seriesIndex: 0.5 });
  });

  it('refuses a volume that is no number before sending', () => {
    for (const typed of ['zwei', '2.5.1', '-1', '10000', '2,']) {
      const plan = bookSave([['seriesIndex', typed]]);
      expect(plan.invalid, typed).toEqual({ seriesIndex: 'invalid_value' });
      expect(plan.metadata, typed).toBeUndefined();
    }
  });

  it('an emptied volume needs an emptied series; then only the series goes', () => {
    expect(bookSave([['seriesIndex', ' ']]).invalid).toEqual({ seriesIndex: 'required' });
    expect(
      bookSave([
        ['seriesName', 'Neue Reihe'],
        ['seriesIndex', ''],
      ]).invalid,
    ).toEqual({ seriesIndex: 'required' });
    expect(
      bookSave([
        ['seriesName', ''],
        ['seriesIndex', ''],
      ]),
    ).toEqual({
      metadata: { seriesName: '' },
      rating: undefined,
      language: undefined,
      invalid: {},
    });
    // A volume without a series is refused, as the server would.
    expect(
      bookSave([
        ['seriesName', ' '],
        ['seriesIndex', '3'],
      ]).invalid,
    ).toEqual({ seriesIndex: 'invalid_value' });
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
