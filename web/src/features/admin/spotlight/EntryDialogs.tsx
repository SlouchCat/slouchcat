import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import {
  setSpotlightEntryOverride,
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

function useOverride(entry: SpotlightEntry, onSaved: EntryDialogProps['onSaved']) {
  return useMutation({
    mutationFn: (body: SpotlightEntryOverride) =>
      setSpotlightEntryOverride(entry.media, entry.tmdbId, body),
    onSuccess: onSaved,
  });
}

function DialogButtons({ isPending, onClose }: { isPending: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  return (
    <div className={styles.dialogActions}>
      <AriaButton variant="ghost" onPress={onClose}>
        {t('common.cancel')}
      </AriaButton>
      <AriaButton type="submit" variant="primary" isPending={isPending}>
        {isPending ? t('common.saving') : t('common.save')}
      </AriaButton>
    </div>
  );
}

/**
 * «Anheften» (SPO-3; concept 12, 10.3): the places of the row as a grid of choices up to
 * «Höchstens» (`itemCount`). A place another entry of the list holds is shown but cannot be
 * chosen, and the description names who holds it; the server still answers 409 `rank_taken` when
 * someone else took it meanwhile. The arrow keys move between the free places.
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
  const update = useOverride(entry, onSaved);
  const errors = update.error ? formErrors(t, update.error) : null;
  const [rank, setRank] = useState<string | null>(null);
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
      initialFocus='input[type="radio"]:not(:disabled)'
    >
      <Form
        className={styles.form}
        onSubmit={onSubmit}
        validationErrors={unchosen ? { pinnedRank: t('errors.field.required') } : errors?.fields}
      >
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
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
            setRank(value);
          }}
          options={ranks.map((place) => ({
            value: String(place),
            label: String(place),
            isDisabled: holders.has(place),
          }))}
        />
        <DialogButtons isPending={update.isPending} onClose={onClose} />
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
