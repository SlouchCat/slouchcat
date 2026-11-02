import type { Announcement, AnnouncementRequest } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useCallback, useLayoutEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDate } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  ConfirmDialog,
  MasterDetail,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  useDrafts,
  useSearchSelection,
  useUnsavedChanges,
  type MasterDetailItem,
} from '@/ui/aria';
import {
  DraftDetail,
  NewDetail,
  PublishedDetail,
  type DraftText,
  type NewAnnouncement,
} from './announcements/AnnouncementDetail';
import { adminAnnouncementsQuery } from './queries';
import styles from './announcements/Announcements.module.css';

/** The item of the new announcement while it is not saved: a stable id of its own (`?item=new`). */
const NEW = 'new';
/**
 * The item of a saved announcement (`?item=announcement-12`). Not the bare id: the router writes a
 * text that reads as a number in quotes (`?item=%2212%22`).
 */
const itemOf = (announcement: Announcement) => `announcement-${announcement.id}`;

const EMPTY_NEW: NewAnnouncement = { title: '', body: '', publish: false };

/** What the server keeps of a text: without the blanks around title and text. */
const trimmed = (text: DraftText): DraftText => ({
  title: text.title.trim(),
  body: text.body.trim(),
});
const sameText = (a: DraftText, b: DraftText) =>
  JSON.stringify(trimmed(a)) === JSON.stringify(trimmed(b));

/** The server refused a change because the announcement was published meanwhile. */
const publishedMeanwhile = (error: unknown) =>
  isApiError(error, 409) && error.code === 'announcement_published';

/** The save of a changed draft before its publication failed: the form shows why. */
class SaveFailed extends Error {
  constructor(readonly error: unknown) {
    super('save before publishing failed');
  }
}

/** The one question of the page (publish, delete, discard). */
interface Question {
  title: string;
  body: string;
  confirmLabel: string;
  tone: 'primary' | 'danger';
  defaultFocus?: 'confirm' | 'cancel';
  onConfirm: () => void;
}

/**
 * Announcements (A-7; concept 12, variant B): news of the main user. The list on the left holds
 * the drafts and the published ones; the right side shows the selected one with its dates, how
 * many profiles read it, and «So sehen es die Profile», the content of the profiles' dialog with
 * what is typed right now. Drafts are written and changed in place (ANK-1 to ANK-3); what is typed
 * stays per announcement while another one is selected (useDrafts, S-17).
 */
export function AnnouncementsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.announcements.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.announcements.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <AnnouncementsContent />;
}

function AnnouncementsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const newTitleField = useRef<HTMLInputElement>(null);
  const draftTitleField = useRef<HTMLInputElement>(null);
  const draftSaveButton = useRef<HTMLButtonElement>(null);

  // No refetch while a question is open or something is typed: the list stays as it was.
  const hold = useRef(false);
  const announcements = useQuery({
    ...adminAnnouncementsQuery,
    refetchOnWindowFocus: () => !hold.current,
  });
  const list = announcements.data;

  const originalOf = useCallback(
    (id: string): DraftText | undefined => {
      const saved = list?.find((entry) => String(entry.id) === id && !entry.publishedAt);
      return saved ? { title: saved.title, body: saved.body } : undefined;
    },
    [list],
  );
  // Drafts of saved announcements, by id; a published one (here or elsewhere) has none. A draft
  // the server comes to hold, or whose announcement is published or deleted, is dropped by
  // useDrafts itself; what is typed while a save waits stays a draft.
  const drafts = useDrafts<DraftText>(originalOf);
  /** A draft counts as a change only if it differs in more than blanks around the text. */
  const isChanged = (id: string) => {
    const draft = drafts.valueOf(id);
    const original = originalOf(id);
    return (
      drafts.isDirty(id) &&
      draft !== undefined &&
      original !== undefined &&
      !sameText(draft, original)
    );
  };
  const changed = drafts.dirtyIds.some(isChanged);
  // The new one is not on the server yet, so it keeps its own state.
  const [created, setCreated] = useState<NewAnnouncement | null>(null);
  const newDirty = created !== null && (created.title.trim() !== '' || created.body.trim() !== '');
  const unsaved = useUnsavedChanges(changed || newDirty);

  const [question, setQuestion] = useState<Question | null>(null);
  const [asking, setAsking] = useState(false);
  // Where the focus goes once a confirmed question has closed (else the h1, if it was lost).
  const afterQuestion = useRef<(() => void) | null>(null);

  // The state of the last render, for the answers of the server that arrive later: what was typed
  // meanwhile must not be lost.
  const latestDrafts = useRef(drafts);
  const latestCreated = useRef(created);
  useLayoutEffect(() => {
    hold.current = asking || unsaved.pausePolling;
    latestDrafts.current = drafts;
    latestCreated.current = created;
  });

  // Drafts first, then the published ones, each in the server's order (newest first).
  const ordered = [
    ...(list ?? []).filter((entry) => !entry.publishedAt),
    ...(list ?? []).filter((entry) => entry.publishedAt),
  ];
  const first = ordered[0];
  // Without `?item=` the first saved one shows; the new one always has `?item=new`.
  const fallback = first ? itemOf(first) : created ? NEW : '';
  const ids = [...(created ? [NEW] : []), ...ordered.map(itemOf)];
  // An unknown `?item=` (deleted, or a new one that was never saved) shows the first one, and the
  // address drops it too (replaced, not pushed).
  const [selected] = useSearchSelection('item', ids, fallback, {
    normalise: announcements.isSuccess,
  });
  /**
   * Selects `id` in the address, replaced, not pushed; the first saved one without `?item=`. A
   * promise, unlike useSearchSelection's: a save waits for it before the new one leaves the list.
   */
  const select = (id: string) =>
    navigate({
      to: '.',
      replace: true,
      search: ((previous: Record<string, unknown>) => ({
        ...previous,
        item: id === fallback && id !== NEW ? undefined : id,
      })) as never,
    });

  // The outcome of the last action. `seq` counts every message, so the same text twice is
  // announced twice: the notice is a new element each time.
  const [message, setMessage] = useState<{ text: string; seq: number } | null>(null);
  const say = (text: string) => setMessage((last) => ({ text, seq: (last?.seq ?? 0) + 1 }));
  // A refusal of the page (publish, delete, a draft published meanwhile); a form's own errors
  // stay with the form.
  const [failure, setFailure] = useState<unknown>(null);

  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: adminAnnouncementsQuery.queryKey });
  const put = (id: number, text: DraftText): Promise<Announcement> =>
    request(api.PUT('/admin/announcements/{id}', { params: { path: { id } }, body: text }));
  /** Another administrator published it meanwhile: say so, drop the draft, show it as sent. */
  const onPublishedMeanwhile = (announcement: Announcement, error: unknown) => {
    setFailure(error);
    latestDrafts.current.discard(String(announcement.id));
    void refresh();
  };

  const create = useMutation({
    mutationFn: (body: AnnouncementRequest): Promise<Announcement> =>
      request(api.POST('/admin/announcements', { body })),
    onSuccess: async (saved, sent) => {
      await refresh();
      // What was typed while the request ran goes on as the draft of the saved one.
      const typed = latestCreated.current;
      // The saved one takes the new one's place in the list and in the address.
      await select(itemOf(saved));
      setCreated(null);
      if (!saved.publishedAt && typed && !sameText(typed, sent))
        latestDrafts.current.set(String(saved.id), { title: typed.title, body: typed.body });
      say(
        saved.publishedAt
          ? t('admin.announcements.published', { title: saved.title })
          : t('admin.announcements.saved', { title: saved.title }),
      );
      if (saved.publishedAt) {
        await afterDialogsClosed();
        focusTitleIfLost(titleRef);
      } else {
        // The form went with the new one; the same place in the saved draft takes the focus.
        await afterRender();
        draftSaveButton.current?.focus();
      }
    },
  });
  const update = useMutation({
    mutationFn: ({ announcement, text }: { announcement: Announcement; text: DraftText }) =>
      put(announcement.id, text),
    onSuccess: async (saved) => {
      await refresh();
      // A draft that differs from the saved text only in blanks is spent.
      const id = String(saved.id);
      const now = latestDrafts.current.valueOf(id);
      if (now && sameText(now, saved)) latestDrafts.current.discard(id);
      say(t('admin.announcements.saved', { title: saved.title }));
    },
    onError: (error, { announcement }) => {
      if (publishedMeanwhile(error)) onPublishedMeanwhile(announcement, error);
    },
  });
  const publish = useMutation({
    // What is typed is what goes out: a changed draft is saved first.
    mutationFn: async (announcement: Announcement): Promise<Announcement> => {
      const id = String(announcement.id);
      const draft = latestDrafts.current.valueOf(id);
      if (isChanged(id) && draft) {
        try {
          await put(announcement.id, trimmed(draft));
        } catch (error) {
          throw new SaveFailed(error);
        }
      }
      return request(
        api.POST('/admin/announcements/{id}/publish', {
          params: { path: { id: announcement.id } },
        }),
      );
    },
    onSuccess: async (published) => {
      await refresh();
      say(t('admin.announcements.published', { title: published.title }));
      // Its form and buttons are gone: once React rendered that, the h1 takes the focus.
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    onError: (error, announcement) => {
      if (error instanceof SaveFailed) {
        if (publishedMeanwhile(error.error)) onPublishedMeanwhile(announcement, error.error);
        return;
      }
      setFailure(error);
      // A draft saved before a refused publication is saved all the same.
      void refresh();
    },
  });
  const remove = useMutation({
    mutationFn: (announcement: Announcement) =>
      request(
        api.DELETE('/admin/announcements/{id}', { params: { path: { id: announcement.id } } }),
      ),
    onSuccess: async (_data, announcement) => {
      await refresh();
      say(t('admin.announcements.deleted', { title: announcement.title }));
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    onError: (error) => setFailure(error),
  });

  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setMessage(null);
    setFailure(null);
    create.reset();
    update.reset();
    publish.reset();
    remove.reset();
  };

  // The question is the page's own: a detail that goes while it is open cannot take it along.
  const ask = (next: Question) => {
    afterQuestion.current = null;
    setQuestion(next);
    setAsking(true);
  };
  const onAskingChange = useCallback((open: boolean) => {
    if (open) return;
    setAsking(false);
    const after = afterQuestion.current;
    afterQuestion.current = null;
    // The button that asked may be gone by now.
    void afterDialogsClosed().then(after ?? (() => focusTitleIfLost(titleRef)));
  }, []);

  const openNew = () => {
    start();
    setCreated((current) => current ?? EMPTY_NEW);
    void select(NEW);
    void afterRender().then(() => newTitleField.current?.focus());
  };
  const dropNew = () => {
    setCreated(null);
    create.reset();
  };

  const askDelete = (announcement: Announcement) =>
    ask({
      title: t('admin.announcements.delete.title', { title: announcement.title }),
      body: t('admin.announcements.delete.body'),
      confirmLabel: t('admin.announcements.delete.confirm'),
      tone: 'danger',
      onConfirm: () => {
        start();
        remove.mutate(announcement);
      },
    });

  const draftCount = (created ? 1 : 0) + ordered.filter((entry) => !entry.publishedAt).length;
  const section = {
    drafts: t('admin.announcements.sections.drafts', { number: draftCount }),
    published: t('admin.announcements.sections.published', {
      number: ordered.filter((entry) => entry.publishedAt).length,
    }),
  };
  const items: MasterDetailItem[] = [
    ...(created
      ? [
          {
            id: NEW,
            label: created.title.trim() || t('admin.announcements.form.title'),
            description: t('admin.announcements.item.new'),
            section: section.drafts,
          },
        ]
      : []),
    ...ordered.map((entry) => ({
      id: itemOf(entry),
      label: entry.title,
      description: entry.publishedAt
        ? t('admin.announcements.item.read', {
            acknowledged: entry.acknowledgedCount,
            total: entry.profileCount,
          })
        : isChanged(String(entry.id))
          ? t('admin.announcements.item.changed')
          : t('admin.announcements.item.created', {
              date: formatDate(entry.createdAt, i18n.language),
            }),
      section: entry.publishedAt ? section.published : section.drafts,
    })),
  ];

  let detail: ReactNode = null;
  const current = ordered.find((entry) => itemOf(entry) === selected);
  if (selected === NEW && created) {
    detail = (
      <NewDetail
        key={NEW}
        value={created}
        onChange={setCreated}
        saving={create.isPending}
        error={create.error}
        titleRef={newTitleField}
        onSave={() => {
          setMessage(null);
          setFailure(null);
          create.mutate({ ...trimmed(created), publish: created.publish });
        }}
        onCancel={() => {
          if (!newDirty) dropNew();
          else
            ask({
              title: t('admin.announcements.discard.newTitle'),
              body: t('admin.announcements.discard.newBody'),
              confirmLabel: t('admin.announcements.discard.confirm'),
              tone: 'danger',
              defaultFocus: 'cancel',
              onConfirm: dropNew,
            });
        }}
      />
    );
  } else if (current?.publishedAt) {
    detail = (
      <PublishedDetail
        key={current.id}
        announcement={current}
        onDelete={() => askDelete(current)}
      />
    );
  } else if (current) {
    const id = String(current.id);
    const text = drafts.valueOf(id) ?? current;
    const mine = (announcement: Announcement | undefined) => announcement?.id === current.id;
    // Its own failed save, or the failed save before its publication; «published meanwhile» is
    // the page's message instead.
    const saveError =
      mine(update.variables?.announcement) && update.error && !publishedMeanwhile(update.error)
        ? update.error
        : mine(publish.variables) &&
            publish.error instanceof SaveFailed &&
            !publishedMeanwhile(publish.error.error)
          ? publish.error.error
          : null;
    detail = (
      <DraftDetail
        key={current.id}
        announcement={current}
        text={text}
        dirty={isChanged(id)}
        onChange={(next) => drafts.set(id, next)}
        saving={update.isPending && mine(update.variables?.announcement)}
        publishing={publish.isPending && mine(publish.variables)}
        error={saveError}
        titleRef={draftTitleField}
        saveRef={draftSaveButton}
        onSave={() => {
          // Nothing changed: nothing to send, the focus stays.
          if (!isChanged(id)) return;
          start();
          update.mutate({ announcement: current, text: trimmed(text) });
        }}
        onDiscard={() =>
          ask({
            title: t('admin.announcements.discard.title'),
            body: t('admin.announcements.discard.body', { title: current.title }),
            confirmLabel: t('admin.announcements.discard.confirm'),
            tone: 'danger',
            defaultFocus: 'cancel',
            onConfirm: () => {
              drafts.discard(id);
              // «Verwerfen» goes with the change: the title takes the focus.
              afterQuestion.current = () => draftTitleField.current?.focus();
            },
          })
        }
        onPublish={() =>
          ask({
            title: t('admin.announcements.publish.title', { title: text.title.trim() }),
            body: t('admin.announcements.publish.body'),
            confirmLabel: t('admin.announcements.publish.confirm'),
            tone: 'primary',
            onConfirm: () => {
              start();
              publish.mutate(current);
            },
          })
        }
        onDelete={() => askDelete(current)}
      />
    );
  }

  return (
    <AdminPage
      title={t('admin.announcements.title')}
      help={t('admin.announcements.intro')}
      titleRef={titleRef}
      actions={
        <AriaButton variant="primary" icon="plus" isDisabled={!list} onPress={openNew}>
          {t('admin.announcements.create')}
        </AriaButton>
      }
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <div role="status" className={styles.live}>
        {message && failure === null && (
          <Notice key={message.seq} live={false}>
            {message.text}
          </Notice>
        )}
      </div>
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
      {list && items.length === 0 && <Text tone="secondary">{t('admin.announcements.empty')}</Text>}
      {list && items.length > 0 && (
        <MasterDetail
          label={t('admin.announcements.list')}
          items={items}
          selectedId={selected}
          onSelect={(id) => void select(id)}
        >
          {detail}
        </MasterDetail>
      )}
      {question && (
        <ConfirmDialog
          isOpen={asking}
          onOpenChange={onAskingChange}
          title={question.title}
          body={question.body}
          confirmLabel={question.confirmLabel}
          cancelLabel={t('common.cancel')}
          tone={question.tone}
          defaultFocus={question.defaultFocus}
          onConfirm={question.onConfirm}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
