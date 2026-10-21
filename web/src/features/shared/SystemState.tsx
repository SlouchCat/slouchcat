import type { GPUInfo, SystemInfo } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { FactList, Notice, Stack, Text } from '@/ui';
import { CodeBlock } from '@/ui/aria';

type ComposeOverride = NonNullable<GPUInfo['composeOverride']>;

const OVERRIDES = ['nvidia', 'intel', 'amd'] as const satisfies readonly ComposeOverride[];

/** The command that starts the stack with the GPU override file of deploy/ (compose.<x>.yaml). */
export const composeCommand = (override: ComposeOverride) =>
  `docker compose -f compose.yaml -f compose.${override}.yaml up -d`;

/** Raises the inotify watch limit on the host (plan 3.4: the wizard warns about a tight limit). */
export const SYSCTL_COMMAND = 'sysctl fs.inotify.max_user_watches=524288';

function Command({ code, label }: { code: string; label: string }) {
  const { t } = useTranslation();
  return (
    <CodeBlock
      code={code}
      label={label}
      copyLabel={t('system.copy.action')}
      copiedMessage={t('system.copy.done')}
      copyFailedMessage={t('system.copy.failed')}
    />
  );
}

interface SystemStateProps {
  system: SystemInfo;
  /** Show how many folders the watcher holds (the dashboard; the wizard has none yet). */
  showUsage?: boolean;
  /** Facts in front of the hardware ones (the dashboard's version and start). */
  facts?: readonly (readonly [string, string])[];
}

/**
 * What the server found in the container (A-1, A-3): the transcode backend with its HDR filter,
 * the inotify limit and the file systems. Without a hardware backend it shows the Compose command
 * that passes the GPU in (the detected maker's, else one per maker); a tight inotify limit comes
 * with the sysctl command. Both commands can be copied.
 */
export function SystemState({ system, showUsage = false, facts = [] }: SystemStateProps) {
  const { t, i18n } = useTranslation();
  const { gpu, inotify } = system;
  const number = (value: number) => new Intl.NumberFormat(i18n.language).format(value);
  const limit =
    inotify.maxUserWatches > 0 ? number(inotify.maxUserWatches) : t('system.inotify.unknown');
  const fs = (type: string) => (type && type !== 'unknown' ? type : t('system.fs.unknown'));
  const overrides = gpu.composeOverride ? [gpu.composeOverride] : OVERRIDES;

  return (
    <Stack gap={4}>
      <FactList
        facts={[
          ...facts,
          [t('system.gpu.encoder'), t(`system.gpu.backend.${gpu.backend}`)],
          [t('system.gpu.tonemap'), t(`system.gpu.tonemapper.${gpu.tonemapper}`)],
          ...(gpu.ffmpegVersion ? [[t('system.gpu.ffmpeg'), gpu.ffmpegVersion] as const] : []),
          showUsage && inotify.maxUserWatches > 0
            ? [
                t('system.inotify.usage'),
                t('system.inotify.usageValue', {
                  used: number(inotify.watchedDirs),
                  limit,
                  percent: number(Math.round(inotify.usagePercent)),
                }),
              ]
            : [t('system.inotify.limit'), limit],
          [t('system.fs.config'), fs(system.configFs)],
          [t('system.fs.cache'), fs(system.cacheFs)],
        ]}
      />
      {gpu.hardwareEncode ? (
        <Text tone="secondary">{t('system.gpu.detected')}</Text>
      ) : (
        <Stack gap={2}>
          <Notice tone="warning" title={t('system.gpu.none.title')}>
            {gpu.composeOverride ? t('system.gpu.none.body') : t('system.gpu.none.unknown')}
          </Notice>
          {overrides.map((override) => (
            <Command
              key={override}
              code={composeCommand(override)}
              label={t('system.gpu.command', { vendor: t(`system.gpu.vendor.${override}`) })}
            />
          ))}
        </Stack>
      )}
      {inotify.warn && (
        <Stack gap={2}>
          <Notice tone="warning" title={t('system.inotify.warn.title')}>
            {t('system.inotify.warn.body')}
          </Notice>
          <Command code={SYSCTL_COMMAND} label={t('system.inotify.command')} />
        </Stack>
      )}
    </Stack>
  );
}
