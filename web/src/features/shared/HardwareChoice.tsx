import type { AdminSettings, SystemInfo, TranscodeChoice } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { Notice, Stack } from '@/ui';
import { RadioGroup, useStatusMessage } from '@/ui/aria';
import { Outcome, SourceChip, useSettingsUpdate } from '../admin/integrations/shared';

const HARDWARE = ['nvenc', 'qsv', 'vaapi'] as const;

/**
 * The transcode backend (concept 22, F-10, O-19): «Automatisch (empfohlen)», every hardware
 * backend that passed its probe at start, «Nur Prozessor»; a backend whose probe failed stands
 * greyed out with the reason. The choice is stored at once and applies after a restart; while
 * `--hw` or SLOUCHCAT_HW sets it, it is shown «aus .env» and cannot be changed. The wizard and
 * Verwaltung › Integrationen › Server show the same.
 */
export function HardwareChoice({
  system,
  settings,
  restartHint,
  hideLabel = false,
}: {
  system: SystemInfo;
  settings: AdminSettings;
  /** What happens to a choice that differs from the running backend (the wizard restarts itself). */
  restartHint: string;
  /** The row of a SettingRow names the group already. */
  hideLabel?: boolean;
}) {
  const { t } = useTranslation();
  const [message, setMessage] = useStatusMessage();
  const save = useSettingsUpdate((body) =>
    setMessage(t('system.gpu.choice.saved', { name: label(body.transcodeHw ?? 'auto') })),
  );
  const env = settings.transcodeHwSource === 'env';
  const chosen = save.isPending
    ? (save.variables.transcodeHw ?? settings.transcodeHw)
    : settings.transcodeHw;
  function label(choice: TranscodeChoice): string {
    if (choice === 'auto') return t('system.gpu.choice.auto');
    if (choice === 'software') return t('system.gpu.choice.software');
    return t(`system.gpu.backend.${choice}`);
  }
  const options = [
    { value: 'auto', label: label('auto') },
    ...HARDWARE.map((backend) => {
      const probe = system.gpu.candidates.find((candidate) => candidate.backend === backend);
      return probe?.ok
        ? { value: backend, label: label(backend) }
        : {
            value: backend,
            label: t('system.gpu.choice.unavailable', {
              name: label(backend),
              reason: t(`system.gpu.choice.reason.${probe?.reason ?? 'test_failed'}`),
            }),
            // A stored backend whose probe failed stays selectable: it is the current value.
            isDisabled: backend !== chosen,
          };
    }),
    { value: 'software', label: label('software') },
  ];
  return (
    <Stack gap={2}>
      <RadioGroup
        label={t('system.gpu.choice.label')}
        hideLabel={hideLabel}
        description={env ? t('admin.integrations.envHint') : t('system.gpu.choice.hint')}
        orientation="vertical"
        isDisabled={env}
        value={chosen}
        onChange={(value) => {
          setMessage(null);
          save.mutate({ transcodeHw: value as TranscodeChoice });
        }}
        options={options}
      />
      <SourceChip source={settings.transcodeHwSource} />
      {system.gpu.fallbackFrom && (
        <Notice tone="warning" live={false} title={t('system.gpu.fallback.title')}>
          {t('system.gpu.fallback.body', { name: label(system.gpu.fallbackFrom) })}
        </Notice>
      )}
      {settings.transcodeHwRestart && <Notice live={false}>{restartHint}</Notice>}
      <Outcome error={save.error} message={message} />
    </Stack>
  );
}
