import type {
  AdminCollection,
  AdminCollectionItem,
  AdminItemHit,
  CollectionRequest,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formText } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Page, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, DataTable, Form, FormDialog, TextField, type DataColumn } from '@/ui/aria';
import { moveBy, titleMeta, TitleSearch, useMoveButtons } from './CurationParts';
import {
  adminCollectionQuery,
  collectionsQuery,
  createCollection,
  deleteCollection,
  renameCollection,
  setCollectionItems,
} from './queries';
import styles from './AdminTablePage.module.css';
import curation from './Curation.module.css';

/** Longest collection name the server takes (`CollectionRequest.name`). */
const MAX_NAME = 120;
/** Kinds a collection may hold: seasons, episodes and extras are refused by the server. */
const MEMBER_KINDS: readonly AdminItemHit['kind'][] = ['movie', 'series', 'audiobook', 'podcast'];

/** "Sammlung anlegen" and "Umbenennen": one name field. */
function NameDialog({
  collection,
  onClose,
  onSaved,
}: {
  /** The collection to rename; absent for a new one. */
  collection?: AdminCollection;
  onClose: () => void;
  onSaved: (saved: AdminCollection) => void;
}) {
  const { t } = useTranslation();
  const save = useMutation({
    mutationFn: (body: CollectionRequest) =>
      collection ? renameCollection(collection.id, body) : createCollection(body),
    onSuccess: onSaved,
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate({ name: formText(new FormData(event.currentTarget), 'name') });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={
        collection
          ? t('admin.collections.rename.title', { name: collection.name })
          : t('admin.collections.create.title')
      }
      description={collection ? undefined : t('admin.collections.create.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('admin.collections.form.name')}
          defaultValue={collection?.name}
          maxLength={MAX_NAME}
          autoComplete="off"
          isRequired
          // A server error sits on this field until the name changes (as on the podcasts page).
          onChange={() => {
            if (save.error) save.reset();
          }}
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            icon={collection ? undefined : 'plus'}
            isPending={save.isPending}
          >
            {save.isPending
              ? t('common.saving')
              : collection
                ? t('common.save')
                : t('admin.collections.create.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * "Mitglieder": the titles of a manual collection in order. The search adds titles at the end,
 * every entry moves up or down or leaves; "Speichern" replaces the members at once.
 */
function MembersDialog({
  collection,
  onClose,
  onSaved,
}: {
  collection: AdminCollection;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const stored = useQuery(adminCollectionQuery(collection.id));
  const [edited, setEdited] = useState<AdminCollectionItem[] | null>(null);
  const members = edited ?? stored.data?.items ?? [];
  const { buttonRef, keepFocus } = useMoveButtons(members.map((member) => member.itemId).join());
  const searchRef = useRef<HTMLDivElement>(null);
  const save = useMutation({
    mutationFn: (items: AdminCollectionItem[]) =>
      setCollectionItems(
        collection.id,
        items.map((item) => item.itemId),
      ),
    onSuccess: onSaved,
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const move = (index: number, delta: -1 | 1) => {
    const member = members[index];
    if (!member) return;
    setEdited(moveBy(members, index, delta));
    keepFocus(member.itemId, delta);
  };
  // The removed entry and its buttons are gone: the focus goes back to the search field.
  const focusSearch = () => searchRef.current?.querySelector('input')?.focus();
  const removeAt = (index: number) => {
    setEdited(members.filter((_member, position) => position !== index));
    focusSearch();
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate(members);
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.collections.members.title', { name: collection.name })}
      description={t('admin.collections.members.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        {/* The search adds to the stored members: it waits for them, or a pick made while they
            load would replace the whole collection on "Speichern". */}
        {stored.isSuccess && (
          <div ref={searchRef}>
            <TitleSearch
              autoFocus
              label={t('admin.collections.members.search.label')}
              hint={t('admin.collections.members.search.hint')}
              emptyText={t('admin.collections.members.search.empty')}
              resultsLabel={t('admin.collections.members.search.results')}
              actionText={t('admin.collections.members.add')}
              actionLabel={(hit) => t('admin.collections.members.addLabel', { title: hit.title })}
              onPick={(hit) => {
                setEdited([
                  ...members,
                  {
                    itemId: hit.id,
                    kind: hit.kind,
                    title: hit.title,
                    year: hit.year,
                    position: members.length,
                  },
                ]);
              }}
              exclude={new Set(members.map((member) => member.itemId))}
              kinds={MEMBER_KINDS}
            />
          </div>
        )}
        {stored.isPending && <Skeleton shape="block" />}
        {stored.isError && <Notice tone="danger">{problemMessage(t, stored.error)}</Notice>}
        {stored.isSuccess && members.length === 0 && (
          <p className={curation.muted}>{t('admin.collections.members.empty')}</p>
        )}
        {members.length > 0 && (
          <ol className={curation.list} aria-label={t('admin.collections.members.list')}>
            {members.map((member, index) => (
              <li key={member.itemId} className={curation.entry}>
                <span className={curation.stack}>
                  <span>{member.title}</span>
                  <span className={curation.muted}>{titleMeta(t, member)}</span>
                </span>
                <span className={curation.actions}>
                  <AriaButton
                    ref={buttonRef(member.itemId, -1)}
                    variant="ghost"
                    aria-label={t('admin.collections.members.upLabel', { title: member.title })}
                    isDisabled={index === 0}
                    onPress={() => move(index, -1)}
                  >
                    {t('admin.collections.members.up')}
                  </AriaButton>
                  <AriaButton
                    ref={buttonRef(member.itemId, 1)}
                    variant="ghost"
                    aria-label={t('admin.collections.members.downLabel', { title: member.title })}
                    isDisabled={index === members.length - 1}
                    onPress={() => move(index, 1)}
                  >
                    {t('admin.collections.members.down')}
                  </AriaButton>
                  <AriaButton
                    variant="ghost"
                    icon="trash"
                    aria-label={t('admin.collections.members.removeLabel', {
                      title: member.title,
                    })}
                    onPress={() => removeAt(index)}
                  >
                    {t('admin.collections.members.remove')}
                  </AriaButton>
                </span>
              </li>
            ))}
          </ol>
        )}
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            isPending={save.isPending}
            isDisabled={!stored.isSuccess}
          >
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

type Dialog =
  | { kind: 'create' }
  | { kind: 'rename'; collection: AdminCollection }
  | { kind: 'members'; collection: AdminCollection };

/**
 * Collections (M7, E-6): automatic ones from TMDB (read only) and manual ones of the main
 * account, with source and number of titles; manual ones can be created, renamed, filled,
 * ordered and deleted.
 */
export function CollectionsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.collections.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const collections = useQuery({ ...collectionsQuery, enabled: allowed });
  const [dialog, setDialog] = useState<Dialog | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const header = useRef<HTMLDivElement>(null);
  const createButton = useRef<HTMLButtonElement>(null);

  // Every viewer place that shows collections changes with an edit: the start page's row
  // "Sammlungen", the collection pages, the detail pages' section and the search results.
  const invalidate = () =>
    Promise.all(
      [collectionsQuery.queryKey, ['home'], ['collection'], ['item'], ['search']].map((queryKey) =>
        queryClient.invalidateQueries({ queryKey }),
      ),
    );
  const remove = useMutation({
    mutationFn: (collection: AdminCollection) => deleteCollection(collection.id),
    onSuccess: async (_data, collection) => {
      setMessage(t('admin.collections.remove.done', { name: collection.name }));
      await invalidate();
      // The row and its buttons are gone: the focus goes to the page heading.
      header.current?.focus();
    },
  });

  const columns: DataColumn<AdminCollection>[] = [
    {
      key: 'name',
      header: t('admin.collections.columns.name'),
      rowHeader: true,
      cell: (collection) => (
        <span className={styles.stack}>
          <span>{collection.name}</span>
          {collection.nameDe && collection.nameDe !== collection.name && (
            <span className={styles.muted}>{collection.nameDe}</span>
          )}
        </span>
      ),
    },
    {
      key: 'source',
      header: t('admin.collections.columns.source'),
      cell: (collection) => (
        <Chip tone={collection.source === 'tmdb' ? 'info' : 'neutral'}>
          {collection.source === 'tmdb'
            ? t('admin.collections.source.tmdb')
            : t('admin.collections.source.manual')}
        </Chip>
      ),
    },
    {
      key: 'count',
      header: t('admin.collections.columns.count'),
      align: 'end',
      cell: (collection) => <span className={styles.numbers}>{collection.itemCount}</span>,
    },
    {
      key: 'actions',
      header: t('admin.collections.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (collection) => {
        // TMDB collections come from the provider: nothing to change here (`not_editable`).
        const locked = collection.source === 'tmdb';
        return (
          <span className={styles.rowActions}>
            <AriaButton
              variant="ghost"
              aria-label={t('admin.collections.members.label', { name: collection.name })}
              isDisabled={locked}
              onPress={() => setDialog({ kind: 'members', collection })}
            >
              {t('admin.collections.members.action')}
            </AriaButton>
            <AriaButton
              variant="ghost"
              aria-label={t('admin.collections.rename.label', { name: collection.name })}
              isDisabled={locked}
              onPress={() => setDialog({ kind: 'rename', collection })}
            >
              {t('admin.collections.rename.action')}
            </AriaButton>
            <ConfirmDialog
              trigger={
                <AriaButton
                  variant="ghost"
                  icon="trash"
                  aria-label={t('admin.collections.remove.label', { name: collection.name })}
                  isDisabled={locked}
                >
                  {t('admin.collections.remove.action')}
                </AriaButton>
              }
              title={t('admin.collections.remove.title', { name: collection.name })}
              body={t('admin.collections.remove.body')}
              confirmLabel={t('admin.collections.remove.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => remove.mutate(collection)}
            />
          </span>
        );
      },
    },
  ];

  const close = () => {
    setDialog(null);
    if (dialog?.kind === 'create') createButton.current?.focus();
  };

  return (
    <Page>
      <div ref={header} tabIndex={-1} className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.collections.title')}
        </Text>
        <Text tone="secondary">{t('admin.collections.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={collections.isPending}>
          <div>
            <AriaButton
              ref={createButton}
              variant="primary"
              icon="plus"
              onPress={() => setDialog({ kind: 'create' })}
            >
              {t('admin.collections.create.action')}
            </AriaButton>
          </div>
          {remove.error && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
          <div role="status" className={curation.live}>
            {message && !remove.error && <Notice live={false}>{message}</Notice>}
          </div>
          {collections.isPending && <Skeleton shape="block" />}
          {collections.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, collections.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void collections.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {collections.data?.length === 0 && <Notice>{t('admin.collections.empty')}</Notice>}
          {collections.data && collections.data.length > 0 && (
            <DataTable
              caption={t('admin.collections.list')}
              columns={columns}
              rows={collections.data}
              rowKey={(collection) => collection.id}
            />
          )}
        </div>
      )}
      {(dialog?.kind === 'create' || dialog?.kind === 'rename') && (
        <NameDialog
          collection={dialog.kind === 'rename' ? dialog.collection : undefined}
          onClose={close}
          onSaved={(saved) => {
            setMessage(
              dialog.kind === 'rename'
                ? t('admin.collections.rename.done', { name: saved.name })
                : t('admin.collections.create.done', { name: saved.name }),
            );
            close();
            void invalidate();
          }}
        />
      )}
      {dialog?.kind === 'members' && (
        <MembersDialog
          collection={dialog.collection}
          onClose={close}
          onSaved={() => {
            setMessage(t('admin.collections.members.done', { name: dialog.collection.name }));
            close();
            // The prefix ['admin', 'collections'] covers the stored members too.
            void invalidate();
          }}
        />
      )}
    </Page>
  );
}
