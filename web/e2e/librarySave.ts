import { expect, type APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';

// Folders of a library as a spec prepares them, through saveLibrary (concept 24 4.3): the whole
// form in one request, every stored folder named, the removed one with its typed path.

interface StoredRoot {
  id: number;
  path: string;
  content: string;
  providers: string[];
  textLangs: string[] | null;
  images: unknown;
}

interface StoredLibrary {
  id: number;
  name: string;
  kind: string;
  revision: string;
  roots: StoredRoot[];
}

async function getLibrary(owner: APIRequestContext, libraryId: number): Promise<StoredLibrary> {
  const response = await owner.get(`/api/v1/admin/libraries/${libraryId}`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as StoredLibrary;
}

/** A stored folder as the form sends it back: its whole state, video folders with metadata. */
const kept = (library: StoredLibrary, root: StoredRoot) => ({
  id: root.id,
  path: root.path,
  content: root.content,
  providers: root.providers,
  ...(library.kind === 'video' ? { textLangs: root.textLangs, images: root.images } : {}),
});

/** Adds a folder to a library and returns it as the server stored it. */
export async function addRoot(
  owner: APIRequestContext,
  libraryId: number,
  root: { path: string; content: string; providers: string[] },
): Promise<StoredRoot> {
  const library = await getLibrary(owner, libraryId);
  const response = await owner.put(`/api/v1/admin/libraries/${libraryId}`, {
    headers: CSRF,
    data: {
      revision: library.revision,
      name: library.name,
      roots: [...library.roots.map((entry) => kept(library, entry)), root],
      removeRoots: [],
    },
  });
  expect(response.status(), await response.text()).toBe(200);
  const saved = (await response.json()) as { library: StoredLibrary };
  const added = saved.library.roots.find((entry) => entry.path === root.path);
  expect(added, root.path).toBeDefined();
  return added!;
}

/** Removes a folder of a library; a scan that still runs refuses it for a moment (409). */
export async function removeRoot(
  owner: APIRequestContext,
  libraryId: number,
  rootId: number,
  path: string,
): Promise<void> {
  await expect
    .poll(
      async () => {
        const library = await getLibrary(owner, libraryId);
        const response = await owner.put(`/api/v1/admin/libraries/${libraryId}`, {
          headers: CSRF,
          data: {
            revision: library.revision,
            name: library.name,
            roots: library.roots
              .filter((entry) => entry.id !== rootId)
              .map((entry) => kept(library, entry)),
            removeRoots: [{ id: rootId, confirmPath: path }],
          },
        });
        return response.status();
      },
      { timeout: 30_000 },
    )
    .toBe(200);
}
