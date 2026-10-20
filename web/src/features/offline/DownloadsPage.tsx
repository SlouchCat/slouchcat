import { useMutation } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { PinDialog } from '@/features/profiles/PinDialog';
import { formatBytes } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import { ButtonLink, Chip, Notice, Page, ProgressBar, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, DataTable, type DataColumn } from '@/ui/aria';
import { removeDownload } from './actions';
import { DownloadRulesForm } from './DownloadRulesForm';
import { downloadStatus, downloadTitle, expectedBytes, qualityLabel } from './labels';
import { pinRequirement } from './pin';
import { StoragePanel } from './StoragePanel';
import { downloadsBytes } from './storage';
import { networkKind, offlineSupported } from './support';
import { downloadsForProfile, waitsForWifi } from './sync';
import type { StoredDownload, StoredProfile } from './types';
import { useStoredDownloads, useStoredProfiles } from './useOfflineData';
import { usePreparingPoll } from './usePreparingPoll';
import styles from './OfflinePages.module.css';

/**
 * Downloads of the active profile on this device (M5): state and progress of each, play and
 * delete, the storage of the device (O-7) and the auto-download rule (O-3). Only downloads whose
 * `allowedProfileIds` (computed by the server) name the profile appear (O-7).
 */
export function DownloadsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('offline.downloads.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const session = useActiveSession();
  const supported = offlineSupported();

  return (
    <Page>
      <div className={styles.header} id="downloads-top" tabIndex={-1}>
        <Text as="h1" variant="display">
          {t('offline.downloads.title')}
        </Text>
        <Text tone="secondary">{t('offline.downloads.intro', { name: session.profile.name })}</Text>
      </div>
      {supported ? (
        <DownloadsContent />
      ) : (
        <Notice tone="warning" title={t('offline.downloads.unsupported.title')}>
          {t('offline.downloads.unsupported.body')}
        </Notice>
      )}
    </Page>
  );
}

function DownloadsContent() {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const online = useOnline();
  const downloads = useStoredDownloads();
  const profiles = useStoredProfiles();
  const [pinOpen, setPinOpen] = useState(false);
  const [removed, setRemoved] = useState<string | null>(null);
  const top = useRef<HTMLElement | null>(null);
  const remove = useMutation({
    mutationFn: (download: StoredDownload) => removeDownload(download.id, online),
    // Offline the delete happens on the device and waits for the server (removeDownload); the
    // query client must not hold it back until the network returns.
    networkMode: 'always',
    onSuccess: (_data, download) => {
      setRemoved(downloadTitle(t, download.item));
      // The row and its button are gone: the focus goes to the page heading.
      top.current ??= document.getElementById('downloads-top');
      top.current?.focus();
    },
  });
  const { account, profile } = session;
  const list = downloadsForProfile(downloads ?? [], account.id, profile.id);
  usePreparingPoll(list);

  if (!downloads || !profiles) return <Skeleton shape="block" />;

  const stored: StoredProfile | undefined = profiles.find((entry) => entry.id === profile.id);
  // A protected profile opens offline only with a verifier of its PIN on this device (AN-19).
  const needsPin =
    list.length > 0 &&
    profile.hasPin &&
    stored !== undefined &&
    pinRequirement(stored) === 'unavailable';
  const network = networkKind();
  const bytes = (value: number) => formatBytes(value, i18n.language);

  const columns: DataColumn<StoredDownload>[] = [
    {
      key: 'title',
      header: t('offline.downloads.columns.title'),
      rowHeader: true,
      cell: (download) => (
        <span className={styles.stack}>
          <span>{downloadTitle(t, download.item)}</span>
          <span className={styles.chips}>
            <Chip tone="outline">{qualityLabel(t, download.quality)}</Chip>
            {download.auto && <Chip tone="info">{t('offline.downloads.auto')}</Chip>}
          </span>
        </span>
      ),
    },
    {
      key: 'state',
      header: t('offline.downloads.columns.state'),
      cell: (download) => {
        const status = downloadStatus(t, download, waitsForWifi(download, profiles, network));
        return (
          <span
            className={styles.stack}
            data-download-state={download.complete ? 'complete' : download.state}
          >
            <Chip tone={status.tone}>{status.text}</Chip>
            {status.percent !== undefined && (
              <ProgressBar value={status.percent} label={status.text} />
            )}
          </span>
        );
      },
    },
    {
      key: 'size',
      header: t('offline.downloads.columns.size'),
      align: 'end',
      cell: (download) => (
        <span className={styles.numbers}>
          {download.complete
            ? bytes(expectedBytes(download))
            : t('offline.downloads.sizeOf', {
                done: bytes(download.bytesWritten),
                total: bytes(expectedBytes(download)),
              })}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('offline.downloads.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (download) => {
        const title = downloadTitle(t, download.item);
        return (
          <span className={styles.rowActions}>
            {download.complete && (
              <ButtonLink
                href={paths.offlinePlay(download.id)}
                variant="secondary"
                icon="play"
                aria-label={t('offline.downloads.playLabel', { title })}
              >
                {t('offline.downloads.play')}
              </ButtonLink>
            )}
            <ConfirmDialog
              trigger={
                <AriaButton
                  variant="ghost"
                  icon="trash"
                  aria-label={t('offline.downloads.delete.label', { title })}
                >
                  {t('offline.downloads.delete.action')}
                </AriaButton>
              }
              title={t('offline.downloads.delete.title', { title })}
              body={t('offline.downloads.delete.body')}
              confirmLabel={t('offline.downloads.delete.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => remove.mutate(download)}
            />
          </span>
        );
      },
    },
  ];

  return (
    <div className={styles.body}>
      {!online && <Notice>{t('offline.downloads.offline')}</Notice>}
      {remove.error && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
      {removed && !remove.error && (
        <Notice>{t('offline.downloads.removed', { title: removed })}</Notice>
      )}
      {needsPin && (
        <Notice
          tone="warning"
          title={t('offline.downloads.pinHint.title')}
          actions={
            <AriaButton icon="lock" onPress={() => setPinOpen(true)} isDisabled={!online}>
              {t('offline.downloads.pinHint.action')}
            </AriaButton>
          }
        >
          {t('offline.downloads.pinHint.body', { name: profile.name })}
        </Notice>
      )}
      <section className={styles.section} aria-labelledby="downloads-list-heading">
        <h2 id="downloads-list-heading" className={styles.sectionTitle}>
          {t('offline.downloads.list')}
        </h2>
        {list.length === 0 ? (
          <Notice>{t('offline.downloads.empty')}</Notice>
        ) : (
          <DataTable
            caption={t('offline.downloads.list')}
            columns={columns}
            rows={list}
            rowKey={(download) => download.id}
          />
        )}
      </section>
      <StoragePanel downloadBytes={downloadsBytes(downloads.filter((d) => !d.pendingDelete))} />
      {online && <DownloadRulesForm profileName={profile.name} />}
      {pinOpen && (
        <PinDialog
          profile={{
            id: profile.id,
            name: profile.name,
            accent: profile.accent,
            hasPin: profile.hasPin,
            isPrimary: profile.isPrimary,
          }}
          onClose={() => setPinOpen(false)}
          onSelected={() => setPinOpen(false)}
        />
      )}
    </div>
  );
}
