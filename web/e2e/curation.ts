import type { APIRequestContext, APIResponse } from '@playwright/test';
import { CSRF, H264_TITLE } from './catalog';

// API helpers for the curation of M7 (stream C2): the user picks (concept 18 part B), manual
// collections and the order of the start page rows. They throw instead of using `expect`, so the global setup and
// the `owner` fixture can use them as well as the specs.

/** A title of the user picks as the administration lists it (anonymous). */
export interface PickData {
  itemId: string;
  title: string;
  count: number;
  lastPickedOn: string;
  expiresInDays: number;
}

export interface CollectionData {
  id: string;
  source: 'tmdb' | 'manual';
  name: string;
  itemCount: number;
}

export interface HomeRowData {
  /** The key, or `library:<id>` for a library row (concept 15 P-7). */
  id: string;
  key: string;
  libraryId?: number;
  libraryName?: string;
  position: number;
  hidden: boolean;
}

/** The start page rows in the order of migration 00015 (E-1), none hidden. */
export const DEFAULT_HOME_ROWS = [
  'spotlight',
  'picks',
  'continue',
  'recent',
  'released',
  'watchlist',
  'collections',
  'genres',
] as const;

async function ok<T>(response: APIResponse, expected: number[]): Promise<T> {
  if (!expected.includes(response.status())) {
    throw new Error(`${response.url()} answered ${response.status()}: ${await response.text()}`);
  }
  return (response.status() === 204 ? undefined : await response.json()) as T;
}

/**
 * Id of a film or series by its title. Without TMDB every title stays unmatched, so the
 * maintenance list of the owner names them all.
 */
export async function titleId(api: APIRequestContext, title: string): Promise<string> {
  const { items } = await ok<{ items: { itemId: string; title: string }[] }>(
    await api.get('/api/v1/admin/maintenance/issues?kind=unmatched&limit=200'),
    [200],
  );
  const item = items.find((candidate) => candidate.title === title);
  if (!item) throw new Error(`title "${title}" is not in the library`);
  return item.itemId;
}

export async function listPicks(api: APIRequestContext): Promise<PickData[]> {
  return ok<PickData[]>(await api.get('/api/v1/admin/picks'), [200]);
}

/**
 * Recommends a title as the active profile of `api` (the star of the title page, concept 18 part
 * B): a profile of the owner's account; the title goes to the front of the user picks.
 */
export async function recommendItem(
  api: APIRequestContext,
  itemId: string,
): Promise<{ expiresAt: string; retentionDays: number }> {
  return ok(await api.put(`/api/v1/items/${itemId}/pick`, { headers: CSRF }), [200]);
}

/** Removes every pick of a title (the administration's «Entfernen»); no pick is fine. */
export async function deletePick(api: APIRequestContext, itemId: string): Promise<void> {
  await ok(await api.delete(`/api/v1/admin/picks/${itemId}`, { headers: CSRF }), [204, 404]);
}

/**
 * One pick of the H264 film, so the start page has its row of user picks (without picks there is
 * none). Idempotent: an existing one is kept.
 */
export async function ensureBaselinePick(api: APIRequestContext): Promise<void> {
  if ((await listPicks(api)).some((pick) => pick.title === H264_TITLE)) return;
  await recommendItem(api, await titleId(api, H264_TITLE));
}

export async function listCollections(api: APIRequestContext): Promise<CollectionData[]> {
  return ok<CollectionData[]>(await api.get('/api/v1/admin/collections'), [200]);
}

/** Creates a manual collection with these members in this order. */
export async function createCollection(
  api: APIRequestContext,
  name: string,
  itemIds: string[] = [],
): Promise<CollectionData> {
  const created = await ok<CollectionData>(
    await api.post('/api/v1/admin/collections', { headers: CSRF, data: { name } }),
    [201],
  );
  if (itemIds.length === 0) return created;
  return ok<CollectionData>(
    await api.put(`/api/v1/admin/collections/${created.id}/items`, {
      headers: CSRF,
      data: { itemIds },
    }),
    [200],
  );
}

export async function deleteCollection(api: APIRequestContext, id: string): Promise<void> {
  await ok(await api.delete(`/api/v1/admin/collections/${id}`, { headers: CSRF }), [204, 404]);
}

export async function listHomeRows(api: APIRequestContext): Promise<HomeRowData[]> {
  return ok<HomeRowData[]>(await api.get('/api/v1/admin/home-rows'), [200]);
}

/**
 * Stores the rows with these ids in this order; ids in `hidden` are hidden. Every other stored row
 * («Meine Medien» and the library rows, concept 15 P-7, unless named) follows hidden.
 */
export async function setHomeRows(
  api: APIRequestContext,
  ids: readonly string[],
  hidden: readonly string[] = [],
): Promise<void> {
  const stored = await listHomeRows(api);
  const keyOf = (id: string) => stored.find((row) => row.id === id)?.key ?? id;
  const named = ids.map((id) => ({ id, key: keyOf(id), hidden: hidden.includes(id) }));
  const rest = stored
    .filter((row) => !ids.includes(row.id))
    .map((row) => ({ id: row.id, key: row.key, hidden: true }));
  const data = [...named, ...rest].map((row, position) => ({ ...row, position }));
  await ok(await api.put('/api/v1/admin/home-rows', { headers: CSRF, data }), [200]);
}

/**
 * Leaves the curation as the global setup made it: only the baseline pick, no manual
 * collection, the start page rows in their default order with none hidden. TMDB collections
 * (none in the e2e run, the server has no key) cannot be deleted and stay.
 */
export async function resetCuration(api: APIRequestContext): Promise<void> {
  for (const pick of await listPicks(api)) {
    if (pick.title !== H264_TITLE) await deletePick(api, pick.itemId);
  }
  await ensureBaselinePick(api);
  for (const collection of await listCollections(api)) {
    if (collection.source === 'manual') await deleteCollection(api, collection.id);
  }
  const rows = await listHomeRows(api);
  const fixed = rows.slice(0, DEFAULT_HOME_ROWS.length);
  const isDefault =
    fixed.every((row) => !row.hidden) &&
    fixed.map((row) => row.id).join() === DEFAULT_HOME_ROWS.join() &&
    rows.slice(DEFAULT_HOME_ROWS.length).every((row) => row.hidden);
  if (!isDefault) await setHomeRows(api, DEFAULT_HOME_ROWS);
}
