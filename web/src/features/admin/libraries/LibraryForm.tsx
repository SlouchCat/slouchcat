import type {
  Library,
  LibraryRequest,
  LibrarySave,
  LibrarySaved,
  LibraryTrickplay,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { Chip, Notice, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  Form,
  FormDialog,
  TextField,
  defaultProviders,
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
import { browseKey } from '../../shared/pathBrowser';
import {
  FolderMetadataFields,
  folderMetadataDraft,
  folderMetadataRequest,
  folderMetadataValues,
  hasOwnMetadata,
  type FolderMetadataDraft,
} from '../metadata/FolderMetadataFields';
import { useFolderMetadataText } from '../metadata/languages';
import { librariesQuery, metadataProvidersQuery, metadataSettingsQuery } from '../queries';
import {
  cardChanged,
  cardsWithErrors,
  confirmationsOf,
  editDirty,
  editState,
  folderChange,
  formFieldErrors,
  identifierChanges,
  pathWithContent,
  saveRequest,
  saveSummary,
  storedKey,
  storedRoot,
  type LibraryFormState,
  type SaveSummary,
} from './libraryForm';
import { RootFacts } from './RootFacts';
import { SaveConfirmDialog } from './SaveConfirmDialog';
import { DEFAULT_TRICKPLAY, sameTrickplay } from './TrickplayFields';
import { TrickplaySection } from './TrickplaySection';
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
  /** «Vorschau beim Spulen» of a video library (concept 18, 9). */
  trickplay: LibraryTrickplay;
  setTrickplay: (trickplay: LibraryTrickplay) => void;
  /** The languages and images of each video card by its key (concept 24 4.2); absent = default. */
  folderMetadata: Readonly<Record<string, FolderMetadataDraft>>;
  setFolderMetadata: (key: string, metadata: FolderMetadataDraft) => void;
  /** Kind and folders together: a new kind starts with its own cards. */
  changeKind: (kind: LibraryKind) => void;
  /** Something differs from an empty form (an untouched form is no draft). */
  dirty: boolean;
  reset: () => void;
}

/**
 * Whether the cards hold nothing typed: a single card without a path, whatever its content, with
 * the sources its content starts with (concept 20, 13.2). Adding a card and removing the first
 * leaves one for series; that is no draft.
 */
function untouchedFolders(folders: readonly FolderEntry[]): boolean {
  const [only] = folders;
  return (
    folders.length === 1 &&
    only !== undefined &&
    only.path.trim() === '' &&
    only.providers.join() === defaultProviders(only.content).join()
  );
}

/** The draft of «Bibliothek hinzufügen», held by the page. */
export function useLibraryDraft(): LibraryDraft {
  const [name, setName] = useState('');
  const [kind, setKind] = useState<LibraryKind>('video');
  const [rating, setRating] = useState('0');
  // The folders are controlled values: the browse dialog writes the chosen path into a card.
  const [folders, setFolders] = useState<FolderEntry[]>(() => initialFolders('video'));
  const [trickplay, setTrickplay] = useState<LibraryTrickplay>(DEFAULT_TRICKPLAY);
  const [folderMetadata, setMetadata] = useState<Record<string, FolderMetadataDraft>>({});
  const setFolderMetadata = useCallback(
    (key: string, metadata: FolderMetadataDraft) =>
      setMetadata((current) => ({ ...current, [key]: metadata })),
    [],
  );
  const changeKind = (next: LibraryKind) => {
    if (next === kind) return;
    setKind(next);
    setFolders(initialFolders(next));
    setMetadata({});
  };
  const reset = useCallback(() => {
    setName('');
    setKind('video');
    setRating('0');
    setFolders(initialFolders('video'));
    setTrickplay(DEFAULT_TRICKPLAY);
    setMetadata({});
  }, []);
  const dirty =
    name.trim() !== '' ||
    kind !== 'video' ||
    rating !== '0' ||
    !untouchedFolders(folders) ||
    !sameTrickplay(trickplay, DEFAULT_TRICKPLAY) ||
    folders.some((folder) => hasOwnMetadata(folderMetadata[folder.key]));
  return {
    name,
    setName,
    kind,
    rating,
    setRating,
    folders,
    setFolders,
    trickplay,
    setTrickplay,
    folderMetadata,
    setFolderMetadata,
    changeKind,
    dirty,
    reset,
  };
}

interface AddProps {
  mode: 'add';
  draft: LibraryDraft;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The library as the server created it; the dialog is closed by then. */
  onCreated: (library: Library) => void;
}

interface EditProps {
  mode: 'edit';
  /**
   * The library the form is filled with; the page mounts the form afresh at each opening. The form
   * keeps it while the page's list refetches: the revision it sends and every comparison (what
   * changed, what needs a confirmation) stay with what the reader saw.
   */
  library: Library;
  /** The library as the page's list has it now (absent: removed elsewhere), for the facts only. */
  live?: Library;
  /** The folder whose card is open at first (the pencil on «Hörbuch-Quellen», exception 1). */
  openRootId?: number;
  /** The form was filled anew because the library changed elsewhere (409 `stale`). */
  reloaded?: boolean;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The answer of `saveLibrary` and what the form changed; the dialog is closed by then. */
  onSaved: (saved: LibrarySaved, summary: SaveSummary) => void;
  /**
   * 409 `stale` or `root_unlisted`: the page loads the library again and fills the form anew
   * (true); false when the library is no longer there, and the form stays as it is.
   */
  onStale: () => Promise<boolean>;
}

export type LibraryFormProps = AddProps | EditProps;

/**
 * «Bibliothek hinzufügen» and «Bibliothek bearbeiten» (concept 24 4.1): one wide dialog with the
 * same fields, filled with the library on edit. Name; the kind, chosen on add and locked on edit;
 * one card per folder (FolderList: path with «Durchsuchen …», content, providers by position
 * S-16, of a video folder its languages and images), «Weiteren Ordner hinzufügen»; the default
 * rating of an audio library; «Vorschau beim Spulen» of a video library as a closed section. The
 * setup wizard adds its libraries with the same dialog.
 */
export function LibraryForm(props: LibraryFormProps) {
  return props.mode === 'add' ? <AddLibraryForm {...props} /> : <EditLibraryForm {...props} />;
}

/** The fields both forms share, in the order of table 4.1. */
interface FieldsProps {
  isEdit: boolean;
  /** The stored library on edit, as the form was filled: the hints of a change, the estimate. */
  library?: Library;
  /** The library as the page has it now: the facts of its folders (titles, files, watch). */
  live?: Library;
  kind: LibraryKind;
  onKindChange: (kind: LibraryKind) => void;
  name: string;
  onNameChange: (name: string) => void;
  rating: string;
  onRatingChange: (rating: string) => void;
  folders: FolderEntry[];
  onFoldersChange: (folders: FolderEntry[]) => void;
  folderMetadata: Readonly<Record<string, FolderMetadataDraft>>;
  onFolderMetadataChange: (key: string, metadata: FolderMetadataDraft) => void;
  trickplay: LibraryTrickplay;
  onTrickplayChange: (trickplay: LibraryTrickplay) => void;
  expanded: ReadonlySet<string>;
  onExpandedChange: (key: string, open: boolean) => void;
  marked?: ReadonlySet<string>;
  onMark?: (key: string) => void;
  onUndo?: (key: string) => void;
  fieldErrors?: Readonly<Record<string, string>>;
  /** Above the fields: the form's message, the notice of a reloaded form. */
  notices?: ReactNode;
  /** The cards that change path and content together (4.2: not in one step). */
  pathAndContent?: readonly string[];
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  actions: ReactNode;
}

const TRICKPLAY = ' trickplay';

function LibraryFormFields({
  isEdit,
  library,
  live,
  kind,
  onKindChange,
  name,
  onNameChange,
  rating,
  onRatingChange,
  folders,
  onFoldersChange,
  folderMetadata,
  onFolderMetadataChange,
  trickplay,
  onTrickplayChange,
  expanded,
  onExpandedChange,
  marked,
  onMark,
  onUndo,
  fieldErrors,
  notices,
  pathAndContent = [],
  onSubmit,
  actions,
}: FieldsProps) {
  const { t } = useTranslation();
  const providers = useQuery(metadataProvidersQuery);
  const metadataText = useFolderMetadataText();
  const video = kind === 'video';
  const kept = folders.filter((folder) => !marked?.has(folder.key));
  const listError = folderListError(t, kept, providers.data);
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));
  const state = (): LibraryFormState => ({ name, rating, trickplay, folders, folderMetadata });

  // A field of a closed card the browser refuses (a path left empty) opens its card, so the
  // message can be seen and the field reached (concept 24 5, exception 3).
  // `invalid` does not bubble, so the form listens in the capture phase.
  const formRef = useRef<HTMLFormElement>(null);
  useEffect(() => {
    const form = formRef.current;
    if (!form) return;
    const onInvalid = (event: Event) => {
      const target = event.target instanceof HTMLElement ? event.target : null;
      const key = target?.closest('[data-folder-key]')?.getAttribute('data-folder-key');
      if (key && !expanded.has(key)) onExpandedChange(key, true);
    };
    form.addEventListener('invalid', onInvalid, true);
    return () => form.removeEventListener('invalid', onInvalid, true);
  }, [expanded, onExpandedChange]);

  return (
    <Form ref={formRef} className={styles.form} onSubmit={onSubmit} validationErrors={fieldErrors}>
      {notices}
      <TextField
        name="name"
        label={t('setup.library.name')}
        isRequired
        validate={required}
        value={name}
        onChange={onNameChange}
      />
      <LibraryFields
        kind={kind}
        onKindChange={onKindChange}
        isEdit={isEdit}
        rating={rating}
        onRatingChange={onRatingChange}
        folders={folders}
        onFoldersChange={onFoldersChange}
        providerInfos={providers.data}
        fieldErrors={fieldErrors}
        ordering="rank"
        expandedKeys={expanded}
        onExpandedChange={onExpandedChange}
        marking={
          library && marked && onMark && onUndo
            ? {
                isStored: (folder) => storedRoot(library, folder.key) !== undefined,
                stored: (folder) => storedRoot(library, folder.key),
                marked,
                onMark,
                onUndo,
              }
            : undefined
        }
        summary={(folder) => {
          const values = folderMetadataValues(folderMetadata[folder.key] ?? folderMetadataDraft());
          return (
            <>
              {video && (
                <>
                  <span className={styles.muted}>{metadataText.langs(values)}</span>
                  <span className={styles.muted}>{metadataText.images(values)}</span>
                </>
              )}
              {library && cardChanged(library, state(), folder.key) && (
                <Chip tone="warning">{t('admin.libraries.form.changed')}</Chip>
              )}
            </>
          );
        }}
        pathButton={(folder, inputRef) => (
          <PathBrowserButton
            path={folder.path}
            onPathChange={(path) =>
              onFoldersChange(
                folders.map((entry) => (entry.key === folder.key ? { ...entry, path } : entry)),
              )
            }
            fieldRef={inputRef}
          />
        )}
        pathAddon={(folder) => {
          const root =
            library && ((live && storedRoot(live, folder.key)) ?? storedRoot(library, folder.key));
          return root ? <RootFacts root={root} /> : null;
        }}
        cardAddon={(folder, cardName) => {
          const root = library && storedRoot(library, folder.key);
          const change = root ? folderChange(root, folder, folderMetadata[folder.key]) : null;
          const [first] = folder.providers;
          return (
            <>
              {change && pathAndContent.includes(folder.key) && (
                <Notice tone="danger" live={false}>
                  {t('admin.libraries.form.pathAndContent')}
                </Notice>
              )}
              {change?.content && (
                <Notice tone="warning" live={false}>
                  {t('admin.libraries.folders.change.content')}
                </Notice>
              )}
              {root && first && identifierChanges(root, folder, providers.data) && (
                <Notice tone="info" live={false}>
                  {t('admin.libraries.folders.change.identifier', {
                    provider: t(`library.provider.${first}`),
                  })}
                </Notice>
              )}
              {change && !change.content && !change.identifier && change.providers && (
                <Notice live={false}>{t('admin.libraries.folders.change.fillers')}</Notice>
              )}
              {change?.path && !change.content && (
                <Text tone="secondary" variant="caption">
                  {t('admin.libraries.form.pathHint')}
                </Text>
              )}
              {video && (
                <FolderMetadataFields
                  name={folder.key}
                  value={folderMetadata[folder.key] ?? folderMetadataDraft()}
                  onChange={(metadata) => onFolderMetadataChange(folder.key, metadata)}
                  fieldPrefix={`${cardName}.`}
                  fieldErrors={fieldErrors}
                />
              )}
            </>
          );
        }}
      />
      {video && (
        <TrickplaySection
          value={trickplay}
          onChange={onTrickplayChange}
          library={library}
          isExpanded={expanded.has(TRICKPLAY)}
          onExpandedChange={(open) => onExpandedChange(TRICKPLAY, open)}
        />
      )}
      {/* The reason whenever it disables «Speichern», a card without a path included (Z-5). */}
      {listError && kept.length > 0 && (
        <Notice tone="warning" live={false}>
          {listError}
        </Notice>
      )}
      {actions}
    </Form>
  );
}

