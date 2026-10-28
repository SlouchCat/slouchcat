import type { Announcement, AnnouncementRequest, AnnouncementUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formText } from '@/lib/form';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Page, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  CheckboxGroup,
  ConfirmDialog,
  DataTable,
  Form,
  FormDialog,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import { adminAnnouncementsQuery } from './queries';
import data from './AdminData.module.css';
import styles from './AdminTablePage.module.css';

/** Longest title and text the server takes (AnnouncementRequest). */
const MAX_TITLE = 120;
const MAX_BODY = 4000;

/**
 * «Neue Ankündigung» or the change of a draft (`announcement`). A new one may go out at once;
 * a draft is published from its row.
 */
function AnnouncementDialog({
  announcement,
  onClose,
}: {
  announcement: Announcement | null;
  onClose: (saved: Announcement | null) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [publish, setPublish] = useState<string[]>([]);
  const publishNow = publish.includes('publish');
  const save = useMutation({
    mutationFn: (body: AnnouncementRequest | AnnouncementUpdate): Promise<Announcement> =>
      announcement
        ? request(
            api.PUT('/admin/announcements/{id}', {
              params: { path: { id: announcement.id } },
              body,
            }),
          )
        : request(api.POST('/admin/announcements', { body: { publish: false, ...body } })),
    onSuccess: async (saved) => {
      await queryClient.invalidateQueries({ queryKey: adminAnnouncementsQuery.queryKey });
      onClose(saved);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const body = { title: formText(form, 'title'), body: formText(form, 'body') };
    save.mutate(announcement ? body : { ...body, publish: publishNow });
  };

  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose(null);
      }}
      title={
        announcement ? t('admin.announcements.form.editTitle') : t('admin.announcements.form.title')
      }
    >
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.form}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <TextField
            name="title"
            label={t('admin.announcements.form.titleLabel')}
            defaultValue={announcement?.title}
            maxLength={MAX_TITLE}
            isRequired
            validate={required}
            autoComplete="off"
          />
          <TextField
            name="body"
            label={t('admin.announcements.form.body')}
            description={t('admin.announcements.form.bodyHint', { max: MAX_BODY })}
            defaultValue={announcement?.body}
            rows={8}
            maxLength={MAX_BODY}
            isRequired
            validate={required}
          />
          {!announcement && (
            <CheckboxGroup
              name="publish"
              label={t('admin.announcements.form.publishLabel')}
              description={t('admin.announcements.publish.reach')}
              value={publish}
              onChange={setPublish}
              options={[{ value: 'publish', label: t('admin.announcements.form.publishNow') }]}
            />
          )}
          <div className={styles.dialogActions}>
            <AriaButton variant="ghost" onPress={() => onClose(null)}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isPending={save.isPending}>
              {save.isPending
                ? t('common.saving')
                : publishNow
                  ? t('admin.announcements.publish.confirm')
                  : t('admin.announcements.form.save')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * Announcements (A-7): news of the main user. Drafts can be changed; publishing shows one to every
 * profile at its next start and sends it on the channels the accounts chose. The counter says how
 * many profiles acknowledged it.
 */
export function AnnouncementsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.announcements.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const announcements = useQuery({ ...adminAnnouncementsQuery, enabled: allowed });
  // `null`: a new one; an announcement: change that draft.
  const [editing, setEditing] = useState<{ announcement: Announcement | null } | null>(null);
  const [done, setDone] = useState<string | null>(null);
  // A deleted row goes away with its buttons: the focus moves to the page heading.
  const header = useRef<HTMLDivElement>(null);
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: adminAnnouncementsQuery.queryKey });
  const publish = useMutation({
    mutationFn: (announcement: Announcement): Promise<Announcement> =>
      request(
        api.POST('/admin/announcements/{id}/publish', {
          params: { path: { id: announcement.id } },
        }),
      ),
    // "Bearbeiten" and "Veröffentlichen" go away with the draft state: focus to the heading.
    onSuccess: async (published) => {
      setDone(t('admin.announcements.published', { title: published.title }));
      await refresh();
      header.current?.focus();
    },
  });
  const remove = useMutation({
    mutationFn: (announcement: Announcement) =>
      request(
        api.DELETE('/admin/announcements/{id}', { params: { path: { id: announcement.id } } }),
      ),
    onSuccess: async (_data, announcement) => {
      setDone(t('admin.announcements.deleted', { title: announcement.title }));
      await refresh();
      header.current?.focus();
    },
  });
  const failure = publish.error ?? remove.error;
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setDone(null);
    publish.reset();
    remove.reset();
  };

  const columns: DataColumn<Announcement>[] = [
    {
      key: 'title',
      header: t('admin.announcements.columns.title'),
      rowHeader: true,
      cell: (announcement) => announcement.title,
    },
    {
      key: 'created',
      header: t('admin.announcements.columns.created'),
      cell: (announcement) => (
        <time className={styles.nowrap} dateTime={announcement.createdAt}>
          {formatDateTime(announcement.createdAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'published',
      header: t('admin.announcements.columns.published'),
      cell: (announcement) =>
        announcement.publishedAt ? (
          <time className={styles.nowrap} dateTime={announcement.publishedAt}>
            {formatDateTime(announcement.publishedAt, i18n.language)}
          </time>
        ) : (
          <Chip tone="outline">{t('admin.announcements.draft')}</Chip>
        ),
    },
    {
      key: 'acknowledged',
      header: t('admin.announcements.columns.acknowledged'),
      cell: (announcement) =>
        announcement.publishedAt ? (
          <span className={styles.numbers}>
            {t('admin.announcements.acknowledged', {
              acknowledged: announcement.acknowledgedCount,
              total: announcement.profileCount,
            })}
          </span>
        ) : (
          <span className={styles.muted}>{t('admin.announcements.notSent')}</span>
        ),
    },
    {
      key: 'actions',
      header: t('admin.announcements.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (announcement) => (
        <span className={styles.rowActions}>
          {!announcement.publishedAt && (
            <>
              <AriaButton
                variant="ghost"
                aria-label={t('admin.announcements.edit.label', { title: announcement.title })}
                onPress={() => {
                  start();
                  setEditing({ announcement });
                }}
              >
                {t('admin.announcements.edit.action')}
              </AriaButton>
              <ConfirmDialog
                trigger={
                  <AriaButton
                    variant="ghost"
                    aria-label={t('admin.announcements.publish.label', {
                      title: announcement.title,
                    })}
                  >
                    {t('admin.announcements.publish.action')}
                  </AriaButton>
                }
                title={t('admin.announcements.publish.title', { title: announcement.title })}
                body={t('admin.announcements.publish.body')}
                confirmLabel={t('admin.announcements.publish.confirm')}
                cancelLabel={t('common.cancel')}
                onConfirm={() => {
                  start();
                  publish.mutate(announcement);
                }}
              />
            </>
          )}
          <ConfirmDialog
            trigger={
              <AriaButton
                variant="ghost"
                aria-label={t('admin.announcements.delete.label', { title: announcement.title })}
              >
                {t('admin.announcements.delete.action')}
              </AriaButton>
            }
            title={t('admin.announcements.delete.title', { title: announcement.title })}
            body={t('admin.announcements.delete.body')}
            confirmLabel={t('admin.announcements.delete.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            onConfirm={() => {
              start();
              remove.mutate(announcement);
            }}
          />
        </span>
      ),
    },
  ];

  return (
    <Page>
      <div ref={header} tabIndex={-1} className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.announcements.title')}
        </Text>
        <Text tone="secondary">{t('admin.announcements.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={announcements.isPending}>
          <div className={data.toolbar}>
            <AriaButton
              variant="primary"
              icon="plus"
              onPress={() => {
                start();
                setEditing({ announcement: null });
              }}
            >
              {t('admin.announcements.create')}
            </AriaButton>
          </div>
          {failure && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
          {done && !failure && <Notice>{done}</Notice>}
          {announcements.isPending && <Skeleton shape="block" />}
          {announcements.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, announcements.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void announcements.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {announcements.data?.length === 0 && <Notice>{t('admin.announcements.empty')}</Notice>}
          {announcements.data && announcements.data.length > 0 && (
            <DataTable
              caption={t('admin.announcements.list')}
              columns={columns}
              rows={announcements.data}
              rowKey={(announcement) => String(announcement.id)}
            />
          )}
        </div>
      )}
      {editing && (
        <AnnouncementDialog
          announcement={editing.announcement}
          onClose={(saved) => {
            if (saved) {
              setDone(
                saved.publishedAt
                  ? t('admin.announcements.published', { title: saved.title })
                  : t('admin.announcements.saved', { title: saved.title }),
              );
            }
            setEditing(null);
          }}
        />
      )}
    </Page>
  );
}
