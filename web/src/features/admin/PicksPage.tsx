import type { AdminPick, PickRequest, PickUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useId, useRef, useState, type FormEvent } from 'react';
import { Trans, useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { manageProfilesQuery } from '@/features/profiles/queries';
import { formText } from '@/lib/form';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Icon,
  Notice,
  Page,
  Panel,
  Skeleton,
  StatusDot,
  Text,
  TextLink,
  type StatusDotState,
} from '@/ui';
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
import {
  fromLocalInput,
  moveBy,
  titleMeta,
  TitleSearch,
  toLocalInput,
  useMoveButtons,
} from './CurationParts';
import { PosterTitle } from './PosterTitle';
import {
  adminItemQuery,
  createPicks,
  deletePick,
  picksQuery,
  reorderPicks,
  updatePick,
} from './queries';
import styles from './AdminTablePage.module.css';
import curation from './Curation.module.css';
import layout from './PicksPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/picks');

const REORDER_KEY = ['admin', 'picks', 'order'];

/** A press of "Nach oben"/"Nach unten": the new order and the button that asked for it. */
interface Move {
  order: AdminPick[];
  pick: AdminPick;
  delta: -1 | 1;
}

/** Longest note the server takes (`PickRequest.note`). */
const MAX_NOTE = 280;
/** The value of the target group that means "every profile" (a global pick). */
const ALL = 'all';

/** A title chosen for a new pick: from the search, or the one `?add=` names. */
interface Choice {
  id: string;
  title: string;
  kind: AdminPick['kind'];
  year?: number;
}