/** The trickplay section's key among the open parts of the form. */
const opensTrickplay = (fields: Readonly<Record<string, string>>) =>
  Object.keys(fields).some((field) => field.startsWith('trickplay'));

/**
 * «Bibliothek hinzufügen» (BIB-6). The draft is the page's, so what was typed is still there when
 * the dialog is closed without adding and opened again, and leaving the page asks first (S-13);
 * a library that was added empties it. «Abbrechen» asks whether to keep or discard a draft;
 * Escape keeps it without asking (concept 20, 13.2). Every card is new, so every card is open
 * until it is closed (concept 24 5, exception 2).
 */
function AddLibraryForm({ draft, isOpen, onOpenChange, onCreated }: AddProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { name, setName, kind, rating, setRating, folders, setFolders, changeKind } = draft;
  const { trickplay, setTrickplay, folderMetadata, setFolderMetadata } = draft;
  const [asking, setAsking] = useState(false);
  // The parts the reader closed: a card is new and open unless it is in here.
  const [closed, setClosed] = useState<ReadonlySet<string>>(() => new Set([TRICKPLAY]));
  const providers = useQuery({ ...metadataProvidersQuery, enabled: isOpen });
  const create = useMutation({
    mutationFn: (body: LibraryRequest): Promise<Library> =>
      request(api.POST('/admin/libraries', { body })),
    onSuccess: async (library) => {
      draft.reset();
      setClosed(new Set([TRICKPLAY]));
      onOpenChange(false);
      onCreated(library);
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
    },
    onError: (error) => {
      const { fields } = formErrors(t, error);
      const open = cardsWithErrors(fields, {
        name,
        rating,
        trickplay,
        folders,
        folderMetadata,
      });
      setClosed((current) => {
        const next = new Set([...current].filter((key) => !open.includes(key)));
        if (opensTrickplay(fields)) next.delete(TRICKPLAY);
        return next;
      });
    },
  });
  // The contract names a folder by its index (`roots[1].path`), and so do the inputs of the
  // cards; a message for the content or the list of a card is shown by the card itself.
  const errors = create.error ? formErrors(t, create.error) : null;
  const listError = folderListError(t, folders, providers.data);
  const expanded = new Set(
    [...folders.map((folder) => folder.key), TRICKPLAY].filter((key) => !closed.has(key)),
  );

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Only video folders have languages and images of their own (concept 24 3.1).
    const body = libraryRequest(
      name.trim(),
      kind,
      folders,
      rating,
      kind === 'video' ? (folder) => folderMetadataRequest(folderMetadata[folder.key]) : undefined,
    );
    create.mutate(kind === 'video' ? { ...body, trickplay } : body);
  };
  // A message belongs to the request that is over once the dialog closes; the draft stays.
  const close = () => {
    create.reset();
    onOpenChange(false);
  };
  // «Abbrechen» with something entered: keep the draft (the page's band offers it again) or
  // discard it. Only «Entwurf verwerfen» empties it; the other ways out of the question keep it.
  const cancel = () => {
    if (draft.dirty) setAsking(true);
    else close();
  };
  const submitText = create.isPending
    ? t('admin.libraries.add.submitting')
    : folders.length === 0
      ? t('library.roots.submitNone')
      : t('library.roots.submit', { count: folders.length });

  return (
    <>
      <FormDialog
        isOpen={isOpen}
        onOpenChange={(open) => (open ? onOpenChange(true) : close())}
        title={t('admin.libraries.add.title')}
        size="wide"
      >
        <LibraryFormFields
          isEdit={false}
          kind={kind}
          onKindChange={changeKind}
          name={name}
          onNameChange={setName}
          rating={rating}
          onRatingChange={setRating}
          folders={folders}
          onFoldersChange={(next) => {
            // The server names a card by its index: a message must not move to another card.
            if (create.error) create.reset();
            setFolders(() => next);
          }}
          folderMetadata={folderMetadata}
          onFolderMetadataChange={setFolderMetadata}
          trickplay={trickplay}
          onTrickplayChange={setTrickplay}
          expanded={expanded}
          onExpandedChange={(key, open) =>
            setClosed((current) => {
              const next = new Set(current);
              if (open) next.delete(key);
              else next.add(key);
              return next;
            })
          }
          fieldErrors={errors?.fields}
          notices={errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          onSubmit={onSubmit}
          actions={
            <div className={styles.dialogActions}>
              <AriaButton variant="ghost" onPress={cancel}>
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
          }
        />
      </FormDialog>
      <ConfirmDialog
        isOpen={asking}
        onOpenChange={(open) => {
          if (open) return;
          setAsking(false);
          close();
        }}
        title={t('admin.libraries.add.draft.question.title')}
        body={t('admin.libraries.add.draft.question.body')}
        confirmLabel={t('admin.libraries.add.draft.question.discard')}
        cancelLabel={t('admin.libraries.add.draft.question.keep')}
        tone="danger"
        defaultFocus="cancel"
        onConfirm={draft.reset}
      />
    </>
  );
}

