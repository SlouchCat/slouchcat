import type { Backup, components, RestoreResult } from '@slouchcat/api-client';
import { api, apiUrl, request } from '@/api/client';

/** Shortest passphrase of a backup with secrets (plan 3.8 point 3). */
export const MIN_PASSPHRASE = 12;

type RestoreUpload = components['schemas']['RestoreUpload'];

export const downloadUrl = (backup: Backup) =>
  apiUrl(`/admin/backups/${encodeURIComponent(backup.name)}/download`);

export const restoreBackup = (name: string, passphrase: string): Promise<RestoreResult> =>
  request(
    api.POST('/admin/backups/{name}/restore', {
      params: { path: { name } },
      body: passphrase ? { passphrase } : {},
    }),
  );

export function restoreUpload(file: File, passphrase: string): Promise<RestoreResult> {
  // The server reads the passphrase before the archive streams in (RestoreUpload): it must be the
  // first part. openapi-fetch sends a FormData as it is; its type knows the parts only as strings.
  const form = new FormData();
  if (passphrase) form.append('passphrase', passphrase);
  form.append('file', file);
  return request(api.POST('/admin/backups/restore', { body: form as unknown as RestoreUpload }));
}

/** A confirmation the page asks (one dialog for the whole page). */
export interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => void;
  /**
   * `cancel` for a restore: Enter in a passphrase field opens it, and a second Enter must not
   * replace every piece of data.
   */
  defaultFocus?: 'confirm' | 'cancel';
}
