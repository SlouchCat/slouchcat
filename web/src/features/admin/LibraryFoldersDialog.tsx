import type { Library, LibraryRoot, LibraryRootUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { Chip, Notice, Text } from '@/ui';
import {
  AriaButton,
  ConfirmInputDialog,
  FolderList,
  Form,
  FormDialog,
  ProviderList,
  RadioGroup,
  canIdentify,
  contentsOf,
  defaultProviders,
  folderListError,
  providerChain,
  providerListError,
  type FolderEntry,
} from '@/ui/aria';
import { browseKey } from '../shared/pathBrowser';
import { PathBrowserButton } from '../shared/PathBrowserButton';
import { librariesQuery, metadataProvidersQuery } from './queries';
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
  /** The library as the server returned it after a folder was added, changed or removed. */
  onChanged: (library: Library) => void;
}

/**
 * "Edit library": the folders of a library, one row each with content and provider list, a way
 * in («Neue Ordner», several at once, F-13), «Ändern» (content and list, ADR 0015) and a way out
 * (idea 02, E8 and E16). The card itself only names how many there are, so it stays short for a
 * library with five of them.
 *
 * Removing is the first destructive operation on a library in this project, so it goes through
 * ConfirmInputDialog: the path has to be typed out (E12, E17). What the text promises is what
 * the server does -- the titles are soft-deleted, only the offline packages are really gone --
 * and it is written out in full, because "delete" alone would be both too frightening and too
 * harmless. A change of the content weighs the same (the folder is read anew), so it goes through
 * the same typed confirmation (concept 9.5).
 */
