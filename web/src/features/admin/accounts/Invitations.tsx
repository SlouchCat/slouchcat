import type { Invitation, InvitationCreated, InvitationRequest } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { formText } from '@/lib/form';
import { Chip, Notice } from '@/ui';
import {
  AriaButton,
  DataTable,
  Form,
  FormDialog,
  RadioGroup,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import { ratingOf, ratingOptions } from '../../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../../shared/LibraryChoice';
import {
  describeLibraries,
  libraryIdsOf,
  type LibraryOption,
} from '../../shared/useLibraryOptions';
import { invitationsQuery } from '../queries';
import styles from './Accounts.module.css';

const stateTone = {
  open: 'success',
  used: 'info',
  expired: 'neutral',
  revoked: 'warning',
} as const;

/** The note of an invitation, or «Ohne Notiz». */
export const invitationNote = (t: TFunction, invitation: Invitation) =>
  invitation.note || t('admin.accounts.invitations.noNote');

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
 * "Konto einladen" (K-6, KON-1): the one way a new account comes into being (ADR 0014, no
 * self-registration). The form becomes the created link, which is shown this once and then
 * closes with the dialog.
 */
export function InviteDialog({
  options,
  onCreated,
  onClose,
}: {
  options: readonly LibraryOption[];
  onCreated: () => void;
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
        <InvitationForm
          options={options}
          onCreated={(next) => {
            setCreated(next);
            onCreated();
          }}
        />
      )}
    </FormDialog>
  );
}

/**
 * The invitation links (K-6, KON-5) in the tab «Einladungen»: note, state, rating, libraries and
 * until when it is valid or who accepted it; an open one can be revoked (the page asks). The
 * account an invitation produced is a row of the tab «Konten».
 */
export function InvitationsTable({
  invitations,
  options,
  onRevoke,
}: {
  invitations: readonly Invitation[];
  options: readonly LibraryOption[];
  onRevoke: (invitation: Invitation) => void;
}) {
  const { t, i18n } = useTranslation();
  const noteId = (invitation: Invitation) => `invitation-${invitation.id}`;
  const columns: DataColumn<Invitation>[] = [
    {
      key: 'note',
      header: t('admin.accounts.invitations.columns.note'),
      rowHeader: true,
      cell: (invitation) => (
        <span id={noteId(invitation)} className={invitation.note ? styles.strong : styles.muted}>
          {invitationNote(t, invitation)}
        </span>
      ),
    },
    {
      key: 'state',
      header: t('admin.accounts.invitations.columns.state'),
      cell: (invitation) => (
        <Chip tone={stateTone[invitation.state]}>
          {t(`admin.accounts.invitations.state.${invitation.state}`)}
        </Chip>
      ),
    },
    {
      key: 'rating',
      header: t('admin.accounts.invitations.columns.rating'),
      cell: (invitation) => (
        <span className={styles.nowrap}>{t('rating.fsk', { age: invitation.maxRating })}</span>
      ),
    },
    {
      key: 'libraries',
      header: t('admin.accounts.invitations.columns.libraries'),
      cell: (invitation) => describeLibraries(t, invitation, options),
    },
    {
      key: 'detail',
      header: t('admin.accounts.invitations.columns.detail'),
      cell: (invitation) =>
        invitation.state === 'used' && invitation.usedByEmail
          ? t('admin.accounts.invitations.usedBy', { email: invitation.usedByEmail })
          : t('admin.accounts.invitations.expires', {
              date: formatDateTime(invitation.expiresAt, i18n.language),
            }),
    },
    {
      key: 'actions',
      header: t('admin.accounts.invitations.columns.actions'),
      hideHeader: true,
      align: 'end',
      // Only an open link can be revoked (KON-5).
      cell: (invitation) =>
        invitation.state === 'open' && (
          <AriaButton
            variant="danger"
            aria-describedby={noteId(invitation)}
            onPress={() => onRevoke(invitation)}
          >
            {t('admin.accounts.invitations.revoke.action')}
          </AriaButton>
        ),
    },
  ];
  return (
    <DataTable
      caption={t('admin.accounts.invitations.list')}
      columns={columns}
      rows={invitations}
      rowKey={(invitation) => String(invitation.id)}
    />
  );
}
