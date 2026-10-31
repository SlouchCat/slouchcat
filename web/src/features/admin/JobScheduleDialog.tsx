import type { JobScheduleUpdate, JobTypeInfo } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { Notice, SelectField, Text } from '@/ui';
import { AriaButton, Form, FormDialog, Switch, TextField } from '@/ui/aria';
import { formatMinute, typeName } from './jobsFormat';
import { updateJobSchedule } from './queries';
import styles from './JobsPage.module.css';

/** The intervals the dialog offers, in seconds; the server's limits of the type filter them. */
const EVERY_OPTIONS = [
  ['m5', 5 * 60],
  ['m15', 15 * 60],
  ['m30', 30 * 60],
  ['h1', 3600],
  ['h2', 2 * 3600],
  ['h6', 6 * 3600],
  ['h12', 12 * 3600],
  ['h24', 24 * 3600],
  ['h48', 48 * 3600],
  ['d7', 7 * 24 * 3600],
] as const;

/** Minutes of the day of an `<input type="time">` value ("03:00"); null when it is not a time. */
function minuteOf(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  return Number(match[1]) * 60 + Number(match[2]);
}

/**
 * The dialog "Zeitplan von X" (concept 09, F-4): the switch, the interval as a select of the
 * values within the type's limits, and a time window with two time fields; Save writes the
 * schedule, and the card shows the new next run. The server checks the limits (rule 12); a 422
 * names the field.
 */
export function JobScheduleDialog({
  info,
  isOpen,
  onOpenChange,
  onSaved,
}: {
  info: JobTypeInfo;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onSaved: (saved: JobTypeInfo) => void;
}) {
  const { t } = useTranslation();
  const schedule = info.schedule;
  const name = typeName(t, info.type);
  const [enabled, setEnabled] = useState(schedule?.enabled ?? true);
  const [every, setEvery] = useState(String(schedule?.everySeconds ?? 0));
  const [windowed, setWindowed] = useState(schedule?.window !== undefined);
  const [start, setStart] = useState(
    schedule?.window ? formatMinute(schedule.window.startMinute) : '03:00',
  );
  const [end, setEnd] = useState(
    schedule?.window ? formatMinute(schedule.window.endMinute) : '05:00',
  );
  const save = useMutation({
    mutationFn: (body: JobScheduleUpdate) => updateJobSchedule(info.type, body),
    onSuccess: (saved) => {
      onSaved(saved);
      onOpenChange(false);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  if (!schedule) return null;

  const options = EVERY_OPTIONS.filter(
    ([, seconds]) =>
      seconds >= schedule.limits.minEverySeconds && seconds <= schedule.limits.maxEverySeconds,
  ).map(([key, seconds]) => ({
    value: String(seconds),
    label: t(`admin.jobs.schedule.everyOptions.${key}`),
  }));
  // The stored interval may lie outside the offered steps (set by hand elsewhere): keep it.
  if (!options.some((option) => option.value === every)) {
    options.unshift({ value: every, label: every });
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const body: JobScheduleUpdate = { enabled, everySeconds: Number(every) };
    if (windowed) {
      const startMinute = minuteOf(start);
      const endMinute = minuteOf(end);
      if (startMinute === null || endMinute === null) return;
      body.window = { startMinute, endMinute };
    } else if (schedule.window) {
      body.clearWindow = true;
    }
    save.mutate(body);
  };

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) save.reset();
        onOpenChange(open);
      }}
      title={t('admin.jobs.schedule.title', { name })}
      description={t('admin.jobs.schedule.description')}
      initialFocus="[role=switch]"
    >
      <Form className={styles.fields} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <Switch isSelected={enabled} onChange={setEnabled}>
          {t('admin.jobs.schedule.enabled')}
        </Switch>
        <SelectField
          label={t('admin.jobs.schedule.every')}
          value={every}
          options={options}
          onChange={setEvery}
        />
        {errors?.fields.everySeconds && (
          <Text tone="secondary" variant="caption">
            {errors.fields.everySeconds}
          </Text>
        )}
        <Switch isSelected={windowed} onChange={setWindowed}>
          {t('admin.jobs.schedule.window')}
        </Switch>
        {windowed && (
          <div className={styles.windowFields}>
            <TextField
              name="windowStart"
              type="time"
              label={t('admin.jobs.schedule.windowStart')}
              value={start}
              onChange={setStart}
              isRequired
            />
            <TextField
              name="windowEnd"
              type="time"
              label={t('admin.jobs.schedule.windowEnd')}
              description={t('admin.jobs.schedule.windowHint', { zone: schedule.timeZone })}
              value={end}
              onChange={setEnd}
              isRequired
            />
          </div>
        )}
        {errors?.fields.window && (
          <Text tone="secondary" variant="caption">
            {errors.fields.window}
          </Text>
        )}
        {info.type === 'trakt.refresh' && (
          <Text tone="secondary" variant="caption">
            {t('admin.jobs.schedule.traktHint')}
          </Text>
        )}
        {info.type === 'update.check' && (
          <Text tone="secondary" variant="caption">
            {t('admin.jobs.schedule.updateHint')}
          </Text>
        )}
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {t('admin.jobs.schedule.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
