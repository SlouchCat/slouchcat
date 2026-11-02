import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import {
  setSpotlightEntryOverride,
  spotlightEntriesQuery,
  type SpotlightEntry,
  type SpotlightEntryOverride,
} from '../queries';
import { entryKey, entryName } from './columns';
import styles from './Spotlight.module.css';

/** Longest note the server takes (`SpotlightEntryOverride.note`). */
const MAX_NOTE = 280;

interface EntryDialogProps {
  entry: SpotlightEntry;
  isOpen: boolean;
  onClose: () => void;
  onSaved: (saved: SpotlightEntry, body: SpotlightEntryOverride) => void;
}

function useOverride(
  entry: SpotlightEntry,
  onSaved: EntryDialogProps['onSaved'],
  onError?: (error: unknown) => void,
) {
  return useMutation({
    mutationFn: (body: SpotlightEntryOverride) =>
      setSpotlightEntryOverride(entry.media, entry.tmdbId, body),
    onSuccess: onSaved,
    onError,
  });
}

function DialogButtons({
  isPending,
  onClose,
  saveDisabled,
  cancelRef,
}: {
  isPending: boolean;
  onClose: () => void;
  saveDisabled?: boolean;
  cancelRef?: Ref<HTMLButtonElement>;
}) {
  const { t } = useTranslation();
  return (
    <div className={styles.dialogActions}>
      <AriaButton ref={cancelRef} variant="ghost" onPress={onClose}>
        {t('common.cancel')}
      </AriaButton>
      <AriaButton type="submit" variant="primary" isPending={isPending} isDisabled={saveDisabled}>
        {isPending ? t('common.saving') : t('common.save')}
      </AriaButton>
    </div>
  );
}

/**
 * «Anheften» (SPO-3; concept 12, 10.3): the places of the row as a grid of choices up to
 * «Höchstens» (`itemCount`). A place another entry of the list holds is shown but cannot be
 * chosen, and the description names who holds it; the server still answers 409 `rank_taken` when
 * someone else took it meanwhile, and the entries are asked again so the grid shows the new holder.
 * With every place taken the dialog says so, «Speichern» is off and the focus starts on
 * «Abbrechen». The arrow keys move between the free places.
 */
export function PinDialog({
  entry,
  isOpen,
  places,
  others,
  onClose,
  onSaved,
}: EntryDialogProps & {
  /** «Höchstens» as stored: the number of places offered. */
  places: number;
  /** The entries of the list (and its orphaned overrides) that may hold a place. */
  others: readonly SpotlightEntry[];
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const update = useOverride(entry, onSaved, (error) => {
    if (isApiError(error) && error.code === 'rank_taken') {
      void queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey });
    }
  });
  const errors = update.error ? formErrors(t, update.error) : null;
  const cancel = useRef<HTMLButtonElement>(null);
  const [picked, setPicked] = useState<string | null>(null);
  // Saving without a place says so at the grid; choosing one takes the message away.
  const [unchosen, setUnchosen] = useState(false);
  const holders = new Map(
    others
      .filter((other) => entryKey(other) !== entryKey(entry) && other.pinnedRank !== undefined)
      .map((other) => [other.pinnedRank as number, entryName(t, other)]),
  );
  const ranks = Array.from({ length: places }, (_, index) => index + 1);
  const taken = ranks
    .filter((place) => holders.has(place))
    .map((place) => t('admin.spotlight.pin.takenPlace', { rank: place, title: holders.get(place) }))
    .join(t('admin.spotlight.state.separator'));
  const allTaken = ranks.every((place) => holders.has(place));
  // A place that someone took meanwhile is no choice any more.
  const rank = picked !== null && !holders.has(Number(picked)) ? picked : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (rank === null) {
      setUnchosen(true);
      return;
    }
    update.mutate({ pinnedRank: Number(rank) });
  };

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.spotlight.pin.title', { title: entryName(t, entry) })}
      initialFocus={allTaken ? cancel : 'input[type="radio"]:not(:disabled)'}
    >
      <Form
        className={styles.form}
        onSubmit={onSubmit}
        validationErrors={unchosen ? { pinnedRank: t('errors.field.required') } : errors?.fields}
      >
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        {allTaken && <Notice tone="warning">{t('admin.spotlight.pin.allTaken')}</Notice>}
        <RadioGroup
          name="pinnedRank"
          label={t('admin.spotlight.pin.rank')}
          description={
            taken
              ? t('admin.spotlight.pin.taken', {
                  hint: t('admin.spotlight.pin.rankHint'),
                  places: taken,
                })
              : t('admin.spotlight.pin.rankHint')
          }
          value={rank}
          onChange={(value) => {
            setUnchosen(false);
            setPicked(value);
          }}
          options={ranks.map((place) => ({
            value: String(place),
            label: String(place),
            isDisabled: holders.has(place),
          }))}
        />
        <DialogButtons
          isPending={update.isPending}
          onClose={onClose}
          saveDisabled={allTaken}
          cancelRef={cancel}
        />
      </Form>
    </FormDialog>
  );
}

/**
 * «Notiz» of an entry (SPO-3). An emptied note removes it; an unchanged one closes the dialog
 * without a request, because an empty body would remove the whole override.
 */
export function NoteDialog({ entry, isOpen, onClose, onSaved }: EntryDialogProps) {
  const { t } = useTranslation();
  const update = useOverride(entry, onSaved);
  const errors = update.error ? formErrors(t, update.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const note = formText(new FormData(event.currentTarget), 'note') || null;
    if (note === (entry.note ?? null)) {
      onClose();
      return;
    }
    update.mutate({ note });
  };

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.spotlight.note.title', { title: entryName(t, entry) })}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="note"
          label={t('admin.spotlight.note.field')}
          description={t('admin.spotlight.note.hint', { max: MAX_NOTE })}
          defaultValue={entry.note}
          maxLength={MAX_NOTE}
          autoComplete="off"
        />
        <DialogButtons isPending={update.isPending} onClose={onClose} />
      </Form>
    </FormDialog>
  );
}
