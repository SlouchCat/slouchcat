import { api, isApiError, request } from '@/api/client';
import { cancelFetch, kickOfflineEngine, rememberDeleted } from './engine';
import { dropUnusedPinMaterial } from './offlineProfiles';
import { removeFiles } from './opfs';
import { deleteDownloads, getMeta, setMeta, updateDownload } from './store';
import { mergeDownload } from './sync';
import type { DownloadRequest, DownloadState } from './types';

/**
 * Creates the downloads of a title on this device (O-1, O-2: a season gives all its episodes),
 * stores them and wakes the engine, which fetches each one once the server has it ready.
 */
export async function createDownloads(
  itemId: string,
  body: DownloadRequest,
  accountId: number,
): Promise<DownloadState[]> {
  const { downloads } = await request(
    api.POST('/items/{id}/downloads', { params: { path: { id: itemId } }, body }),
  );
  const now = Date.now();
  for (const state of downloads) {
    await updateDownload(state.id, (local) => mergeDownload(local, state, accountId, now));
  }
  await setMeta('accountId', accountId);
  kickOfflineEngine();
  return downloads;
}

/**
 * Asks the browser once (with the first download) not to evict the site's storage. Called from
 * the click itself: Firefox asks the viewer and wants a user gesture.
 */
export function persistOnFirstDownload(alreadyAsked: boolean): void {
  if (alreadyAsked || typeof navigator.storage?.persist !== 'function') return;
  void navigator.storage
    .persist()
    .catch(() => false)
    .then(() => setMeta('persistRequested', true));
}

export const persistAsked = async (): Promise<boolean> =>
  (await getMeta('persistRequested').catch(() => undefined)) === true;

/**
 * Deletes a download (O-6): the files at once (after a running fetch closed them, W8), the
 * server's row with deleteDownload. Offline, or when the request fails (network, 5xx, K3), the
 * entry stays hidden as `pendingDelete` and the engine sends the delete at the next contact; a
 * folder that could not be removed goes at the engine's next run.
 */
export async function removeDownload(id: string, online: boolean): Promise<void> {
  await cancelFetch(id);
  await removeFiles(id).catch(() => undefined);
  await rememberDeleted([id]);
  if (online) {
    try {
      await request(api.DELETE('/downloads/{id}', { params: { path: { id } } }));
      await deleteDownloads([id]);
      await dropUnusedPinMaterial();
      return;
    } catch (error) {
      if (isApiError(error, 404)) {
        await deleteDownloads([id]);
        await dropUnusedPinMaterial();
        return;
      }
    }
  }
  await updateDownload(
    id,
    (download) =>
      download && {
        ...download,
        pendingDelete: true,
        complete: false,
        bytesWritten: 0,
        subtitlesStored: [],
        posterStored: false,
        updatedAt: Date.now(),
      },
  );
  await dropUnusedPinMaterial();
  kickOfflineEngine();
}
