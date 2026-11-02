import type { AdminPick, PickUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useCallback, useMemo, useRef, useState } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Icon, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  StatusRegion,
  afterDialogsClosed,
  afterRender,
  focusLost,
  focusTitleIfLost,
  rankTo,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type TabItem,
} from '@/ui/aria';
import { fromLocalInput, toLocalInput } from '../CurationParts';
import { deletePick, picksQuery, reorderPicks, updatePick } from '../queries';
import { AddPickDialog } from './AddPickDialog';
import { phaseOf, pickColumns, type PickPhase } from './columns';
import { PickDetail, type PickDraft } from './PickDetail';
import styles from './Picks.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/picks');

const REORDER_KEY = ['admin', 'picks', 'order'];

/** The filter tabs (PIC-2): every pick, then by where it stands in time. */
const ALL = 'all';
const TAB_IDS: readonly string[] = [ALL, 'active', 'scheduled', 'expired'];

/** A save of the fold-out: the pick, what goes to the server and the fields as they were sent. */
interface Save {
  pick: AdminPick;
  body: PickUpdate;
  sent: PickDraft;
}

/** A choice of «Stelle»: the new order and the pick that moved. */
interface Move {
  order: AdminPick[];
  pick: AdminPick;
}

/** A question of the page (only «Entfernen» asks today). */
interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => void;
}

/** What the server holds of a pick, in the form of the fields. */
const draftOf = (pick: AdminPick): PickDraft => ({
  note: pick.note ?? '',
  from: toLocalInput(pick.startsAt),
  to: toLocalInput(pick.endsAt),
});

/**
 * Whether a draft says something else than the server: the note as it is saved (trimmed), the
 * window as the times it sends. A blank typed after the note is no change worth asking about.
 */
const differs = (draft: PickDraft, original: PickDraft) =>
  draft.note.trim() !== original.note ||
  fromLocalInput(draft.from) !== fromLocalInput(original.from) ||
  fromLocalInput(draft.to) !== fromLocalInput(original.to);

/**
 * Cat's recommendations (M7, E-2; concept 12, variant C): the picks in carousel order, filtered by
 * tabs «Alle», «Aktiv», «Geplant» and «Abgelaufen» (time only, rule 12). Each pick folds out into
 * its note and time window, saved in place, its place in the row as a choice (S-16) and
 * «Entfernen». «Hinzufügen» opens a dialog; `?add=<itemId>` (the detail page's link) opens it with
 * that title chosen.
 */
export function PicksPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.picks.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.picks.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <PicksContent />;
}

function PicksContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const { add } = route.useSearch();
  const navigate = route.useNavigate();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const addButton = useRef<HTMLButtonElement>(null);
  /** The button that opened «Hinzufügen» (the header's or the empty list's). */
  const addOpener = useRef<HTMLElement | null>(null);
  /** The fold-out of each open pick, to find its «Stelle» again after a refused move. */
  const details = useRef(new Map<string, HTMLElement>());

  // The drafts need what the server holds before the list query knows whether to pause for them:
  // a passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...picksQuery, enabled: false }).data;
  const originals = useMemo(
    () => new Map((cached ?? []).map((pick) => [String(pick.id), draftOf(pick)] as const)),
    [cached],
  );
  const originalOf = useCallback((id: string) => originals.get(id), [originals]);
  const drafts = useDrafts<PickDraft>(originalOf);
  const { set: setDraft, discard: discardDraft, valueOf: draftValue } = drafts;
  /** The pick has input that the server does not hold yet (S-17). */
  const isChanged = useCallback(
    (id: string) => {
      const original = originalOf(id);
      const draft = draftValue(id);
      return original !== undefined && draft !== undefined && differs(draft, original);
    },
    [originalOf, draftValue],
  );
  const changedIds = drafts.dirtyIds.filter(isChanged);
  const unsaved = useUnsavedChanges(changedIds.length > 0, { keep: ['add'] });

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const onExpandedChange = useCallback(
    (key: string, open: boolean) =>
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      }),
    [],
  );
  const [adding, setAdding] = useState(false);
  const addOpen = adding || Boolean(add);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  /** The button that asked: it gets the focus back when the question closes without a change. */
  const asker = useRef<HTMLElement | null>(null);
  // No refetch on focus while a dialog is open or an open fold-out has unsaved input; a draft
  // kept in a folded pick does not hold the list (it keeps its draft either way, useDrafts).
  const hold = confirmOpen || addOpen || changedIds.some((id) => expanded.has(id));
  const picks = useQuery({ ...picksQuery, refetchOnWindowFocus: !hold });

  // An unknown `?tab=` shows «Alle»; the address says so too.
  const [tab, selectTab] = useSearchSelection('tab', TAB_IDS, ALL, { normalise: true });

  // Every message gets its own number, so the same words twice are announced twice.
  const [message, setMessage] = useStatusMessage();
  const say = useCallback((text: string) => setMessage(text), [setMessage]);
  const [failure, setFailure] = useState<unknown>(null);
  // Saving and the server's last refusal belong to one pick each: two fold-outs save on their own.
  const [saving, setSaving] = useState<ReadonlySet<string>>(() => new Set());
  const [saveErrors, setSaveErrors] = useState<ReadonlyMap<string, unknown>>(() => new Map());
  const setSaveError = useCallback(
    (id: string, error: unknown) =>
      setSaveErrors((current) => {
        if (error === undefined && !current.has(id)) return current;
        const next = new Map(current);
        if (error === undefined) next.delete(id);
        else next.set(id, error);
        return next;
      }),
    [],
  );
  const failed = useCallback(
    (error: unknown) => {
      setMessage(null);
      setFailure(error);
    },
    [setMessage],
  );

  /**
   * The focus is gone from the fold-out of `id` (its row moved back after a refused move):
   * `selector` in that fold-out takes it, else the h1.
   */
  const focusDetailIfLost = useCallback((id: string, selector: string) => {
    if (!focusLost()) return;
    const target = details.current.get(id)?.querySelector<HTMLElement>(selector);
    if (target) target.focus();
    else focusTitleIfLost(titleRef);
  }, []);

  // The start page and the viewer's page of picks show them: asked again after every change.
  const invalidate = useCallback(
    () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: picksQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
        queryClient.invalidateQueries({ queryKey: ['picks'] }),
      ]),
    [queryClient],
  );

  // Quick choices queue their PUTs in order (scope); each shows its order at once, and only the
  // last one to settle asks the server again, so an earlier answer cannot undo a later move.
  const reorder = useMutation({
    mutationKey: REORDER_KEY,
    scope: { id: REORDER_KEY.join('/') },
    mutationFn: ({ order }: Move) => reorderPicks(order.map((pick) => pick.id)),
    onMutate: async ({ order }) => {
      setFailure(null);
      await queryClient.cancelQueries({ queryKey: picksQuery.queryKey });
      const previous = queryClient.getQueryData(picksQuery.queryKey);
      queryClient.setQueryData(picksQuery.queryKey, order);
      return { previous };
    },
    onError: async (error, { pick }, context) => {
      failed(error);
      // Later moves still wait in the queue: their orders build on this one, and the last one to
      // settle asks the server again, which puts right whatever this refusal left.
      if (!context?.previous || queryClient.isMutating({ mutationKey: REORDER_KEY }) > 1) return;
      queryClient.setQueryData(picksQuery.queryKey, context.previous);
      // Back in place, the row moved and its «Stelle» lost the focus: it takes it back.
      await afterRender();
      focusDetailIfLost(String(pick.id), 'select');
    },
    onSettled: () =>
      queryClient.isMutating({ mutationKey: REORDER_KEY }) > 1 ? undefined : invalidate(),
  });
  const update = useMutation({
    mutationFn: ({ pick, body }: Save) => updatePick(pick.id, body),
    onMutate: ({ pick }) => {
      const id = String(pick.id);
      setSaving((current) => new Set(current).add(id));
      setSaveError(id, undefined);
    },
    onSuccess: async (saved, { pick, sent }) => {
      const id = String(pick.id);
      setFailure(null);
      // The answer is what the server holds now; the list shows it before it is asked again.
      queryClient.setQueryData(picksQuery.queryKey, (current: AdminPick[] | undefined) =>
        current?.map((entry) => (entry.id === saved.id ? saved : entry)),
      );
      // The draft is spent when it still says what was sent (the server may hold the note
      // trimmed); what was typed while the save ran stays a draft.
      setDraft(id, (current) =>
        JSON.stringify(current) === JSON.stringify(sent) ? draftOf(saved) : current,
      );
      say(t('admin.picks.edit.done', { title: pick.title }));
      await invalidate();
    },
    onError: (error, { pick }) => setSaveError(String(pick.id), error),
    onSettled: (_data, _error, { pick }) =>
      setSaving((current) => {
        const next = new Set(current);
        next.delete(String(pick.id));
        return next;
      }),
  });
  const remove = useMutation({
    mutationFn: (pick: AdminPick) => deletePick(pick.id),
    onSuccess: async (_data, pick) => {
      setFailure(null);
      say(t('admin.picks.remove.done', { title: pick.title }));
      onExpandedChange(String(pick.id), false);
      await invalidate();
      // The row and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    onError: failed,
  });

  // The confirmation is the page's own: a row that goes while it is open (another tab, a
  // refetch) cannot take the dialog with it.
  const ask = (next: Confirmation) => {
    asker.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setConfirmation(next);
    setConfirmOpen(true);
  };
  const onConfirmOpenChange = (open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // Back to the button that asked; it may be gone by now (its row went), then the h1.
    void afterDialogsClosed().then(() => {
      const button = asker.current;
      asker.current = null;
      if (focusLost() && button?.isConnected) button.focus();
      else focusTitleIfLost(titleRef);
    });
  };
  const askRemove = (pick: AdminPick) =>
    ask({
      title: t('admin.picks.remove.title', { title: pick.title }),
      body: t('admin.picks.remove.body'),
      confirmLabel: t('admin.picks.remove.confirm'),
      onConfirm: () => remove.mutate(pick),
    });

  const list = useMemo(() => picks.data ?? [], [picks.data]);
  const place = useCallback((pick: AdminPick) => list.indexOf(pick), [list]);
  const { mutate: reorderMutate } = reorder;
  const onPlace = useCallback(
    (pick: AdminPick, position: number) => {
      const from = list.indexOf(pick);
      if (from < 0 || from === position) return;
      say(t('admin.picks.moved', { title: pick.title, position: position + 1 }));
      reorderMutate({ order: rankTo(list, from, position, 'move'), pick });
    },
    [list, say, t, reorderMutate],
  );
  const onChange = useCallback(
    (pick: AdminPick, next: Partial<PickDraft>) =>
      setDraft(String(pick.id), (current) => ({ ...current, ...next })),
    [setDraft],
  );
  const { mutate: updateMutate } = update;
  const onSave = useCallback(
    (pick: AdminPick) => {
      const id = String(pick.id);
      const draft = draftValue(id);
      // Nothing changed: nothing to send.
      if (!draft || !isChanged(id)) return;
      const note = draft.note.trim();
      updateMutate({
        pick,
        body: {
          note: note || null,
          startsAt: fromLocalInput(draft.from) ?? null,
          endsAt: fromLocalInput(draft.to) ?? null,
        },
        sent: draft,
      });
    },
    [draftValue, isChanged, updateMutate],
  );
  const onDiscard = useCallback(
    (pick: AdminPick) => {
      const id = String(pick.id);
      discardDraft(id);
      // The server's refusal was about the input that is gone now.
      setSaveError(id, undefined);
    },
    [discardDraft, setSaveError],
  );

  // Expired, scheduled or active as of the list's fetch (the list is asked again after every
  // change), so a row does not change its tab while it is being looked at.
  const now = picks.dataUpdatedAt;
  const columns = useMemo(
    () =>
      pickColumns(
        t,
        i18n.language,
        now,
        (pick) => place(pick) + 1,
        (pick) => isChanged(String(pick.id)),
      ),
    [t, i18n.language, now, place, isChanged],
  );
  const detail = (pick: AdminPick) => {
    const id = String(pick.id);
    const saveError = saveErrors.get(id);
    return (
      <div
        ref={(node) => {
          if (node) details.current.set(id, node);
          else details.current.delete(id);
        }}
      >
        <PickDetail
          pick={pick}
          draft={draftValue(id) ?? draftOf(pick)}
          changed={isChanged(id)}
          saving={saving.has(id)}
          errors={saveError === undefined ? null : formErrors(t, saveError)}
          place={place(pick)}
          count={list.length}
          onChange={onChange}
          onSave={onSave}
          onDiscard={onDiscard}
          onPlace={onPlace}
          onRemove={askRemove}
        />
      </div>
    );
  };

  const phases = useMemo(() => {
    const byPhase: Record<PickPhase, AdminPick[]> = { active: [], scheduled: [], expired: [] };
    for (const pick of list) byPhase[phaseOf(pick, now)].push(pick);
    return byPhase;
  }, [list, now]);
  const tabOf = (id: string, label: string, rows: AdminPick[]): TabItem => ({
    id,
    label,
    count: number.format(rows.length),
    countLabel: t('admin.picks.tabs.count', {
      count: rows.length,
      number: number.format(rows.length),
    }),
    content:
      rows.length === 0 ? (
        <Text tone="secondary">{t('admin.picks.emptyTab')}</Text>
      ) : (
        <DataTable
          caption={t('admin.picks.list')}
          columns={columns}
          rows={rows}
          rowKey={(pick) => String(pick.id)}
          expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
        />
      ),
  });

  const entries = t('admin.picks.count.entries', {
    count: list.length,
    number: number.format(list.length),
  });
  const planned = phases.scheduled.length;
  const counts =
    planned > 0
      ? t('admin.picks.count.both', {
          entries,
          planned: t('admin.picks.count.planned', {
            count: planned,
            number: number.format(planned),
          }),
        })
      : entries;

  const closeAdd = () => {
    setAdding(false);
    // The link's title is used once: a reload must not open the dialog again.
    if (add) {
      void navigate({
        search: (previous) => ({ ...previous, add: undefined }),
        replace: true,
      });
    }
    // Back to the button that opened it; the header's after a link's `?add=` or when the empty
    // list's button went with the new pick.
    const opener = addOpener.current;
    addOpener.current = null;
    (opener?.isConnected ? opener : addButton.current)?.focus();
  };
  const openAdd = ({ target }: { target: Element }) => {
    addOpener.current = target instanceof HTMLElement ? target : null;
    setAdding(true);
  };

  return (
    <AdminPage
      title={t('admin.picks.title')}
      help={t('admin.picks.intro')}
      titleRef={titleRef}
      actions={
        <AriaButton ref={addButton} variant="primary" icon="plus" onPress={openAdd}>
          {t('admin.picks.add.action')}
        </AriaButton>
      }
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {picks.isPending && <Skeleton shape="block" />}
      {picks.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, picks.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void picks.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {picks.data?.length === 0 && (
        <Notice
          actions={
            <AriaButton variant="secondary" icon="plus" onPress={openAdd}>
              {t('admin.picks.add.action')}
            </AriaButton>
          }
        >
          {t('admin.picks.empty')}
        </Notice>
      )}
      {picks.data && picks.data.length > 0 && (
        <BandPanel
          title={t('admin.picks.list')}
          icon="star"
          meta={counts}
          tabs={{
            label: t('admin.picks.tabs.label'),
            selectedKey: tab,
            onSelectionChange: selectTab,
            tabs: [
              tabOf(ALL, t('admin.picks.tabs.all'), list),
              tabOf('active', t('admin.picks.tabs.active'), phases.active),
              tabOf('scheduled', t('admin.picks.tabs.scheduled'), phases.scheduled),
              tabOf('expired', t('admin.picks.tabs.expired'), phases.expired),
            ],
          }}
        />
      )}
      <p className={styles.hint}>
        <Icon name="info" size="sm" />
        <span>
          {/* Not <link>: the Trans parser takes it for the void HTML element. */}
          <Trans
            i18nKey="admin.picks.homeRowsHint"
            components={{ page: <TextLink href={paths.adminHomeRows}>{null}</TextLink> }}
          />
        </span>
      </p>
      {confirmation && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={confirmation.title}
          body={confirmation.body}
          confirmLabel={confirmation.confirmLabel}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={confirmation.onConfirm}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
      {addOpen && (
        <AddPickDialog
          presetId={adding ? undefined : add}
          onClose={closeAdd}
          onAdded={(title) => {
            setFailure(null);
            say(t('admin.picks.add.done', { title }));
            void invalidate();
            closeAdd();
          }}
        />
      )}
    </AdminPage>
  );
}
