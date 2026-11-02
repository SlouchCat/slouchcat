import type { Library, LibraryRequest } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { Notice } from '@/ui';
import {
  AriaButton,
  Form,
  FormDialog,
  TextField,
  folderListError,
  type FolderEntry,
} from '@/ui/aria';
import {
  initialFolders,
  LibraryFields,
  libraryRequest,
  type LibraryKind,
} from '../../shared/LibraryFields';
import { PathBrowserButton } from '../../shared/PathBrowserButton';
import { librariesQuery, metadataProvidersQuery } from '../queries';
import styles from './Libraries.module.css';

/** What «Bibliothek hinzufügen» holds; the page owns it, so it outlives the dialog (S-13, S-17). */
export interface LibraryDraft {
  name: string;
  setName: (name: string) => void;
  kind: LibraryKind;
  rating: string;
  setRating: (rating: string) => void;
  folders: FolderEntry[];
  setFolders: (update: (folders: FolderEntry[]) => FolderEntry[]) => void;
  /** Kind and folders together: a new kind starts with its own cards. */
  changeKind: (kind: LibraryKind) => void;
  /** Something differs from an empty form (an untouched form is no draft). */
  dirty: boolean;
  reset: () => void;
}

/** Whether the cards are the ones a new video library starts with, apart from their keys. */
function untouchedFolders(folders: readonly FolderEntry[]): boolean {
  const [start] = initialFolders('video');
  const [only] = folders;
  return (
    folders.length === 1 &&
    only !== undefined &&
    start !== undefined &&
    only.path.trim() === '' &&
    only.content === start.content &&
    only.providers.join() === start.providers.join()
  );
}

/** The draft of «Bibliothek hinzufügen», held by the page. */
export function useLibraryDraft(): LibraryDraft {
  const [name, setName] = useState('');
  const [kind, setKind] = useState<LibraryKind>('video');
  const [rating, setRating] = useState('0');
  // The folders are controlled values: the browse dialog writes the chosen path into a card.
  const [folders, setFolders] = useState<FolderEntry[]>(() => initialFolders('video'));
  const changeKind = (next: LibraryKind) => {
    if (next === kind) return;
    setKind(next);
    setFolders(initialFolders(next));
  };
  const reset = useCallback(() => {
    setName('');
    setKind('video');
    setRating('0');
    setFolders(initialFolders('video'));
  }, []);
  const dirty =
    name.trim() !== '' || kind !== 'video' || rating !== '0' || !untouchedFolders(folders);
  return { name, setName, kind, rating, setRating, folders, setFolders, changeKind, dirty, reset };
}

interface AddLibraryDialogProps {
  draft: LibraryDraft;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The library as the server created it; the dialog is closed by then. */
  onCreated: (library: Library) => void;
}

/**
 * «Bibliothek hinzufügen» (BIB-6; concept 12, 11.5 point 8: from the page into a dialog): name,
 * kind, default rating of an audio library, the folder cards with their sources (by position,
 * S-16) and the folder browser, «N Ordner hinzufügen». The draft is the page's, so what was
 * typed is still there when the dialog is closed without adding and opened again, and leaving the
 * page asks first (S-13); a library that was added empties it.
 */
export function AddLibraryDialog({
  draft,
  isOpen,
  onOpenChange,
  onCreated,
}: AddLibraryDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { name, setName, kind, rating, setRating, folders, setFolders, changeKind } = draft;
  const providers = useQuery({ ...metadataProvidersQuery, enabled: isOpen });
  const create = useMutation({
    mutationFn: (body: LibraryRequest): Promise<Library> =>
      request(api.POST('/admin/libraries', { body })),
    onSuccess: async (library) => {
      draft.reset();
      onOpenChange(false);
      onCreated(library);
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
    },
  });
  // The contract names a folder by its index (`roots[1].path`), and so do the inputs of the
  // cards; a message for the content or the list of a card is shown by the card itself.
  const errors = create.error ? formErrors(t, create.error) : null;
  const listError = folderListError(t, folders, providers.data);

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate(libraryRequest(name.trim(), kind, folders, rating));
  };
  // A message belongs to the request that is over once the dialog closes; the draft stays.
  const close = () => {
    create.reset();
    onOpenChange(false);
  };
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));
  const submitText = create.isPending
    ? t('admin.libraries.add.submitting')
    : folders.length === 0
      ? t('library.roots.submitNone')
      : t('library.roots.submit', { count: folders.length });

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={(open) => (open ? onOpenChange(true) : close())}
      title={t('admin.libraries.add.title')}
      size="wide"
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('setup.library.name')}
          isRequired
          validate={required}
          value={name}
          onChange={setName}
        />
        <LibraryFields
          kind={kind}
          onKindChange={changeKind}
          rating={rating}
          onRatingChange={setRating}
          folders={folders}
          onFoldersChange={(next) => {
            // The server names a card by its index: a message must not move to another card.
            if (create.error) create.reset();
            setFolders(() => next);
          }}
          providerInfos={providers.data}
          fieldErrors={errors?.fields}
          ordering="rank"
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
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={close}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            icon="plus"
            isPending={create.isPending}
            isDisabled={listError !== null && folders.length > 0}
          >
            {submitText}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
