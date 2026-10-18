import type { Library, LibraryRequest } from '@slouchcat/api-client';
import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formText } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Panel, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, RadioGroup, TextField } from '@/ui/aria';
import { LibraryCard } from './LibraryCard';
import styles from './LibrariesPage.module.css';

/** Poll interval while a scan is queued or running. */
export const SCAN_POLL_MS = 5000;

const isScanning = (library: Library) =>
  library.scan.state === 'queued' || library.scan.state === 'running';

export const librariesQuery = queryOptions({
  queryKey: ['admin', 'libraries'],
  queryFn: ({ signal }): Promise<Library[]> => request(api.GET('/admin/libraries', { signal })),
  refetchInterval: (query) => (query.state.data?.some(isScanning) ? SCAN_POLL_MS : false),
});

function AddLibraryForm() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [kind, setKind] = useState<'movies' | 'series'>('movies');
  const [formKey, setFormKey] = useState(0);
  const create = useMutation({
    mutationFn: (body: LibraryRequest): Promise<Library> =>
      request(api.POST('/admin/libraries', { body })),
    onSuccess: async () => {
      setFormKey((key) => key + 1);
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
    },
  });
  // The contract reports the path of the setup wizard as `library.path`; here it is `path`.
  const errors = create.error ? formErrors(t, create.error, { 'library.path': 'path' }) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    create.mutate({
      name: formText(data, 'name'),
      kind,
      path: formText(data, 'path'),
    });
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
        <RadioGroup
          name="kind"
          label={t('setup.library.kind')}
          value={kind}
          onChange={(value) => setKind(value === 'series' ? 'series' : 'movies')}
          options={[
            { value: 'movies', label: t('library.kind.movies') },
            { value: 'series', label: t('library.kind.series') },
          ]}
        />
        <TextField
          name="path"
          label={t('setup.library.path')}
          description={t('setup.library.pathHint')}
          autoComplete="off"
          spellCheck="false"
          mono
          isRequired
          validate={(value) => {
            if (!value.trim()) return t('errors.field.required');
            return value.trim().startsWith('/') ? null : t('errors.field.not_absolute');
          }}
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
  const actionError = scan.error ?? acceptLoss.error;

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
                  {library.scan.state === 'aborted' && (
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