export function LibraryFoldersDialog({
  library,
  isOpen,
  onOpenChange,
  onChanged,
}: LibraryFoldersDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [folders, setFolders] = useState<FolderEntry[]>([]);
  const [removing, setRemoving] = useState<LibraryRoot | null>(null);
  const [changing, setChanging] = useState<LibraryRoot | null>(null);
  const [saved, setSaved] = useState<string | null>(null);
  const providers = useQuery({ ...metadataProvidersQuery, enabled: isOpen });

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
    mutationFn: (list: readonly FolderEntry[]): Promise<Library> =>
      request(
        api.POST('/admin/libraries/{id}/roots', {
          params: { path: { id: library.id } },
          body: {
            roots: list.map((folder) => ({
              path: folder.path.trim(),
              content: folder.content,
              providers: folder.providers,
            })),
          },
        }),
      ),
    onSuccess: async (updated) => {
      setFolders([]);
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

  const addErrors = add.error ? formErrors(t, add.error) : null;
  const listError = folderListError(t, folders, providers.data);

  // Every way out of the dialog clears it, the "Schliessen" button included: closing on an error
  // and opening again must not show the message of a request that is long over.
  const close = () => {
    add.reset();
    remove.reset();
    setFolders([]);
    setRemoving(null);
    setChanging(null);
    setSaved(null);
    onOpenChange(false);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (folders.length > 0 && !listError) add.mutate(folders);
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
        size="wide"
      >
        <div className={styles.body}>
          {saved && <Notice>{saved}</Notice>}
          {library.roots.length === 0 ? (
            <Notice tone="warning" live={false}>
              {library.kind === 'podcasts'
                ? t('admin.libraries.noRoots')
                : t('admin.libraries.folders.none')}
            </Notice>
          ) : (
            <ul className={styles.list}>
              {library.roots.map((root) => (
                <li key={root.id} className={styles.row}>
                  <div className={styles.rowMain}>
                    <code className={styles.path}>{root.path}</code>
                    <span className={styles.meta}>
                      <Chip tone="outline">{t(`library.content.${root.content}`)}</Chip>
                      <Text tone="secondary" variant="caption">
                        {providerChain(t, root.providers)}
                      </Text>
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
                  <span className={styles.rowActions}>
                    <AriaButton
                      variant="ghost"
                      aria-label={t('admin.libraries.folders.change.label', { path: root.path })}
                      onPress={() => {
                        setSaved(null);
                        setChanging(root);
                      }}
                    >
                      {t('admin.libraries.folders.change.action')}
                    </AriaButton>
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
                  </span>
                </li>
              ))}
            </ul>
          )}

          <Form className={styles.add} onSubmit={onSubmit} validationErrors={addErrors?.fields}>
            <h3 className={styles.addTitle}>{t('admin.libraries.folders.new')}</h3>
            {addErrors?.form && <Notice tone="danger">{addErrors.form}</Notice>}
            <FolderList
              kind={library.kind}
              entries={folders}
              onChange={(next) => {
                // The server names a card by its index: a message must not move to another card.
                if (add.error) add.reset();
                setFolders(next);
              }}
              infos={providers.data}
              min={0}
              namePrefix="roots"
              fieldErrors={addErrors?.fields}
              pathButton={(folder, inputRef) => (
                <PathBrowserButton
                  path={folder.path}
                  onPathChange={(path) =>
                    setFolders((list) =>
                      list.map((entry) => (entry.key === folder.key ? { ...entry, path } : entry)),
                    )
                  }
                  fieldRef={inputRef}
                />
              )}
            />
            {listError && folders.some((folder) => folder.path.trim()) && (
              <Notice tone="warning" live={false}>
                {listError}
              </Notice>
            )}
            {folders.length > 0 && (
              <div className={styles.addActions}>
                <AriaButton
                  type="submit"
                  variant="primary"
                  icon="plus"
                  isDisabled={listError !== null}
                  isPending={add.isPending}
                >
                  {t('library.roots.addSubmit', { count: folders.length })}
                </AriaButton>
              </div>
            )}
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

      {changing && (
        <ChangeRootDialog
          library={library}
          root={changing}
          onClose={() => setChanging(null)}
          onSaved={async (updated) => {
            setChanging(null);
            setSaved(t('admin.libraries.folders.change.saved', { path: changing.path }));
            await refresh(updated);
          }}
        />
      )}
    </>
  );
}

/**
 * «Ordner ändern» (updateLibraryRoot, ADR 0015, concept 6.3): content and provider list of one
 * folder. The three weights show before the save: only fillers change (nothing is deleted), the
 * identifier changes (season and episode texts follow the new numbering), the content changes
 * (the folder is read anew, which weighs as much as removing it -- the typed path confirms it,
 * concept 9.5).
 */
function ChangeRootDialog({
  library,
  root,
  onClose,
  onSaved,
}: {
  library: Library;
  root: LibraryRoot;
  onClose: () => void;
  onSaved: (library: Library) => Promise<void>;
}) {
  const { t } = useTranslation();
  const providers = useQuery(metadataProvidersQuery);
  const [content, setContent] = useState(root.content);
  const [list, setList] = useState(root.providers);
  const [confirming, setConfirming] = useState(false);
  const contents = contentsOf(library.kind);
  const contentChanges = content !== root.content;
  const identifierChanges = !contentChanges && list[0] !== root.providers[0];
  const listChanges = list.join() !== root.providers.join();
  const save = useMutation({
    mutationFn: (body: LibraryRootUpdate): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}/roots/{rootId}', {
          params: { path: { id: library.id, rootId: root.id } },
          body,
        }),
      ),
    onSuccess: onSaved,
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const listError = providerListError(t, content, list, providers.data);
  const unchanged = !contentChanges && !listChanges;

  const submit = (confirmPath?: string) => {
    save.mutate({
      ...(contentChanges ? { content } : {}),
      ...(listChanges || contentChanges ? { providers: list } : {}),
      ...(confirmPath !== undefined ? { confirmPath } : {}),
    });
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (listError || unchanged) return;
    if (contentChanges) {
      setConfirming(true);
      return;
    }
    submit();
  };

  return (
    <>
      <FormDialog
        isOpen={!confirming}
        onOpenChange={(open) => {
          if (!open) onClose();
        }}
        title={t('admin.libraries.folders.change.title')}
        description={t('admin.libraries.folders.change.description', { path: root.path })}
      >
        <Form className={styles.add} onSubmit={onSubmit} validationErrors={errors?.fields}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          {contents.length > 1 && (
            <RadioGroup
              name="content"
              label={t('library.content.title')}
              description={t('library.content.hint')}
              value={content}
              onChange={(value) => {
                const next = contents.find((candidate) => candidate === value);
                if (!next || next === content) return;
                setContent(next);
                setList(next === root.content ? root.providers : defaultProviders(next));
              }}
              options={contents.map((value) => ({ value, label: t(`library.content.${value}`) }))}
            />
          )}
          <ProviderList
            name="providers"
            content={content}
            value={list}
            onChange={setList}
            infos={providers.data}
            error={errors?.fields.providers ?? errors?.fields.content}
          />
          {contentChanges && (
            <Notice tone="warning" live={false}>
              {t('admin.libraries.folders.change.content')}
            </Notice>
          )}
          {identifierChanges && list[0] && canIdentify(content, list[0], providers.data) && (
            <Notice tone="info" live={false}>
              {t('admin.libraries.folders.change.identifier', {
                provider: t(`library.provider.${list[0]}`),
              })}
            </Notice>
          )}
          {!contentChanges && !identifierChanges && listChanges && (
            <Notice live={false}>{t('admin.libraries.folders.change.fillers')}</Notice>
          )}
          <div className={styles.addActions}>
            <AriaButton variant="ghost" onPress={onClose}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton
              type="submit"
              variant="primary"
              isDisabled={unchanged || listError !== null}
              isPending={save.isPending}
            >
              {t('admin.libraries.folders.change.submit')}
            </AriaButton>
          </div>
        </Form>
      </FormDialog>

      <ConfirmInputDialog
        isOpen={confirming}
        onOpenChange={(open) => {
          if (!open) setConfirming(false);
        }}
        title={t('admin.libraries.folders.change.confirmTitle')}
        expected={root.path}
        inputLabel={t('admin.libraries.folders.remove.inputLabel')}
        mismatchText={t('admin.libraries.folders.remove.mismatch', { path: root.path })}
        confirmLabel={t('admin.libraries.folders.change.confirm')}
        cancelLabel={t('common.cancel')}
        isPending={save.isPending}
        onConfirm={(confirmPath) => submit(confirmPath)}
      >
        {save.error && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
        <p>{t('admin.libraries.folders.change.content')}</p>
        <RemovalConsequences library={library} root={root} />
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
