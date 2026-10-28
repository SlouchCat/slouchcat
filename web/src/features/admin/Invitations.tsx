import type { Invitation, InvitationCreated, InvitationRequest } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { formText } from '@/lib/form';
import { Chip, Notice, Panel, Skeleton } from '@/ui';
import { AriaButton, ConfirmDialog, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import { ratingOf, ratingOptions } from '../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../shared/LibraryChoice';
import { describeLibraries, libraryIdsOf, type LibraryOption } from '../shared/useLibraryOptions';
import { invitationsQuery } from './queries';
import styles from './Invitations.module.css';

const stateTone = {
  open: 'success',
  used: 'info',
  expired: 'neutral',
  revoked: 'warning',
} as const;

/** The link of a just created invitation: shown once, with a copy button. */
function CreatedLink({ created, onDone }: { created: InvitationCreated; onDone: () => void }) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<'copied' | 'failed' | null>(null);
  // The server returns a path; the invited person needs the address of this very server.
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
    <div className={styles.form}>
      <Notice tone="warning">{t('admin.accounts.invitations.created.once')}</Notice>
      <TextField
        label={t('admin.accounts.invitations.created.label')}
        value={link}
        isReadOnly
        mono
        selectOnFocus
        inputRef={input}
      />
      <p role="status" className={styles.status}>
        {status === 'copied' && t('admin.accounts.invitations.created.copied')}
        {status === 'failed' && t('admin.accounts.invitations.created.copyFailed')}
      </p>
      <div className={styles.dialogActions}>
        <AriaButton variant="primary" icon="copy" onPress={() => void copy()}>
          {t('admin.accounts.invitations.created.copy')}
        </AriaButton>
        <AriaButton variant="ghost" onPress={onDone}>
          {t('admin.accounts.invitations.created.dismiss')}
        </AriaButton>
      </div>
    </div>
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
  const [rating, setRating] = useState('18');
  const [selection, setSelection] = useState<LibrarySelection>({ all: true, ids: [] });

  const create = useMutation({
    mutationFn: (body: InvitationRequest): Promise<InvitationCreated> =>
      request(api.POST('/admin/invitations', { body })),
    onSuccess: async (created) => {
      onCreated(created);
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
    <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
      <div className={styles.form}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="note"
          label={t('admin.accounts.invitations.note')}
          description={t('admin.accounts.invitations.noteHint')}
          maxLength={120}
          autoComplete="off"
        />
        <RadioGroup
          name="maxRating"
          label={t('admin.accounts.invitations.form.rating')}
          value={rating}
          onChange={setRating}
          options={ratingOptions(t)}
        />
        <LibraryChoice
          label={t('admin.accounts.invitations.form.libraries')}
          value={selection}
          onChange={setSelection}
          options={options}
        />
        <div className={styles.dialogActions}>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending
              ? t('admin.accounts.invitations.creating')
              : t('admin.accounts.invitations.create')}
          </AriaButton>
        </div>
      </div>
    </Form>
  );
}

/**
 * "Konto einladen" (K-6): the one way a new account comes into being (ADR 0014, no
 * self-registration). The form becomes the created link, which is shown this once and then
 * closes with the dialog.
 */
export function InviteDialog({
  options,
  onClose,
}: {
  options: readonly LibraryOption[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [created, setCreated] = useState<InvitationCreated | null>(null);

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.accounts.invitations.invite')}
      description={t('admin.accounts.invitations.intro')}
    >
      {created ? (
        <CreatedLink created={created} onDone={onClose} />
      ) : (
        <InvitationForm options={options} onCreated={setCreated} />
      )}
    </FormDialog>
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
            {invitation.note || t('admin.accounts.invitations.noNote')}
          </span>
          <Chip tone={stateTone[invitation.state]}>
            {t(`admin.accounts.invitations.state.${invitation.state}`)}
          </Chip>
          <Chip tone="outline">{t('rating.fsk', { age: invitation.maxRating })}</Chip>
        </div>
        <span className={styles.muted}>{describeLibraries(t, invitation, options)}</span>
        <span className={styles.muted}>
          {invitation.state === 'used' && invitation.usedByEmail
            ? t('admin.accounts.invitations.usedBy', { email: invitation.usedByEmail })
            : t('admin.accounts.invitations.expires', {
                date: formatDateTime(invitation.expiresAt, i18n.language),
              })}
        </span>
      </div>
      {invitation.state === 'open' && (
        <ConfirmDialog
          trigger={
            <AriaButton variant="ghost" aria-describedby={noteId}>
              {t('admin.accounts.invitations.revoke.action')}
            </AriaButton>
          }
          title={t('admin.accounts.invitations.revoke.title')}
          body={t('admin.accounts.invitations.revoke.body')}
          confirmLabel={t('admin.accounts.invitations.revoke.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => onRevoke(invitation.id)}
        />
      )}
    </li>
  );
}

/**
 * The invitation links below the account list (K-6): their state, and the way to revoke an open
 * one. A link is created in the dialog above; the account it produces is in the list itself.
 */
export function InvitationsSection({ options }: { options: readonly LibraryOption[] }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const invitations = useQuery(invitationsQuery);
  const revoke = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/admin/invitations/{id}', { params: { path: { id } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey }),
  });

  return (
    <section className={styles.section} aria-labelledby="invitations-heading">
      <h2 id="invitations-heading" className={styles.sectionTitle}>
        {t('admin.accounts.invitations.title')}
      </h2>
      <div className={styles.listBlock} aria-busy={invitations.isPending}>
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
        {invitations.data?.length === 0 && <Notice>{t('admin.accounts.invitations.empty')}</Notice>}
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
