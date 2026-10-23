import type { Invitation, InvitationCreated, InvitationRequest } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { formText } from '@/lib/form';
import { Chip, Notice, Panel, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, Form, RadioGroup, TextField } from '@/ui/aria';
import { ratingOf, ratingOptions } from '../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../shared/LibraryChoice';
import {
  describeLibraries,
  libraryIdsOf,
  type LibraryOption,
  type LibraryOptions,
} from '../shared/useLibraryOptions';
import { invitationsQuery } from './queries';
import styles from './AccessPage.module.css';

const stateTone = {
  open: 'success',
  used: 'info',
  expired: 'neutral',
  revoked: 'warning',
} as const;

/** The link of a just created invitation: shown once, with a copy button. */
function CreatedLink({
  created,
  onDismiss,
}: {
  created: InvitationCreated;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<'copied' | 'failed' | null>(null);
  // The server returns a path; the guest needs the address of this very server.
  const link = `${window.location.origin}${created.url}`;

  // The link is shown once: put the cursor on it (selected), so Ctrl+C is all that is left to do.
  useEffect(() => {
    input.current?.focus();
  }, []);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(link);
      setStatus('copied');
    } catch {
      // No clipboard on plain-HTTP origins: select the text for Ctrl+C instead.
      input.current?.focus();
      input.current?.select();
      setStatus('failed');
    }
  };

  return (
    <Panel as="section" aria-labelledby="created-link-heading">
      <h3 id="created-link-heading" className={styles.panelTitle}>
        {t('admin.access.invitations.created.title')}
      </h3>
      <Notice tone="warning">{t('admin.access.invitations.created.once')}</Notice>
      <TextField
        label={t('admin.access.invitations.created.label')}
        value={link}
        isReadOnly
        mono
        selectOnFocus
        inputRef={input}
      />
      <div className={styles.actions}>
        <AriaButton variant="primary" icon="copy" onPress={() => void copy()}>
          {t('admin.access.invitations.created.copy')}
        </AriaButton>
        <AriaButton variant="ghost" onPress={onDismiss}>
          {t('admin.access.invitations.created.dismiss')}
        </AriaButton>
      </div>
      <p role="status" className={styles.status}>
        {status === 'copied' && t('admin.access.invitations.created.copied')}
        {status === 'failed' && t('admin.access.invitations.created.copyFailed')}
      </p>
    </Panel>
  );
}

function InvitationForm({
  options,
  onCreated,
}: {
  options: readonly LibraryOption[];
  onCreated: (created: InvitationCreated) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const form = useRef<HTMLFormElement>(null);
  const [rating, setRating] = useState('18');
  const [selection, setSelection] = useState<LibrarySelection>({ all: true, ids: [] });

  const create = useMutation({
    mutationFn: (body: InvitationRequest): Promise<InvitationCreated> =>
      request(api.POST('/admin/invitations', { body })),
    onSuccess: async (created) => {
      onCreated(created);
      form.current?.reset();
      setRating('18');
      setSelection({ all: true, ids: [] });
      await queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey });
    },
  });
  const errors = create.error ? formErrors(t, create.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const note = formText(data, 'note');
    create.mutate({
      ...(note ? { note } : {}),
      maxRating: ratingOf(rating),
      allLibraries: selection.all,
      libraryIds: selection.all ? [] : libraryIdsOf(selection.ids),
    });
  };

  return (
    <Panel as="section" aria-labelledby="new-invitation-heading">
      <h3 id="new-invitation-heading" className={styles.panelTitle}>
        {t('admin.access.invitations.create')}
      </h3>
      <Form
        ref={form}
        className={styles.form}
        onSubmit={onSubmit}
        validationErrors={errors?.fields}
      >
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="note"
          label={t('admin.access.invitations.note')}
          description={t('admin.access.invitations.noteHint')}
          maxLength={120}
          autoComplete="off"
        />
        <RadioGroup
          name="maxRating"
          label={t('admin.access.form.rating')}
          value={rating}
          onChange={setRating}
          options={ratingOptions(t)}
        />
        <LibraryChoice
          label={t('admin.access.form.libraries')}
          value={selection}
          onChange={setSelection}
          options={options}
        />
        <div>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending
              ? t('admin.access.invitations.creating')
              : t('admin.access.invitations.create')}
          </AriaButton>
        </div>
      </Form>
    </Panel>
  );
}

