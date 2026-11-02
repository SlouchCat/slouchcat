import type {
  AdminSettings,
  AdminSettingsUpdate,
  SettingSource,
  SMTPTLS,
} from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, Notice } from '@/ui';
import { AriaButton, ConfirmDialog, StatusRegion, type StatusMessage } from '@/ui/aria';
import { dashboardQuery, settingsQuery, updateSettings } from '../queries';
import styles from '../IntegrationsPage.module.css';

// The parts the blocks of the integrations page share: draft ids, the save hooks, the source
// chip, the band's state, the outcome line and the save/delete buttons.

export type KeyName = keyof AdminSettings['keys'];

export const KEYS = [
  'tmdb',
  'tvdb',
  'tvdbPin',
  'youtube',
  'fanart',
  'fanartClient',
  'trakt',
] as const satisfies readonly KeyName[];

export const TLS_MODES = ['starttls', 'tls', 'none'] as const satisfies readonly SMTPTLS[];

export const sourceTone = { env: 'info', stored: 'success', none: 'outline' } as const;

/** Draft ids of the single text values: a key, the bot token, the SMTP password, the address. */
export const keyDraft = (keyName: KeyName) => `keys.${keyName}`;
export const TELEGRAM_DRAFT = 'telegram';
export const PASSWORD_DRAFT = 'email.password';
export const PUBLIC_URL_DRAFT = 'publicUrl';
export const EMAIL_DRAFT = 'email';

/** After a part of the settings is stored: the answer is the whole new state of the page. */
export function useSettingsSaved(onSaved: (body: AdminSettingsUpdate) => void) {
  const queryClient = useQueryClient();
  return async (settings: AdminSettings, body: AdminSettingsUpdate) => {
    queryClient.setQueryData(settingsQuery.queryKey, settings);
    onSaved(body);
    // The dashboard shows the integrations' state.
    await queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
  };
}

/** Stores a part of the settings without a secret (update check, public address). */
export function useSettingsUpdate(onSaved: (body: AdminSettingsUpdate) => void) {
  return useMutation({ mutationFn: updateSettings, onSuccess: useSettingsSaved(onSaved) });
}

/**
 * Stores a part of the settings that may carry a secret (a key, the SMTP password, the bot
 * token): the value never stays in the query client (useTransientMutation).
 */
export function useSecretUpdate(onSaved: (body: AdminSettingsUpdate) => void) {
  return useTransientMutation(updateSettings, useSettingsSaved(onSaved));
}

/** Where a value comes from: deploy/.env, stored here (with its last four characters), or none. */
export function SourceChip({ source, hint }: { source: SettingSource; hint?: string }) {
  const { t } = useTranslation();
  return (
    <Chip tone={sourceTone[source]}>
      {source === 'stored' && hint
        ? t('admin.integrations.source.storedHint', { hint })
        : t(`admin.integrations.source.${source}`)}
    </Chip>
  );
}

/** The band's state: where the block's value comes from, and whether it holds unsaved input. */
export function BandMeta({
  source,
  unsaved,
  count,
}: {
  source?: SettingSource;
  unsaved: boolean;
  /** How much of the block is set up ("2 von 7 gesetzt", "3 Integrationen"). */
  count?: string;
}) {
  const { t } = useTranslation();
  if (!source && !unsaved && !count) return null;
  return (
    <>
      {count && <span>{count}</span>}
      {unsaved && <Chip tone="warning">{t('admin.integrations.unsaved')}</Chip>}
      {source && <SourceChip source={source} />}
    </>
  );
}

/** The outcome of a block's last action: an error notice or a polite status line. */
export function Outcome({ error, message }: { error: unknown; message: StatusMessage | null }) {
  const { t } = useTranslation();
  return (
    <>
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
      <StatusRegion message={error ? null : message} />
    </>
  );
}

/** The buttons of a stored value: save, and delete after a confirmation. */
export function Actions({
  name,
  saveLabel,
  stored,
  isPending,
  onClear,
  children,
}: {
  name: string;
  saveLabel?: string;
  stored: boolean;
  isPending: boolean;
  onClear: () => void;
  children?: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div className={styles.actions}>
      <AriaButton type="submit" variant="primary" isPending={isPending} aria-label={saveLabel}>
        {isPending ? t('admin.integrations.saving') : t('admin.integrations.save')}
      </AriaButton>
      {stored && (
        <ConfirmDialog
          trigger={
            <AriaButton variant="danger" aria-label={t('admin.integrations.clear.label', { name })}>
              {t('admin.integrations.clear.action')}
            </AriaButton>
          }
          title={t('admin.integrations.clear.title', { name })}
          body={t('admin.integrations.clear.body')}
          confirmLabel={t('admin.integrations.clear.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={onClear}
        />
      )}
      {children}
    </div>
  );
}

export interface EmailValues {
  host: string;
  port: string;
  user: string;
  from: string;
  tls: SMTPTLS;
}

export const emailValues = (email: AdminSettings['email']): EmailValues => ({
  host: email.host ?? '',
  port: email.port ? String(email.port) : '587',
  user: email.user ?? '',
  from: email.from ?? '',
  tls: email.tls ?? 'starttls',
});
