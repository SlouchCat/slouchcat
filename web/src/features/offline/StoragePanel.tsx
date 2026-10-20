import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatBytes } from '@/lib/format';
import { Notice, Text } from '@/ui';
import { StorageMeter, type StoragePart } from '@/ui/aria';
import { storageInfo, type StorageInfo } from './storage';
import { isAppleStorage } from './support';
import styles from './OfflinePages.module.css';

interface StoragePanelProps {
  /** Bytes the downloads of this device take (all profiles: the storage is the device's). */
  downloadBytes: number;
}

/**
 * The storage of this device (O-7): what the browser allows and uses, the share of the downloads,
 * whether the browser keeps it for good, and the hint for Apple devices and Safari.
 */
export function StoragePanel({ downloadBytes }: StoragePanelProps) {
  const { t, i18n } = useTranslation();
  const [info, setInfo] = useState<StorageInfo | null>(null);
  // Ask again whenever the downloads grew or shrank.
  useEffect(() => {
    let active = true;
    void storageInfo().then((next) => {
      if (active) setInfo(next);
    });
    return () => {
      active = false;
    };
  }, [downloadBytes]);

  const bytes = (value: number) => formatBytes(value, i18n.language);
  const usage = info?.usage ?? null;
  const quota = info?.quota ?? null;
  const parts: StoragePart[] = [
    {
      key: 'downloads',
      value: downloadBytes,
      label: t('offline.storage.downloads', { size: bytes(downloadBytes) }),
      tone: 'accent',
    },
  ];
  if (usage !== null) {
    const other = Math.max(0, usage - downloadBytes);
    parts.push({
      key: 'other',
      value: other,
      label: t('offline.storage.other', { size: bytes(other) }),
      tone: 'neutral',
    });
  }

  return (
    <section
      className={styles.section}
      aria-labelledby="storage-heading"
      data-usage-bytes={usage ?? undefined}
      data-quota-bytes={quota ?? undefined}
      data-download-bytes={downloadBytes}
    >
      <h2 id="storage-heading" className={styles.sectionTitle}>
        {t('offline.storage.title')}
      </h2>
      {quota !== null && usage !== null ? (
        <StorageMeter
          label={t('offline.storage.meter')}
          valueText={t('offline.storage.used', { used: bytes(usage), quota: bytes(quota) })}
          total={quota}
          parts={parts}
          caption={t('offline.storage.free', {
            free: bytes(Math.max(0, quota - usage)),
            quota: bytes(quota),
          })}
        />
      ) : (
        info && (
          <Text tone="secondary">
            {t('offline.storage.unknown', { size: bytes(downloadBytes) })}
          </Text>
        )
      )}
      {info?.persisted !== null && info?.persisted !== undefined && (
        <Text tone="secondary">
          {info.persisted ? t('offline.storage.persisted') : t('offline.storage.notPersisted')}
        </Text>
      )}
      {isAppleStorage() && (
        <Notice tone="warning" title={t('offline.storage.apple.title')}>
          {t('offline.storage.apple.body')}
        </Notice>
      )}
    </section>
  );
}
