import type { Library, LibraryRoot } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { Chip, Notice, Text } from '@/ui';
import { AriaButton, ConfirmInputDialog, Form, FormDialog, TextField } from '@/ui/aria';
import { defaultRoot } from '../shared/LibraryFields';
import { browseKey } from '../shared/pathBrowser';
import { PathBrowserButton } from '../shared/PathBrowserButton';
import { librariesQuery } from './queries';
import styles from './LibraryFoldersDialog.module.css';

const watchKeys = {
  inotify: 'admin.libraries.watch.inotify',
  poll: 'admin.libraries.watch.poll',
  inotify_poll: 'admin.libraries.watch.inotify_poll',
} as const;

interface LibraryFoldersDialogProps {
  library: Library;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The library as the server returned it after a folder was added or removed. */
  onChanged: (library: Library) => void;
}

/**
 * "Edit library": the folders of a library, one row each, with a way in and a way out (idea 02,
 * E8 and E16). The card itself only names how many there are, so it stays short for a library
 * with five of them.
 *
 * Removing is the first destructive operation on a library in this project, so it goes through
 * ConfirmInputDialog: the path has to be typed out (E12, E17). What the text promises is what
 * the server does -- the titles are soft-deleted, only the offline packages are really gone --
 * and it is written out in full, because "delete" alone would be both too frightening and too
 * harmless.
 */