/** The parts of a failed `saveLibrary` the form shows, with the request it answers. */
interface Refusal {
  error: unknown;
  sent: { roots: string[]; removals: string[] };
}

/**
 * «Bibliothek bearbeiten» (concept 24 4.3): the library's values in the same fields as on add,
 * every folder card closed. Nothing takes effect before «Speichern»: «Ordner entfernen …» only
 * marks the card, a new path, content or source list waits in the card. «Speichern» saves all of
 * it in one `saveLibrary`; removals, other contents and new paths are confirmed first in
 * «Änderungen bestätigen». A stale form (409 `stale`, `root_unlisted`) is filled anew by the page;
 * a running scan (409 `scan_running`) keeps every input and offers «Scan abbrechen». Field
 * messages land on their fields and open the card that holds them. «Abbrechen» and Escape with
 * changes ask «Änderungen verwerfen?»; there is no kept draft of an edit.
 */
function EditLibraryForm({
  library: opened,
  live,
  openRootId,
  reloaded,
  isOpen,
  onOpenChange,
  onSaved,
  onStale,
}: EditProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // The library as the form was filled with it (finding of the review: the list refetches while
  // the form is open). A change elsewhere is shown, and the save answers 409 `stale` for it.
  const [library] = useState(opened);
  const [state, setState] = useState<LibraryFormState>(() => editState(library));
  const [marked, setMarked] = useState<ReadonlySet<string>>(() => new Set());
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(
    () => new Set(openRootId !== undefined ? [storedKey(openRootId)] : []),
  );
  const [typed, setTyped] = useState<Record<string, string>>({});
  const [confirming, setConfirming] = useState(false);
  // A typed path confirms the change the dialog listed when it was typed. It stays while the dialog
  // stays open across a refusal (409 `scan_running`, a wrong typed path) and goes when it closes,
  // so no later «Speichern» opens it filled in.
  const endConfirming = () => {
    setConfirming(false);
    setTyped({});
  };
  const [asking, setAsking] = useState(false);
  const [refusal, setRefusal] = useState<Refusal | null>(null);
  // The reload after `stale` found the library removed elsewhere.
  const [gone, setGone] = useState(false);
  // The cards whose path and content changed together, as the last «Speichern» found them.
  const [blocked, setBlocked] = useState<readonly string[]>([]);
  const providers = useQuery({ ...metadataProvidersQuery, enabled: isOpen });
  const dirty = editDirty(library, state, marked);
  const confirmations = confirmationsOf(library, state, marked);
  const kept = state.folders.filter((folder) => !marked.has(folder.key));
  const listError = folderListError(t, kept, providers.data);
  const changedElsewhere = live !== undefined && live.revision !== library.revision;

  const open = (keys: readonly string[]) =>
    setExpanded((current) => new Set([...current, ...keys]));
  const setOpen = (key: string, isOpen: boolean) =>
    setExpanded((current) => {
      const next = new Set(current);
      if (isOpen) next.add(key);
      else next.delete(key);
      return next;
    });
  const change = (patch: Partial<LibraryFormState>) => {
    // The server names a card by its index: a message must not move to another card.
    if (refusal && 'folders' in patch) setRefusal(null);
    setState((current) => ({ ...current, ...patch }));
  };

  const save = useMutation({
    mutationFn: (body: LibrarySave): Promise<LibrarySaved> =>
      request(api.PUT('/admin/libraries/{id}', { params: { path: { id: library.id } }, body })),
    onSuccess: async (saved) => {
      const summary = saveSummary(library, state, marked);
      endConfirming();
      onOpenChange(false);
      onSaved(saved, summary);
      // The browse dialog marks every folder of a library; a folder's own languages widen the
      // profiles' choice (concept 24 3.3), and the name shows in the navigation and the start
      // page («Neu in …»).
      await Promise.all(
        [
          librariesQuery.queryKey,
          browseKey,
          metadataSettingsQuery.queryKey,
          ['session'],
          ['libraries'],
          ['home'],
          ['homeList'],
          ['library'],
          ['admin', 'library', library.id],
        ].map((queryKey) => queryClient.invalidateQueries({ queryKey })),
      );
    },
  });
  const cancelScan = useMutation({
    mutationFn: (): Promise<Library> =>
      request(api.DELETE('/admin/libraries/{id}/scan', { params: { path: { id: library.id } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey }),
  });

  const send = () => {
    const sent = saveRequest(library, state, marked, typed);
    setRefusal(null);
    setGone(false);
    cancelScan.reset();
    save.mutate(sent.body, {
      onError: (error) => {
        if (isApiError(error) && (error.code === 'stale' || error.code === 'root_unlisted')) {
          endConfirming();
          void onStale().then((filled) => {
            if (!filled) setGone(true);
          });
          return;
        }
        setRefusal({ error, sent: { roots: sent.roots, removals: sent.removals } });
        if (isApiError(error) && error.code === 'scan_running') return;
        const fields = formFieldErrors(formErrors(t, error).fields, state, sent);
        // A wrong typed path is the confirmation's business; anything else goes to the form.
        if (Object.keys(fields).some((field) => field.startsWith('confirm.'))) return;
        endConfirming();
        open(cardsWithErrors(fields, state));
        if (opensTrickplay(fields)) setOpen(TRICKPLAY, true);
      },
    });
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (save.isPending) return;
    // Nothing changed: nothing to send (decision 18), the dialog closes as after a save.
    if (!dirty) {
      onOpenChange(false);
      return;
    }
    if (listError) return;
    const together = pathWithContent(library, state, marked);
    setBlocked(together);
    if (together.length > 0) {
      open(together);
      return;
    }
    if (confirmations.length > 0) setConfirming(true);
    else send();
  };

  const refusedFields = refusal
    ? formFieldErrors(formErrors(t, refusal.error).fields, state, refusal.sent)
    : {};
  const refused = refusal?.error;
  const scanRunning = isApiError(refused) && refused.code === 'scan_running';
  // 404 or a reload that no longer finds it: the library was removed elsewhere while the form
  // was open.
  const formMessage = gone
    ? t('admin.libraries.form.gone')
    : refusal && !scanRunning && !confirming
      ? isApiError(refused, 404)
        ? t('admin.libraries.form.gone')
        : formErrors(t, refusal.error).form
      : null;
  const requestClose = () => {
    if (dirty) setAsking(true);
    else onOpenChange(false);
  };
  const last = library.roots.length > 0 && state.folders.every((folder) => marked.has(folder.key));

  return (
    <>
      <FormDialog
        isOpen={isOpen}
        onOpenChange={(next) => (next ? onOpenChange(true) : requestClose())}
        title={t('admin.libraries.form.editTitle')}
        description={library.name}
        initialFocus={
          openRootId !== undefined
            ? `[data-folder-key="${storedKey(openRootId)}"] input[name$=".path"]`
            : undefined
        }
        size="wide"
      >
        <LibraryFormFields
          isEdit
          library={library}
          live={live}
          kind={library.kind}
          onKindChange={() => undefined}
          name={state.name}
          onNameChange={(name) => change({ name })}
          rating={state.rating}
          onRatingChange={(rating) => change({ rating })}
          folders={state.folders}
          onFoldersChange={(folders) => change({ folders })}
          folderMetadata={state.folderMetadata}
          onFolderMetadataChange={(key, metadata) =>
            change({ folderMetadata: { ...state.folderMetadata, [key]: metadata } })
          }
          trickplay={state.trickplay}
          onTrickplayChange={(trickplay) => change({ trickplay })}
          expanded={expanded}
          onExpandedChange={setOpen}
          marked={marked}
          onMark={(key) => setMarked((current) => new Set([...current, key]))}
          onUndo={(key) =>
            setMarked((current) => new Set([...current].filter((entry) => entry !== key)))
          }
          fieldErrors={refusedFields}
          pathAndContent={blocked}
          notices={
            <>
              {reloaded && <Notice tone="warning">{t('admin.libraries.form.reloaded')}</Notice>}
              {changedElsewhere && (
                <Notice tone="warning">{t('admin.libraries.form.changedElsewhere')}</Notice>
              )}
              {formMessage && <Notice tone="danger">{formMessage}</Notice>}
            </>
          }
          onSubmit={onSubmit}
          actions={
            <div className={styles.dialogActions}>
              <AriaButton variant="ghost" onPress={requestClose}>
                {t('common.cancel')}
              </AriaButton>
              <AriaButton
                type="submit"
                variant="primary"
                isPending={save.isPending && !confirming}
                isDisabled={listError !== null && kept.length > 0}
              >
                {save.isPending ? t('common.saving') : t('common.save')}
              </AriaButton>
            </div>
          }
        />
      </FormDialog>
      <SaveConfirmDialog
        isOpen={confirming}
        onOpenChange={(next) => {
          if (next || save.isPending) return;
          endConfirming();
          if (scanRunning) setRefusal(null);
        }}
        library={library}
        confirmations={confirmations}
        last={last}
        typed={typed}
        onTypedChange={(key, value) => setTyped((current) => ({ ...current, [key]: value }))}
        fieldErrors={refusedFields}
        isPending={save.isPending}
        onConfirm={send}
        problem={
          scanRunning ? (
            <Notice
              tone="warning"
              title={problemMessage(t, refusal?.error)}
              actions={
                <AriaButton
                  icon="close"
                  isPending={cancelScan.isPending}
                  onPress={() => cancelScan.mutate()}
                >
                  {t('admin.libraries.cancel')}
                </AriaButton>
              }
            >
              {cancelScan.isSuccess
                ? t('admin.libraries.form.scanCancelled')
                : cancelScan.isError
                  ? problemMessage(t, cancelScan.error)
                  : t('admin.libraries.form.scanRunning')}
            </Notice>
          ) : undefined
        }
      />
      <ConfirmDialog
        isOpen={asking}
        onOpenChange={(next) => {
          if (!next) setAsking(false);
        }}
        title={t('admin.libraries.form.discard.title')}
        body={t('admin.libraries.form.discard.body', { name: library.name })}
        confirmLabel={t('admin.libraries.form.discard.confirm')}
        cancelLabel={t('admin.libraries.form.discard.keep')}
        tone="danger"
        defaultFocus="cancel"
        onConfirm={() => onOpenChange(false)}
      />
    </>
  );
}
