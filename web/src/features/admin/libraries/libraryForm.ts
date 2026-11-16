import type {
  Library,
  LibraryRoot,
  LibrarySave,
  LibraryTrickplay,
  MetadataProviderInfo,
} from '@slouchcat/api-client';
import { canIdentify, type FolderEntry } from '@/ui/aria';
import { isAudioLibrary } from '../../shared/LibraryFields';
import { titleRatingOf } from '../../shared/choices';
import {
  folderMetadataDraft,
  folderMetadataValues,
  type FolderMetadataDraft,
} from '../metadata/FolderMetadataFields';
import { DEFAULT_TRICKPLAY, sameTrickplay } from './TrickplayFields';

/**
 * The state of the form «Bibliothek bearbeiten» and how it becomes one `saveLibrary` (concept 24
 * 4.3). A stored folder's card has the key `root-<id>`; a new card the key FolderList gave it.
 */
export interface LibraryFormState {
  name: string;
  /** Default age rating as a radio value ("0" … "18"); only an audio library has one. */
  rating: string;
  trickplay: LibraryTrickplay;
  folders: FolderEntry[];
  /** The languages and images of each video card by its key; absent = the default. */
  folderMetadata: Readonly<Record<string, FolderMetadataDraft>>;
}

const STORED = 'root-';

/** The card key of a stored folder. */
export const storedKey = (id: number) => `${STORED}${id}`;

/** The stored folder a card stands for, or undefined for a new card. */
export function storedRoot(library: Library, key: string): LibraryRoot | undefined {
  if (!key.startsWith(STORED)) return undefined;
  const id = Number(key.slice(STORED.length));
  return library.roots.find((root) => root.id === id);
}

/** The form filled with the library as the server holds it: one card per folder. */
export function editState(library: Library): LibraryFormState {
  return {
    name: library.name,
    rating: String(library.defaultRating ?? 0),
    trickplay: library.trickplay ?? DEFAULT_TRICKPLAY,
    folders: library.roots.map((root) => ({
      key: storedKey(root.id),
      path: root.path,
      content: root.content,
      providers: [...root.providers],
    })),
    folderMetadata: Object.fromEntries(
      library.roots.map((root) => [storedKey(root.id), folderMetadataDraft(root)]),
    ),
  };
}