export function LibraryFoldersDialog({
  library,
  isOpen,
  onOpenChange,
  onChanged,
}: LibraryFoldersDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [path, setPath] = useState('');
  const [removing, setRemoving] = useState<LibraryRoot | null>(null);
  const pathInput = useRef<HTMLInputElement>(null);

  // The browse dialog marks every folder that belongs to a library already; after a change that
  // mark is stale, so its queries go with the library list.
  const refresh = async (updated: Library) => {
    onChanged(updated);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: browseKey }),
    ]);
  };

  const add = useMutation({
    mutationFn: (folder: string): Promise<Library> =>
      request(
        api.POST('/admin/libraries/{id}/roots', {
          params: { path: { id: library.id } },
          body: { roots: [defaultRoot(library.kind, folder)] },
        }),
      ),
    onSuccess: async (updated) => {
      setPath('');
      await refresh(updated);
    },
  });

  const remove = useMutation({
    // The path travels as it was typed, not as the list knows it: the server compares it against
    // its own row, so the gate sits there and not only in the dialog.
    mutationFn: ({
      root,
      confirmPath,
    }: {
      root: LibraryRoot;
      confirmPath: string;
    }): Promise<Library> =>
      request(
        api.POST('/admin/libraries/{id}/roots/{rootId}/removal', {
          params: { path: { id: library.id, rootId: root.id } },
          body: { confirmPath },
        }),
      ),
    onSuccess: async (updated) => {
      setRemoving(null);
      await refresh(updated);
    },
  });

  // The contract names the folder by its index (`roots[0].path`); the field of this dialog is `path`.
  const addErrors = add.error
    ? formErrors(t, add.error, { 'library.path': 'path', 'roots[0].path': 'path' })
    : null;

  // Every way out of the dialog clears it, the "Schliessen" button included: closing on an error
  // and opening again must not show the message of a request that is long over.
  const close = () => {
    add.reset();
    remove.reset();
    setPath('');
    setRemoving(null);
    onOpenChange(false);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (path.trim()) add.mutate(path.trim());
  };

  return (
    <>
      <FormDialog
        isOpen={isOpen}
        onOpenChange={(open) => {
          if (open) {
            onOpenChange(true);
            return;
          }
          close();
        }}
        title={t('admin.libraries.folders.title', { name: library.name })}
        description={t('admin.libraries.folders.description')}
      >
        <div className={styles.body}>
          {library.roots.length === 0 ? (
            <Notice tone="warning" live={false}>
              {t('admin.libraries.folders.none')}
            </Notice>
          ) : (
            <ul className={styles.list}>
              {library.roots.map((root) => (
                <li key={root.id} className={styles.row}>
                  <div className={styles.rowMain}>
                    <code className={styles.path}>{root.path}</code>
                    <span className={styles.meta}>
                      <Chip tone="info">{t('admin.libraries.fsType', { type: root.fsType })}</Chip>
                      {root.warning === 'network_share' && (
                        <Chip tone="warning" icon="warning">
                          {t('admin.libraries.warning.network_share')}
                        </Chip>
                      )}
                      <Text tone="secondary" variant="caption">
                        {t(watchKeys[root.watch])}
                      </Text>
                    </span>
                    <Text tone="secondary" variant="caption">
                      {t('admin.libraries.folders.content', {
                        items: root.itemCount,
                        files: root.fileCount,
                      })}
                    </Text>
                  </div>
                  <AriaButton
                    variant="ghost"
                    icon="trash"
                    aria-label={t('admin.libraries.folders.remove.label', { path: root.path })}
                    isPending={remove.isPending && remove.variables?.root.id === root.id}
                    onPress={() => {
                      remove.reset();
                      setRemoving(root);
                    }}
                  >
                    {t('admin.libraries.folders.remove.action')}
                  </AriaButton>
                </li>
              ))}
            </ul>
          )}

          <Form className={styles.add} onSubmit={onSubmit} validationErrors={addErrors?.fields}>
            {addErrors?.form && <Notice tone="danger">{addErrors.form}</Notice>}
            <TextField
              name="path"
              label={t('admin.libraries.folders.add.label')}
              description={t('setup.library.pathHint')}
              autoComplete="off"
              spellCheck="false"
              mono
              inputRef={pathInput}
              value={path}
              onChange={setPath}
            />
            <div className={styles.addActions}>
              <PathBrowserButton path={path} onPathChange={setPath} fieldRef={pathInput} />
              <AriaButton
                type="submit"
                variant="primary"
                icon="plus"
                isDisabled={!path.trim()}
                isPending={add.isPending}
              >
                {t('admin.libraries.folders.add.submit')}
              </AriaButton>
            </div>
          </Form>

          <div className={styles.close}>
            <AriaButton variant="ghost" onPress={close}>
              {t('common.close')}
            </AriaButton>
          </div>
        </div>
      </FormDialog>

      <ConfirmInputDialog
        isOpen={removing !== null}
        onOpenChange={(open) => {
          if (!open) setRemoving(null);
        }}
        title={t('admin.libraries.folders.remove.title')}
        expected={removing?.path ?? ''}
        inputLabel={t('admin.libraries.folders.remove.inputLabel')}
        mismatchText={t('admin.libraries.folders.remove.mismatch', { path: removing?.path ?? '' })}
        confirmLabel={t('admin.libraries.folders.remove.confirm')}
        cancelLabel={t('common.cancel')}
        isPending={remove.isPending}
        onConfirm={(confirmPath) => {
          if (removing) remove.mutate({ root: removing, confirmPath });
        }}
      >
        {remove.error && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
        {removing && <RemovalConsequences library={library} root={removing} />}
      </ConfirmInputDialog>
    </>
  );
}

/**
 * What removing a folder really does. It is longer than a confirmation usually is on purpose:
 * "delete" is both too frightening (the titles are soft-deleted and come back with their user
 * data when the folder returns within 30 days) and too harmless (the offline packages on the
 * devices are gone, a running playback breaks off, and the path map of Radarr and Sonarr points
 * nowhere afterwards).
 */
function RemovalConsequences({ library, root }: { library: Library; root: LibraryRoot }) {
  const { t } = useTranslation();
  const last = library.roots.length === 1;
  return (
    <>
      <p>
        <Trans
          i18nKey="admin.libraries.folders.remove.lead"
          values={{ path: root.path, name: library.name, count: root.itemCount }}
          components={{ path: <code /> }}
        />
      </p>
      <ul>
        <li>{t('admin.libraries.folders.remove.soft')}</li>
        <li>{t('admin.libraries.folders.remove.downloads')}</li>
        <li>{t('admin.libraries.folders.remove.playback')}</li>
        <li>{t('admin.libraries.folders.remove.webhooks')}</li>
        <li>{t('admin.libraries.folders.remove.relink')}</li>
        <li>{t('admin.libraries.folders.remove.oneWay')}</li>
        {last && <li>{t('admin.libraries.folders.remove.last', { name: library.name })}</li>}
      </ul>
    </>
  );
}
