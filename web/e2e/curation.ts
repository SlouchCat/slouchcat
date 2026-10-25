import type { APIRequestContext, APIResponse } from '@playwright/test';
import { CSRF, H264_TITLE } from './catalog';

// API helpers for the curation of M7 (stream C2): Cat's recommendations, manual collections and
// the order of the start page rows. They throw instead of using `expect`, so the global setup and
// the `owner` fixture can use them as well as the specs.

export interface PickData {
  id: number;
  itemId: string;
  title: string;
  profileId?: number;
  profileName?: string;
  position: number;
  note?: string;
  startsAt?: string;
  endsAt?: string;
}

export interface CollectionData {
  id: string;
  source: 'tmdb' | 'manual';
  name: string;
  itemCount: number;
}

export interface HomeRowData {
  key: string;
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

/** Creates a pick: global without `profileIds`, else one per profile. */
export async function createPick(
  api: APIRequestContext,
  body: {
    itemId: string;
    profileIds?: number[];
    note?: string;
    startsAt?: string;
    endsAt?: string;
  },
): Promise<PickData[]> {
  return ok<PickData[]>(
    await api.post('/api/v1/admin/picks', { headers: CSRF, data: body }),
    [201],
  );
}

export async function deletePick(api: APIRequestContext, id: number): Promise<void> {
  await ok(await api.delete(`/api/v1/admin/picks/${id}`, { headers: CSRF }), [204, 404]);
}

/** The pick every run starts with: the H264 film for all profiles, without note and window. */
const isBaseline = (pick: PickData) =>
  pick.title === H264_TITLE &&
  pick.profileId == null &&
  !pick.note &&
  !pick.startsAt &&
  !pick.endsAt;

/**
 * One global pick of the H264 film, so the start page has its carousel (without picks there is
 * none since M7). Idempotent: an existing one is kept.
 */
export async function ensureBaselinePick(api: APIRequestContext): Promise<void> {
  if ((await listPicks(api)).some(isBaseline)) return;
  await createPick(api, { itemId: await titleId(api, H264_TITLE) });
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

/** Stores the rows in this order; keys in `hidden` are hidden. */
export async function setHomeRows(
  api: APIRequestContext,
  keys: readonly string[],
  hidden: readonly string[] = [],
): Promise<void> {
  const data = keys.map((key, position) => ({ key, position, hidden: hidden.includes(key) }));
  await ok(await api.put('/api/v1/admin/home-rows', { headers: CSRF, data }), [200]);
}

/**
 * Leaves the curation as the global setup made it: only the baseline pick, no manual
 * collection, the start page rows in their default order with none hidden. TMDB collections
 * (none in the e2e run, the server has no key) cannot be deleted and stay.
 */
export async function resetCuration(api: APIRequestContext): Promise<void> {
  let kept = false;
  for (const pick of await listPicks(api)) {
    if (!kept && isBaseline(pick)) {
      kept = true;
      continue;
    }
    await deletePick(api, pick.id);
  }
  if (!kept) await ensureBaselinePick(api);
  for (const collection of await listCollections(api)) {
    if (collection.source === 'manual') await deleteCollection(api, collection.id);
  }
  const rows = await listHomeRows(api);
  const isDefault =
    rows.every((row) => !row.hidden) &&
    rows.map((row) => row.key).join() === DEFAULT_HOME_ROWS.join();
  if (!isDefault) await setHomeRows(api, DEFAULT_HOME_ROWS);
}