/** A path as the server stores it: without surrounding spaces and without a trailing slash. */
export function cleanPath(path: string): string {
  const trimmed = path.trim();
  return trimmed.length > 1 ? trimmed.replace(/\/+$/, '') || '/' : trimmed;
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

/** What the form changed on a stored folder; the three weights of concept 6.3 included. */
export interface FolderChange {
  path: boolean;
  content: boolean;
  /** The provider list differs from the stored one. */
  providers: boolean;
  /** The first provider (the identifier) differs while the content stays. */
  identifier: boolean;
  langs: boolean;
  images: boolean;
  any: boolean;
}

export function folderChange(
  root: LibraryRoot,
  entry: FolderEntry,
  metadata: FolderMetadataDraft | undefined,
): FolderChange {
  const values = folderMetadataValues(metadata ?? folderMetadataDraft(root));
  const path = cleanPath(entry.path) !== root.path;
  const content = entry.content !== root.content;
  const providers = entry.providers.join() !== root.providers.join();
  const identifier = !content && entry.providers[0] !== root.providers[0];
  const langs = !same(values.textLangs, root.textLangs);
  const images = !same(values.images, root.images);
  return {
    path,
    content,
    providers,
    identifier,
    langs,
    images,
    any: path || content || providers || langs || images,
  };
}

/** Whether a card has unsaved changes: a new card always, a stored one when it differs. */
export function cardChanged(library: Library, state: LibraryFormState, key: string): boolean {
  const entry = state.folders.find((folder) => folder.key === key);
  const root = storedRoot(library, key);
  if (!entry) return false;
  if (!root) return true;
  return folderChange(root, entry, state.folderMetadata[key]).any;
}

/** Whether the form differs from the library it was filled with. */
export function editDirty(
  library: Library,
  state: LibraryFormState,
  marked: ReadonlySet<string>,
): boolean {
  return (
    marked.size > 0 ||
    state.name.trim() !== library.name ||
    (isAudioLibrary(library.kind) && state.rating !== String(library.defaultRating ?? 0)) ||
    (library.kind === 'video' &&
      !sameTrickplay(state.trickplay, library.trickplay ?? DEFAULT_TRICKPLAY)) ||
    state.folders.length !== library.roots.length ||
    state.folders.some((folder) => cardChanged(library, state, folder.key))
  );
}

/** One heavy change the dialog «Änderungen bestätigen» lists (concept 24 4.3). */
export interface Confirmation {
  key: string;
  root: LibraryRoot;
  /** `remove` and `content` need the typed path; `path` only names old and new path. */
  kind: 'remove' | 'content' | 'path';
  /** The new path of a moved folder. */
  path?: string;
}

/** The removals, content changes and path changes the save would make, in the form's order. */
export function confirmationsOf(
  library: Library,
  state: LibraryFormState,
  marked: ReadonlySet<string>,
): Confirmation[] {
  const out: Confirmation[] = [];
  for (const entry of state.folders) {
    const root = storedRoot(library, entry.key);
    if (!root) continue;
    if (marked.has(entry.key)) {
      out.push({ key: entry.key, root, kind: 'remove' });
      continue;
    }
    const change = folderChange(root, entry, state.folderMetadata[entry.key]);
    if (change.content) out.push({ key: entry.key, root, kind: 'content' });
    else if (change.path)
      out.push({ key: entry.key, root, kind: 'path', path: cleanPath(entry.path) });
  }
  return out;
}

/** The cards whose path and content changed together, which one save cannot do (4.2). */
export function pathWithContent(
  library: Library,
  state: LibraryFormState,
  marked: ReadonlySet<string>,
): string[] {
  return state.folders
    .filter((entry) => !marked.has(entry.key))
    .filter((entry) => {
      const root = storedRoot(library, entry.key);
      if (!root) return false;
      const change = folderChange(root, entry, state.folderMetadata[entry.key]);
      return change.path && change.content;
    })
    .map((entry) => entry.key);
}

/** Whether the identifier of a card changes to a provider that can identify its content. */
export function identifierChanges(
  root: LibraryRoot,
  entry: FolderEntry,
  infos: readonly MetadataProviderInfo[] | undefined,
): boolean {
  const first = entry.providers[0];
  return (
    entry.content === root.content &&
    first !== undefined &&
    first !== root.providers[0] &&
    canIdentify(entry.content, first, infos)
  );
}

/**
 * The body of `saveLibrary` and the card key behind each folder of it, by its index in `roots`
 * and `removeRoots`: the server names a folder by that index (`roots[1].path`), the form by the
 * card's index in the list.
 */
export function saveRequest(
  library: Library,
  state: LibraryFormState,
  marked: ReadonlySet<string>,
  typed: Readonly<Record<string, string>>,
): { body: LibrarySave; roots: string[]; removals: string[] } {
  const video = library.kind === 'video';
  const roots: string[] = [];
  const removals: string[] = [];
  const body: LibrarySave = {
    revision: library.revision ?? '',
    name: state.name.trim(),
    roots: [],
    removeRoots: [],
  };
  if (isAudioLibrary(library.kind)) {
    const rating = titleRatingOf(state.rating);
    if (rating !== undefined) body.defaultRating = rating;
  }
  if (video) body.trickplay = state.trickplay;
  for (const entry of state.folders) {
    const root = storedRoot(library, entry.key);
    if (root && marked.has(entry.key)) {
      removals.push(entry.key);
      body.removeRoots.push({ id: root.id, confirmPath: (typed[entry.key] ?? '').trim() });
      continue;
    }
    roots.push(entry.key);
    // The form sends a folder's whole state: `null` follows the default (concept 24 3.6).
    const metadata = video
      ? folderMetadataValues(state.folderMetadata[entry.key] ?? folderMetadataDraft())
      : {};
    body.roots.push({
      ...(root ? { id: root.id } : {}),
      path: entry.path.trim(),
      content: entry.content,
      providers: entry.providers,
      ...metadata,
      ...(root && entry.content !== root.content
        ? { confirmPath: (typed[entry.key] ?? '').trim() }
        : {}),
    });
  }
  return { body, roots, removals };
}

/**
 * The server's field messages under the names of the form: `roots[i].…` of the request becomes
 * `roots[n].…` of the card's place in the list; `removeRoots[j].…` and a content change's
 * `roots[i].confirmPath` go to the typed field of that card (`confirm.<key>`).
 */
export function formFieldErrors(
  fields: Readonly<Record<string, string>>,
  state: LibraryFormState,
  sent: { roots: readonly string[]; removals: readonly string[] },
): Record<string, string> {
  const place = (key: string | undefined) =>
    key === undefined ? -1 : state.folders.findIndex((folder) => folder.key === key);
  const out: Record<string, string> = {};
  for (const [field, message] of Object.entries(fields)) {
    const root = /^roots\[(\d+)\]\.(.+)$/.exec(field);
    const removal = /^removeRoots\[(\d+)\]\.(.+)$/.exec(field);
    if (root) {
      const key = sent.roots[Number(root[1])];
      if (root[2] === 'confirmPath' && key) out[`confirm.${key}`] = message;
      else if (place(key) >= 0) out[`roots[${place(key)}].${root[2]}`] = message;
      else out[field] = message;
    } else if (removal) {
      const key = sent.removals[Number(removal[1])];
      out[key ? `confirm.${key}` : field] = message;
    } else {
      out[field] = message;
    }
  }
  return out;
}

/** The card keys a field message belongs to, so their cards open (concept 24 5, exception 3). */
export function cardsWithErrors(
  fields: Readonly<Record<string, string>>,
  state: LibraryFormState,
): string[] {
  const keys = new Set<string>();
  for (const field of Object.keys(fields)) {
    const match = /^roots\[(\d+)\]\./.exec(field);
    const key = match ? state.folders[Number(match[1])]?.key : undefined;
    if (key) keys.add(key);
  }
  return [...keys];
}

/** What one save changed, for the one band on the page afterwards. */
export interface SaveSummary {
  name?: string;
  rating: boolean;
  trickplay: boolean;
  added: number;
  removed: number;
  content: number;
  moved: number;
  /** Folders whose provider list changed (with neither content nor path). */
  providers: number;
  /** Folders whose languages changed; `apply` then queues texts, else images. */
  langs: number;
  images: number;
}

export function saveSummary(
  library: Library,
  state: LibraryFormState,
  marked: ReadonlySet<string>,
): SaveSummary {
  const summary: SaveSummary = {
    rating: isAudioLibrary(library.kind) && state.rating !== String(library.defaultRating ?? 0),
    trickplay:
      library.kind === 'video' &&
      !sameTrickplay(state.trickplay, library.trickplay ?? DEFAULT_TRICKPLAY),
    added: 0,
    removed: 0,
    content: 0,
    moved: 0,
    providers: 0,
    langs: 0,
    images: 0,
  };
  if (state.name.trim() !== library.name) summary.name = state.name.trim();
  for (const entry of state.folders) {
    const root = storedRoot(library, entry.key);
    if (!root) {
      summary.added += 1;
      continue;
    }
    if (marked.has(entry.key)) {
      summary.removed += 1;
      continue;
    }
    const change = folderChange(root, entry, state.folderMetadata[entry.key]);
    if (change.content) summary.content += 1;
    else if (change.path) summary.moved += 1;
    else if (change.providers) summary.providers += 1;
    if (change.langs) summary.langs += 1;
    if (change.images) summary.images += 1;
  }
  return summary;
}
