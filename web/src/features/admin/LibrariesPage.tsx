import type { Library, LibraryRequest, Rating } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formText } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Panel, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from '../shared/choices';
import {
  isAudioLibrary,
  LibraryFields,
  libraryRequest,
  type LibraryKind,
} from '../shared/LibraryFields';
import { PathBrowserButton } from '../shared/PathBrowserButton';
import { LibraryCard } from './LibraryCard';
import { LibraryFoldersDialog } from './LibraryFoldersDialog';
import { isScanning, librariesQuery } from './queries';
import styles from './LibrariesPage.module.css';

function AddLibraryForm() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState<LibraryKind>('movies');
  const [rating, setRating] = useState('0');
  // The folder is a controlled value, so the browse dialog can write the chosen path into it.
  const [path, setPath] = useState('');
  const [formKey, setFormKey] = useState(0);
  const pathInput = useRef<HTMLInputElement>(null);
  const create = useMutation({
    mutationFn: (body: LibraryRequest): Promise<Library> =>
      request(api.POST('/admin/libraries', { body })),
    onSuccess: async () => {
      setFormKey((key) => key + 1);
      setRating('0');
      setPath('');
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
    },
  });
  // The contract reports the path of the setup wizard as `library.path`; here it is `path`.
  const errors = create.error ? formErrors(t, create.error, { 'library.path': 'path' }) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    create.mutate(libraryRequest(formText(data, 'name'), kind, formText(data, 'path'), rating));
  };
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Panel as="section" aria-labelledby="add-library-heading">
      <h2 id="add-library-heading" className={styles.panelTitle}>
        {t('admin.libraries.add.title')}
      </h2>
      <Form
        key={formKey}
        className={styles.form}
        onSubmit={onSubmit}
        validationErrors={errors?.fields}
      >
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField name="name" label={t('setup.library.name')} isRequired validate={required} />
        <LibraryFields
          kind={kind}
          onKindChange={setKind}
          rating={rating}
          onRatingChange={setRating}
          path={path}
          onPathChange={setPath}
          pathInputRef={pathInput}
          pathAddon={<PathBrowserButton path={path} onPathChange={setPath} fieldRef={pathInput} />}
        />
        <div>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending
              ? t('admin.libraries.add.submitting')
              : t('admin.libraries.add.submit')}
          </AriaButton>
        </div>
      </Form>
    </Panel>
  );
}

/**
 * "Standardstufe ändern" of an audio library (AN-2): every title whose rating comes from the
 * library follows; titles rated by hand keep theirs (the server decides).
 */
function DefaultRatingAction({
  library,
  onSaved,
}: {
  library: Library;
  onSaved: (library: Library) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const [rating, setRating] = useState(String(library.defaultRating ?? 0));
  const save = useMutation({
    mutationFn: (defaultRating: Rating): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}', {
          params: { path: { id: library.id } },
          body: { defaultRating },
        }),
      ),
    onSuccess: (updated) => {
      setOpen(false);
      onSaved(updated);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = titleRatingOf(rating);
    if (value !== undefined) save.mutate(value);
  };
  return (
    <>
      <AriaButton
        variant="ghost"
        aria-label={t('admin.libraries.changeRating.label', { name: library.name })}
        onPress={() => {
          setRating(String(library.defaultRating ?? 0));
          save.reset();
          setOpen(true);
        }}
      >
        {t('admin.libraries.changeRating.action')}
      </AriaButton>
      <FormDialog
        isOpen={open}
        onOpenChange={setOpen}
        title={t('admin.libraries.changeRating.title', { name: library.name })}
        description={t('admin.libraries.changeRating.description')}
      >
        <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <RadioGroup
            name="defaultRating"
            label={t('library.defaultRating')}
            value={rating}
            onChange={setRating}
            options={ratingOptions(t)}
          />
          <div className={styles.actions}>
            <AriaButton variant="ghost" onPress={() => setOpen(false)}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isPending={save.isPending}>
              {t('admin.libraries.changeRating.submit')}
            </AriaButton>
          </div>
        </Form>
      </FormDialog>
    </>
  );
}

/**
 * "Folders" of one library: the card names how many there are, this opens the dialog that lists
 * them and adds or removes one (idea 02, E16). A podcast library has none, so it has no button.
 */