function InvitationRow({
  invitation,
  options,
  onRevoke,
}: {
  invitation: Invitation;
  options: readonly LibraryOption[];
  onRevoke: (id: number) => void;
}) {
  const { t, i18n } = useTranslation();
  const noteId = `invitation-${invitation.id}`;
  return (
    <li className={styles.row}>
      <div className={styles.rowMain}>
        <div className={styles.rowHead}>
          <span id={noteId} className={invitation.note ? styles.strong : styles.muted}>
            {invitation.note || t('admin.access.invitations.noNote')}
          </span>
          <Chip tone={stateTone[invitation.state]}>
            {t(`admin.access.invitations.state.${invitation.state}`)}
          </Chip>
          <Chip tone="outline">{t('rating.fsk', { age: invitation.maxRating })}</Chip>
        </div>
        <span className={styles.muted}>{describeLibraries(t, invitation, options)}</span>
        <span className={styles.muted}>
          {invitation.state === 'used' && invitation.usedByEmail
            ? t('admin.access.invitations.usedBy', { email: invitation.usedByEmail })
            : t('admin.access.invitations.expires', {
                date: formatDateTime(invitation.expiresAt, i18n.language),
              })}
        </span>
      </div>
      {invitation.state === 'open' && (
        <ConfirmDialog
          trigger={
            <AriaButton variant="ghost" aria-describedby={noteId}>
              {t('admin.access.invitations.revoke.action')}
            </AriaButton>
          }
          title={t('admin.access.invitations.revoke.title')}
          body={t('admin.access.invitations.revoke.body')}
          confirmLabel={t('admin.access.invitations.revoke.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => onRevoke(invitation.id)}
        />
      )}
    </li>
  );
}

/** Invitation links (K-6): create one, copy its link once, see and revoke the others. */
export function InvitationsSection({ libraries }: { libraries: LibraryOptions }) {
  const { options } = libraries;
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const invitations = useQuery(invitationsQuery);
  const [created, setCreated] = useState<InvitationCreated | null>(null);
  const revoke = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/admin/invitations/{id}', { params: { path: { id } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey }),
  });

  return (
    <section className={styles.section} aria-labelledby="invitations-heading">
      <div className={styles.sectionHead}>
        <h2 id="invitations-heading" className={styles.sectionTitle}>
          {t('admin.access.invitations.title')}
        </h2>
        <Text tone="secondary">{t('admin.access.invitations.intro')}</Text>
      </div>
      {created && (
        <CreatedLink key={created.token} created={created} onDismiss={() => setCreated(null)} />
      )}
      {libraries.isReady && <InvitationForm options={options} onCreated={setCreated} />}
      {libraries.isPending && <Skeleton shape="block" />}
      {libraries.isError && (
        <Notice
          tone="danger"
          title={t('libraries.loadFailed')}
          actions={
            <AriaButton icon="refresh" onPress={libraries.retry}>
              {t('common.retry')}
            </AriaButton>
          }
        >
          {problemMessage(t, libraries.error)}
        </Notice>
      )}

      <div className={styles.listBlock} aria-busy={invitations.isPending}>
        <h3 className={styles.listTitle}>{t('admin.access.invitations.list')}</h3>
        {revoke.error && <Notice tone="danger">{problemMessage(t, revoke.error)}</Notice>}
        {invitations.isPending && <Skeleton shape="block" />}
        {invitations.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, invitations.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void invitations.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {invitations.data?.length === 0 && <Notice>{t('admin.access.invitations.empty')}</Notice>}
        {invitations.data && invitations.data.length > 0 && (
          <Panel>
            <ul className={styles.rows}>
              {invitations.data.map((invitation) => (
                <InvitationRow
                  key={invitation.id}
                  invitation={invitation}
                  options={options}
                  onRevoke={(id) => revoke.mutate(id)}
                />
              ))}
            </ul>
          </Panel>
        )}
      </div>
    </section>
  );
}
