import { useTranslation } from 'react-i18next';
import { TextField } from '@/ui/aria';

/** Longest note the server takes (`PickRequest.note`). */
export const MAX_NOTE = 280;

/**
 * Note and time window of a pick, shared by «Hinzufügen» and the fold-out of a pick. The end must
 * lie after the start; the browser checks that before anything is sent.
 */
export function WindowFields({
  note,
  from,
  to,
  onNote,
  onFrom,
  onTo,
}: {
  note: string;
  from: string;
  to: string;
  onNote: (value: string) => void;
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
        value={note}
        onChange={onNote}
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