function FoldersAction({
  library,
  onChanged,
}: {
  library: Library;
  onChanged: (library: Library) => void;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <>
      <AriaButton
        variant="ghost"
        icon="folder"
        aria-label={t('admin.libraries.openFoldersLabel', { name: library.name })}
        onPress={() => setOpen(true)}
      >
        {t('admin.libraries.openFolders')}
      </AriaButton>
      <LibraryFoldersDialog
        library={library}
        isOpen={open}
        onOpenChange={setOpen}
        onChanged={onChanged}
      />
    </>
  );
}

/** Minimal library administration for M1a (A-1, B-1): scan state, rescan, loss protection. */
export function LibrariesPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.libraries.title'));
  const queryClient = useQueryClient();
  const libraries = useQuery(librariesQuery);

  const refresh = (library: Library) => {
    queryClient.setQueryData<Library[]>(librariesQuery.queryKey, (list) =>
      list?.map((entry) => (entry.id === library.id ? library : entry)),
    );
    return queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
  };
  const scan = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.POST('/admin/libraries/{id}/scan', { params: { path: { id } } })),
    onSuccess: refresh,
  });
  const acceptLoss = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.POST('/admin/libraries/{id}/accept-loss', { params: { path: { id } } })),
    onSuccess: refresh,
  });
  const cancel = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.DELETE('/admin/libraries/{id}/scan', { params: { path: { id } } })),
    onSuccess: refresh,
  });
  const actionError = scan.error ?? acceptLoss.error ?? cancel.error;
  const [ratingSaved, setRatingSaved] = useState<Library | null>(null);

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.libraries.title')}
        </Text>
        <Text tone="secondary">{t('admin.libraries.intro')}</Text>
      </div>

      <div className={styles.layout}>
        <section className={styles.list} aria-busy={libraries.isPending}>
          {actionError && <Notice tone="danger">{problemMessage(t, actionError)}</Notice>}
          {ratingSaved?.defaultRating !== undefined && (
            <Notice>
              {t('admin.libraries.changeRating.saved', {
                name: ratingSaved.name,
                rating: t('rating.fsk', { age: ratingSaved.defaultRating }),
              })}
            </Notice>
          )}
          {libraries.isPending && <Skeleton shape="block" />}
          {libraries.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, libraries.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void libraries.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {libraries.data?.length === 0 && <Notice>{t('admin.libraries.empty')}</Notice>}
          {libraries.data?.map((library) => (
            <LibraryCard
              key={library.id}
              library={library}
              actions={
                <>
                  <AriaButton
                    icon="refresh"
                    isDisabled={isScanning(library)}
                    isPending={scan.isPending && scan.variables === library.id}
                    onPress={() => scan.mutate(library.id)}
                  >
                    {t('admin.libraries.rescan')}
                  </AriaButton>
                  {isScanning(library) && (
                    <AriaButton
                      variant="ghost"
                      isPending={cancel.isPending && cancel.variables === library.id}
                      onPress={() => cancel.mutate(library.id)}
                    >
                      {t('admin.libraries.cancel')}
                    </AriaButton>
                  )}
                  {library.kind !== 'podcasts' && (
                    <FoldersAction
                      library={library}
                      onChanged={(updated) => void refresh(updated)}
                    />
                  )}
                  {isAudioLibrary(library.kind) && (
                    <DefaultRatingAction
                      library={library}
                      onSaved={(updated) => {
                        setRatingSaved(updated);
                        void refresh(updated);
                      }}
                    />
                  )}
                  {library.scan.state === 'aborted' && library.scan.lastError !== 'cancelled' && (
                    <ConfirmDialog
                      trigger={
                        <AriaButton variant="ghost" icon="warning">
                          {t('admin.libraries.acceptLoss.action')}
                        </AriaButton>
                      }
                      title={t('admin.libraries.acceptLoss.title')}
                      body={t('admin.libraries.acceptLoss.body')}
                      confirmLabel={t('admin.libraries.acceptLoss.confirm')}
                      cancelLabel={t('common.cancel')}
                      tone="danger"
                      onConfirm={() => acceptLoss.mutate(library.id)}
                    />
                  )}
                </>
              }
            />
          ))}
        </section>
        <aside className={styles.aside}>
          <AddLibraryForm />
        </aside>
      </div>
    </Page>
  );
}