/** Note and time window of a pick, shared by the add and the edit dialog. */
function WindowFields({
  note,
  from,
  to,
  onFrom,
  onTo,
}: {
  note?: string;
  from: string;
  to: string;
  onFrom: (value: string) => void;
  onTo: (value: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <>
      <TextField
        name="note"
        label={t('admin.picks.form.note')}
        description={t('admin.picks.form.noteHint', { max: MAX_NOTE })}
        defaultValue={note}
        maxLength={MAX_NOTE}
        autoComplete="off"
      />
      <TextField
        name="startsAt"
        type="datetime-local"
        label={t('admin.picks.form.from')}
        description={t('admin.picks.form.windowHint')}
        value={from}
        onChange={onFrom}
      />
      <TextField
        name="endsAt"
        type="datetime-local"
        label={t('admin.picks.form.to')}
        value={to}
        onChange={onTo}
        validate={(value) =>
          value && from && value <= from ? t('admin.picks.form.endBeforeStart') : null
        }
      />
    </>
  );
}

/**
 * "Hinzufügen": a title from the search (or the one of `?add=`), for every profile or for single
 * ones, with an optional note and time window.
 */
function AddPickDialog({
  presetId,
  onClose,
  onAdded,
}: {
  presetId?: string;
  onClose: () => void;
  onAdded: (title: string) => void;
}) {
  const { t } = useTranslation();
  const profiles = useQuery(manageProfilesQuery);
  // The title the link named, from the admin view: no profile's visibility hides it here.
  const preset = useQuery({ ...adminItemQuery(presetId ?? ''), enabled: Boolean(presetId) });
  const [picked, setPicked] = useState<Choice | null>(null);
  const chosen: Choice | null =
    picked ??
    (preset.data
      ? {
          id: preset.data.id,
          title: preset.data.title,
          kind: preset.data.kind,
          year: preset.data.year,
        }
      : null);
  const [missingTitle, setMissingTitle] = useState(false);
  const [targets, setTargets] = useState<string[]>([ALL]);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const create = useMutation({
    mutationFn: (body: PickRequest) => createPicks(body),
    onSuccess: () => onAdded(chosen?.title ?? ''),
  });
  const errors = create.error ? formErrors(t, create.error) : null;

  // "Alle Profile" and single profiles exclude each other: the last choice wins.
  const onTargets = (next: string[]) => {
    if (next.includes(ALL) && !targets.includes(ALL)) setTargets([ALL]);
    else setTargets(next.length > 1 ? next.filter((value) => value !== ALL) : next);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!chosen) {
      setMissingTitle(true);
      return;
    }
    const data = new FormData(event.currentTarget);
    const note = formText(data, 'note');
    create.mutate({
      itemId: chosen.id,
      ...(targets.includes(ALL) ? {} : { profileIds: targets.map(Number) }),
      ...(note ? { note } : {}),
      ...(fromLocalInput(from) ? { startsAt: fromLocalInput(from) } : {}),
      ...(fromLocalInput(to) ? { endsAt: fromLocalInput(to) } : {}),
    });
  };
  const itemError = errors?.fields.itemId;

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.picks.add.title')}
      description={t('admin.picks.add.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TitleSearch
          label={t('admin.picks.search.label')}
          hint={t('admin.picks.search.hint')}
          emptyText={t('admin.picks.search.empty')}
          resultsLabel={t('admin.picks.search.results')}
          actionText={t('admin.picks.search.select')}
          actionLabel={(hit) => t('admin.picks.search.selectLabel', { title: hit.title })}
          onPick={(hit) => {
            setPicked(hit);
            setMissingTitle(false);
            if (create.error) create.reset();
          }}
          exclude={chosen ? new Set([chosen.id]) : undefined}
        />
        <div aria-live="polite">
          {chosen ? (
            <p className={curation.chosen}>
              {t('admin.picks.search.chosen', { title: chosen.title })}
              <span className={curation.muted}> · {titleMeta(t, chosen)}</span>
            </p>
          ) : presetId && preset.isPending ? (
            <p className={curation.muted}>{t('common.loading')}</p>
          ) : (
            <p className={curation.muted}>{t('admin.picks.search.none')}</p>
          )}
          {presetId && preset.isError && !picked && (
            <Notice tone="warning">{t('admin.picks.add.presetMissing')}</Notice>
          )}
          {missingTitle && !chosen && (
            <Notice tone="danger">{t('admin.picks.add.titleRequired')}</Notice>
          )}
          {itemError && <Notice tone="danger">{itemError}</Notice>}
        </div>
        <CheckboxGroup
          name="profileIds"
          label={t('admin.picks.targets.label')}
          description={t('admin.picks.targets.description')}
          value={targets}
          onChange={onTargets}
          validate={(value) => (value.length === 0 ? t('admin.picks.targets.required') : null)}
          options={[
            { value: ALL, label: t('admin.picks.targets.all') },
            ...(profiles.data ?? []).map((profile) => ({
              value: String(profile.id),
              label: profile.name,
            })),
          ]}
        />
        <WindowFields from={from} to={to} onFrom={setFrom} onTo={setTo} />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending ? t('admin.picks.add.submitting') : t('admin.picks.add.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

/** "Bearbeiten": note and time window of a pick; an emptied field clears the value. */
function EditPickDialog({
  pick,
  onClose,
  onSaved,
}: {
  pick: AdminPick;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useTranslation();
  const [from, setFrom] = useState(toLocalInput(pick.startsAt));
  const [to, setTo] = useState(toLocalInput(pick.endsAt));
  const update = useMutation({
    mutationFn: (body: PickUpdate) => updatePick(pick.id, body),
    onSuccess: onSaved,
  });
  const errors = update.error ? formErrors(t, update.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const note = formText(new FormData(event.currentTarget), 'note');
    update.mutate({
      note: note || null,
      startsAt: fromLocalInput(from) ?? null,
      endsAt: fromLocalInput(to) ?? null,
    });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.picks.edit.title', { title: pick.title })}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <WindowFields note={pick.note} from={from} to={to} onFrom={setFrom} onTo={setTo} />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={update.isPending}>
            {update.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * Cat's recommendations (M7, E-2): the picks in carousel order with target, note and time window;
 * "Nach oben"/"Nach unten" reorder them (keyboard and mouse), "Bearbeiten" changes note and window,
 * "Entfernen" removes one, "Hinzufügen" adds a title. `?add=<itemId>` (the detail page's link)
 * opens the dialog with that title chosen.
 */
export function PicksPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.picks.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const { add } = route.useSearch();
  const navigate = route.useNavigate();
  const queryClient = useQueryClient();
  const picks = useQuery({ ...picksQuery, enabled: allowed });
  const { buttonRef, keepFocus } = useMoveButtons((picks.data ?? []).map((pick) => pick.id).join());
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<AdminPick | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const header = useRef<HTMLDivElement>(null);
  const addButton = useRef<HTMLButtonElement>(null);
  const listId = useId();

  // The start page shows the picks: it is asked again after every change.
  const invalidate = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: picksQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: ['home'] }),
    ]);

  // Quick presses queue their PUTs in order (scope); each shows its order at once, and only the
  // last one to settle asks the server again, so an earlier answer cannot undo a later move.
  const reorder = useMutation({
    mutationKey: REORDER_KEY,
    scope: { id: REORDER_KEY.join('/') },
    mutationFn: ({ order }: Move) => reorderPicks(order.map((pick) => pick.id)),
    onMutate: async ({ order }) => {
      await queryClient.cancelQueries({ queryKey: picksQuery.queryKey });
      const previous = queryClient.getQueryData(picksQuery.queryKey);
      queryClient.setQueryData(picksQuery.queryKey, order);
      return { previous };
    },
    onError: (_error, { pick, delta }, context) => {
      if (!context?.previous) return;
      // Back in place, the focus returns to the pressed button (as after the move itself).
      if (queryClient.getQueryData(picksQuery.queryKey) !== context.previous) {
        keepFocus(String(pick.id), delta);
      }
      queryClient.setQueryData(picksQuery.queryKey, context.previous);
    },
    onSettled: () =>
      queryClient.isMutating({ mutationKey: REORDER_KEY }) > 1 ? undefined : invalidate(),
  });
  const remove = useMutation({
    mutationFn: (pick: AdminPick) => deletePick(pick.id),
    onSuccess: async (_data, pick) => {
      setMessage(t('admin.picks.remove.done', { title: pick.title }));
      await invalidate();
      // The row and its buttons are gone: the focus goes to the page heading.
      header.current?.focus();
    },
  });
  const actionError = reorder.error ?? remove.error;

  const move = (pick: AdminPick, delta: -1 | 1) => {
    const list = picks.data ?? [];
    const index = list.findIndex((entry) => entry.id === pick.id);
    const order = moveBy(list, index, delta);
    setMessage(t('admin.picks.moved', { title: pick.title, position: index + delta + 1 }));
    keepFocus(String(pick.id), delta);
    reorder.mutate({ order, pick, delta });
  };

  const timeWindow = (pick: AdminPick) => {
    const date = (iso: string) => formatDateTime(iso, i18n.language);
    if (pick.startsAt && pick.endsAt) {
      return t('admin.picks.window.range', { from: date(pick.startsAt), to: date(pick.endsAt) });
    }
    if (pick.startsAt) return t('admin.picks.window.from', { date: date(pick.startsAt) });
    if (pick.endsAt) return t('admin.picks.window.until', { date: date(pick.endsAt) });
    return t('admin.picks.window.always');
  };
  // Expired, scheduled or limited as of the list's fetch (the list is asked again after every
  // change); the dot marks a window, the word names the two states in which the row is missing.
  const now = picks.dataUpdatedAt;
  const scheduled = (pick: AdminPick) => Boolean(pick.startsAt && Date.parse(pick.startsAt) > now);
  const windowState = (pick: AdminPick): { dot: StatusDotState; word?: string } | null => {
    if (pick.endsAt && Date.parse(pick.endsAt) <= now) {
      return { dot: 'warning', word: t('admin.picks.window.expired') };
    }
    if (scheduled(pick)) return { dot: 'pending', word: t('admin.picks.window.scheduled') };
    if (pick.endsAt) return { dot: 'success' };
    return null;
  };

  const count = picks.data?.length ?? 0;
  const planned = picks.data?.filter(scheduled).length ?? 0;
  const entries = t('admin.picks.count.entries', { count });
  const columns: DataColumn<AdminPick>[] = [
    {
      key: 'position',
      header: t('admin.picks.columns.position'),
      align: 'end',
      // The place in the row; the list comes in the order of `position`, which may have gaps
      // after a removal and is stale while a move is on its way to the server.
      cell: (pick) => (
        <span className={styles.numbers}>{(picks.data?.indexOf(pick) ?? 0) + 1}</span>
      ),
    },
    {
      key: 'title',
      header: t('admin.picks.columns.title'),
      rowHeader: true,
      cell: (pick) => (
        <PosterTitle posterUrl={pick.posterUrl}>
          <span className={styles.stack}>
            <span>{pick.title}</span>
            <span className={styles.muted}>{titleMeta(t, pick)}</span>
          </span>
        </PosterTitle>
      ),
    },
    {
      key: 'target',
      header: t('admin.picks.columns.target'),
      cell: (pick) => pick.profileName ?? t('admin.picks.allProfiles'),
    },
    {
      key: 'note',
      header: t('admin.picks.columns.note'),
      cell: (pick) =>
        pick.note === undefined ? (
          t('admin.picks.noNote')
        ) : (
          <span className={layout.note}>{t('admin.picks.quoted', { note: pick.note })}</span>
        ),
    },
    {
      key: 'window',
      header: t('admin.picks.columns.window'),
      cell: (pick) => {
        const state = windowState(pick);
        return (
          <span className={styles.stack}>
            <span className={layout.dot}>
              {state && <StatusDot state={state.dot} />}
              {timeWindow(pick)}
            </span>
            {state?.word && <span className={styles.muted}>{state.word}</span>}
          </span>
        );
      },
    },
    {
      key: 'actions',
      header: t('admin.picks.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (pick) => {
        const index = picks.data?.indexOf(pick) ?? 0;
        return (
          <span className={styles.rowActions}>
            <AriaButton
              ref={buttonRef(String(pick.id), -1)}
              variant="ghost"
              aria-label={t('admin.picks.up.label', { title: pick.title })}
              isDisabled={index === 0}
              onPress={() => move(pick, -1)}
            >
              {t('admin.picks.up.action')}
            </AriaButton>
            <AriaButton
              ref={buttonRef(String(pick.id), 1)}
              variant="ghost"
              aria-label={t('admin.picks.down.label', { title: pick.title })}
              isDisabled={index === count - 1}
              onPress={() => move(pick, 1)}
            >
              {t('admin.picks.down.action')}
            </AriaButton>
            <AriaButton
              variant="ghost"
              aria-label={t('admin.picks.edit.label', { title: pick.title })}
              onPress={() => setEditing(pick)}
            >
              {t('admin.picks.edit.action')}
            </AriaButton>
            <ConfirmDialog
              trigger={
                <AriaButton
                  variant="ghost"
                  icon="trash"
                  aria-label={t('admin.picks.remove.label', { title: pick.title })}
                >
                  {t('admin.picks.remove.action')}
                </AriaButton>
              }
              title={t('admin.picks.remove.title', { title: pick.title })}
              body={t('admin.picks.remove.body')}
              confirmLabel={t('admin.picks.remove.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => remove.mutate(pick)}
            />
          </span>
        );
      },
    },
  ];

  const closeAdd = () => {
    setAdding(false);
    // The link's title is used once: a reload must not open the dialog again.
    if (add) void navigate({ search: {}, replace: true });
    addButton.current?.focus();
  };

  return (
    <Page>
      <div className={layout.top}>
        <div ref={header} tabIndex={-1} className={layout.intro}>
          <Text as="h1" variant="display">
            {t('admin.picks.title')}
          </Text>
          <Text tone="secondary">{t('admin.picks.intro')}</Text>
        </div>
        {allowed && (
          <AriaButton ref={addButton} variant="primary" icon="plus" onPress={() => setAdding(true)}>
            {t('admin.picks.add.action')}
          </AriaButton>
        )}
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <Panel as="section" aria-labelledby={listId}>
          <div className={layout.cardHead}>
            <Text as="h2" variant="title">
              <span id={listId}>{t('admin.picks.list')}</span>
            </Text>
            {picks.data && (
              <span className={layout.small}>
                {planned > 0
                  ? t('admin.picks.count.both', {
                      entries,
                      planned: t('admin.picks.count.planned', { count: planned }),
                    })
                  : entries}
              </span>
            )}
          </div>
          {actionError && <Notice tone="danger">{problemMessage(t, actionError)}</Notice>}
          <div role="status" className={curation.live}>
            {message && !actionError && <Notice live={false}>{message}</Notice>}
          </div>
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
          {picks.data?.length === 0 && <Notice>{t('admin.picks.empty')}</Notice>}
          {picks.data && picks.data.length > 0 && (
            <DataTable
              caption={t('admin.picks.list')}
              columns={columns}
              rows={picks.data}
              rowKey={(pick) => String(pick.id)}
            />
          )}
          <p className={layout.hint}>
            <Icon name="info" size="sm" />
            <span>
              {/* Not <link>: the Trans parser takes it for the void HTML element. */}
              <Trans
                i18nKey="admin.picks.homeRowsHint"
                components={{ page: <TextLink href={paths.adminHomeRows}>{null}</TextLink> }}
              />
            </span>
          </p>
        </Panel>
      )}
      {allowed && (adding || add) && (
        <AddPickDialog
          presetId={adding ? undefined : add}
          onClose={closeAdd}
          onAdded={(title) => {
            setMessage(t('admin.picks.add.done', { title }));
            void invalidate();
            closeAdd();
          }}
        />
      )}
      {editing && (
        <EditPickDialog
          pick={editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setMessage(t('admin.picks.edit.done', { title: editing.title }));
            setEditing(null);
            void invalidate();
          }}
        />
      )}
    </Page>
  );
}
