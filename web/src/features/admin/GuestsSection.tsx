import type { Guest, GuestUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, Panel, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, FormDialog, RadioGroup } from '@/ui/aria';
import { ratingOf, ratingOptions } from '../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../shared/LibraryChoice';
import {
  LimitFields,
  limitParts,
  limitsOf,
  limitValues,
  type LimitValues,
} from '../shared/LimitFields';
import {
  describeLibraries,
  libraryIdsOf,
  type LibraryOption,
  type LibraryOptions,
} from '../shared/useLibraryOptions';
import { guestsQuery, invitationsQuery } from './queries';
import styles from './AccessPage.module.css';

function GuestForm({
  guest,
  options,
  onClose,
}: {
  guest: Guest;
  options: readonly LibraryOption[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [rating, setRating] = useState(String(guest.maxRating));
  const [selection, setSelection] = useState<LibrarySelection>({
    all: guest.allLibraries,
    ids: guest.libraryIds.map(String),
  });
  const [limits, setLimits] = useState<LimitValues>(() => limitValues(guest.limits));
  const save = useMutation({
    mutationFn: (body: GuestUpdate): Promise<Guest> =>
      request(api.PATCH('/admin/guests/{id}', { params: { path: { id: guest.id } }, body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: guestsQuery.queryKey });
      onClose();
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate({
      maxRating: ratingOf(rating),
      allLibraries: selection.all,
      libraryIds: selection.all ? [] : libraryIdsOf(selection.ids),
      // W-8: bandwidth, quality and streams cap every profile of the guest account.
      limits: limitsOf(limits, false),
    });
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
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
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t('limits.title')}</legend>
        <Text tone="secondary" variant="caption">
          {t('admin.access.guests.limitsHint')}
        </Text>
        <LimitFields value={limits} onChange={setLimits} withWatch={false} />
      </fieldset>
      <div className={styles.dialogActions}>
        <AriaButton variant="ghost" onPress={onClose}>
          {t('common.cancel')}
        </AriaButton>
        <AriaButton type="submit" variant="primary" isPending={save.isPending}>
          {save.isPending ? t('admin.access.guests.saving') : t('admin.access.guests.save')}
        </AriaButton>
      </div>
    </Form>
  );
}

/** Guest accounts (K-7): ceiling and libraries, and removal that ends their sessions at once. */
export function GuestsSection({ libraries }: { libraries: LibraryOptions }) {
  const { options } = libraries;
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const guests = useQuery(guestsQuery);
  const [editing, setEditing] = useState<Guest | null>(null);
  const remove = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/admin/guests/{id}', { params: { path: { id } } })),
    // The invitation the guest used no longer names an account.
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: guestsQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey }),
      ]),
  });

  return (
    <section className={styles.section} aria-labelledby="guests-heading">
      <div className={styles.sectionHead}>
        <h2 id="guests-heading" className={styles.sectionTitle}>
          {t('admin.access.guests.title')}
        </h2>
        <Text tone="secondary">{t('admin.access.guests.intro')}</Text>
      </div>

      <div className={styles.listBlock} aria-busy={guests.isPending}>
        {remove.error && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
        {guests.isPending && <Skeleton shape="block" />}
        {guests.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, guests.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void guests.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {guests.data?.length === 0 && <Notice>{t('admin.access.guests.empty')}</Notice>}
        {guests.data && guests.data.length > 0 && (
          <Panel>
            <ul className={styles.rows} aria-label={t('admin.access.guests.list')}>
              {guests.data.map((guest) => {
                const emailId = `guest-${guest.id}`;
                return (
                  <li key={guest.id} className={styles.row}>
                    <div className={styles.rowMain}>
                      <div className={styles.rowHead}>
                        <span id={emailId} className={styles.strong}>
                          {guest.email}
                        </span>
                        <Chip tone="outline">{t('rating.fsk', { age: guest.maxRating })}</Chip>
                        <Chip tone="info">
                          {t('admin.access.guests.profiles', { count: guest.profileCount })}
                        </Chip>
                        {limitParts(t, guest.limits).map((part) => (
                          <Chip key={part} tone="outline">
                            {part}
                          </Chip>
                        ))}
                      </div>
                      <span className={styles.muted}>{describeLibraries(t, guest, options)}</span>
                      <span className={styles.muted}>
                        {guest.lastSeenAt
                          ? t('admin.access.guests.lastSeen', {
                              date: formatDateTime(guest.lastSeenAt, i18n.language),
                            })
                          : t('admin.access.guests.neverSeen')}
                      </span>
                    </div>
                    <div className={styles.rowActions}>
                      <AriaButton
                        aria-describedby={emailId}
                        isDisabled={!libraries.isReady}
                        onPress={() => setEditing(guest)}
                      >
                        {t('admin.access.guests.edit')}
                      </AriaButton>
                      <ConfirmDialog
                        trigger={
                          <AriaButton variant="ghost" aria-describedby={emailId}>
                            {t('admin.access.guests.remove.action')}
                          </AriaButton>
                        }
                        title={t('admin.access.guests.remove.title', { email: guest.email })}
                        body={t('admin.access.guests.remove.body')}
                        confirmLabel={t('admin.access.guests.remove.confirm')}
                        cancelLabel={t('common.cancel')}
                        tone="danger"
                        onConfirm={() => remove.mutate(guest.id)}
                      />
                    </div>
                  </li>
                );
              })}
            </ul>
          </Panel>
        )}
      </div>

      {editing && (
        <FormDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) setEditing(null);
          }}
          title={t('admin.access.guests.editTitle', { email: editing.email })}
        >
          <GuestForm guest={editing} options={options} onClose={() => setEditing(null)} />
        </FormDialog>
      )}
    </section>
  );
}
