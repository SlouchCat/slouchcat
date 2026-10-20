import type { TFunction } from 'i18next';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { episodeCode } from '@/features/catalog/labels';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useOnline } from '@/lib/useOnline';
import {
  ButtonLink,
  Chip,
  Logo,
  Notice,
  PosterCard,
  ProfileTile,
  setSpatialFocus,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  Text,
} from '@/ui';
import { kickOfflineEngine } from './engine';
import { downloadStatus, downloadTitle } from './labels';
import { OfflinePinDialog } from './OfflinePinDialog';
import { chooseOfflineProfile, leaveOfflineProfile, useOfflineChoice } from './offlineSession';
import { pinRequirement } from './pin';
import { offlineSupported } from './support';
import { downloadsForProfile, offlineProfiles } from './sync';
import type { StoredDownload, StoredProfile } from './types';
import { useStoredDownloads, useStoredProfiles } from './useOfflineData';
import { usePosterUrl } from './usePosterUrl';
import styles from './OfflinePages.module.css';

const tileKey = (profile: StoredProfile) => `offline-profile-${profile.id}`;
const cardKey = (download: StoredDownload) => `offline-download-${download.id}`;

/** Line under a card: "S01E02 · Series" for an episode, the year of a film. */
function captionOf(t: TFunction, download: StoredDownload): string | undefined {
  const item = download.item;
  if (!item) return undefined;
  if (item.kind === 'episode') {
    const code = episodeCode(t, item);
    return code && item.seriesTitle
      ? t('item.episodeOf', { code, series: item.seriesTitle })
      : code;
  }
  return item.year ? String(item.year) : undefined;
}

/**
 * The offline library (/offline, O-4, C-1): without a session and without network. First the
 * profiles that may play a download on this device (the server's `allowedProfileIds`), with the
 * PIN checked locally (AN-19); then that profile's downloads.
 */
export function OfflineLibraryPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('offline.library.title'));
  const online = useOnline();
  const supported = offlineSupported();
  // Back online: the engine asks the server what changed (revocations, new downloads).
  useEffect(() => {
    if (online && supported) kickOfflineEngine();
  }, [online, supported]);

  return (
    <main className={styles.library}>
      <div className={styles.libraryTop}>
        <Logo name={t('app.name')} />
        {online ? (
          <ButtonLink href={paths.home} variant="secondary">
            {t('offline.library.toApp')}
          </ButtonLink>
        ) : (
          <Chip tone="info">{t('offline.library.offline')}</Chip>
        )}
      </div>
      {supported ? (
        <LibraryContent />
      ) : (
        <div className={styles.libraryBody}>
          <Text as="h1" variant="display">
            {t('offline.library.title')}
          </Text>
          <Notice tone="warning" title={t('offline.downloads.unsupported.title')}>
            {t('offline.downloads.unsupported.body')}
          </Notice>
        </div>
      )}
    </main>
  );
}

function LibraryContent() {
  const downloads = useStoredDownloads();
  const profiles = useStoredProfiles();
  const choice = useOfflineChoice();
  if (!downloads || !profiles) {
    return (
      <div className={styles.libraryBody} aria-busy="true">
        <Skeleton shape="block" />
      </div>
    );
  }
  const candidates = offlineProfiles(profiles, downloads);
  const chosen = choice
    ? candidates.find(
        (profile) => profile.id === choice.profileId && profile.accountId === choice.accountId,
      )
    : undefined;
  return chosen ? (
    <ProfileDownloads
      key={chosen.id}
      profile={chosen}
      downloads={downloadsForProfile(downloads, chosen.accountId, chosen.id)}
    />
  ) : (
    <OfflinePicker profiles={candidates} />
  );
}

function OfflinePicker({ profiles }: { profiles: StoredProfile[] }) {
  const { t } = useTranslation();
  const [pinFor, setPinFor] = useState<StoredProfile | null>(null);
  const open = (profile: StoredProfile) =>
    chooseOfflineProfile({ accountId: profile.accountId, profileId: profile.id });
  const pick = (profile: StoredProfile) => {
    if (pinRequirement(profile) === 'none') open(profile);
    else setPinFor(profile);
  };
  const first = profiles[0];

  return (
    <SpatialPage defaultFocusKey={first ? tileKey(first) : 'offline-actions'}>
      <div className={styles.libraryBody}>
        <div className={styles.libraryHead}>
          <Text as="h1" variant="display">
            {t('offline.library.title')}
          </Text>
          <Text tone="secondary">
            {profiles.length > 0 ? t('offline.library.intro') : t('offline.library.empty.body')}
          </Text>
        </div>
        {profiles.length === 0 ? (
          <Notice title={t('offline.library.empty.title')}>
            {t('offline.library.empty.hint')}
          </Notice>
        ) : (
          <SpatialGroup
            as="ul"
            focusKey="offline-profiles"
            className={styles.tiles}
            aria-label={t('offline.library.profiles')}
          >
            {profiles.map((profile) => (
              <li key={profile.id}>
                <ProfileTile
                  focusKey={tileKey(profile)}
                  name={profile.snapshot.name}
                  accent={profile.snapshot.accent}
                  hasPin={pinRequirement(profile) !== 'none'}
                  isActive={false}
                  autoFocus={profile.id === first?.id}
                  lockedLabel={t('profiles.picker.locked')}
                  caption={profile.snapshot.isPrimary ? t('profiles.picker.primary') : undefined}
                  onPress={() => pick(profile)}
                />
              </li>
            ))}
          </SpatialGroup>
        )}
      </div>
      {pinFor && (
        <OfflinePinDialog
          key={pinFor.id}
          profile={pinFor}
          onClose={() => setPinFor(null)}
          onOpened={() => {
            setPinFor(null);
            open(pinFor);
          }}
        />
      )}
    </SpatialPage>
  );
}

function DownloadCard({ download, focusKey }: { download: StoredDownload; focusKey: string }) {
  const { t } = useTranslation();
  const poster = usePosterUrl(download);
  return (
    <PosterCard
      focusKey={focusKey}
      href={paths.offlinePlay(download.id)}
      title={downloadTitle(t, download.item)}
      caption={captionOf(t, download)}
      posterUrl={poster}
      accent={download.item?.accent}
      eager
    />
  );
}

function ProfileDownloads({
  profile,
  downloads,
}: {
  profile: StoredProfile;
  downloads: StoredDownload[];
}) {
  const { t } = useTranslation();
  useBackKeys(leaveOfflineProfile);
  const ready = downloads.filter((download) => download.complete);
  const pending = downloads.filter((download) => !download.complete);
  const first = ready[0];
  // The profile was just opened (its tile is gone): the focus moves to the first title.
  const [initialKey] = useState(() => (first ? cardKey(first) : 'offline-switch'));
  useEffect(() => setSpatialFocus(initialKey), [initialKey]);

  return (
    <SpatialPage defaultFocusKey={first ? cardKey(first) : 'offline-switch'}>
      <div className={styles.libraryBody}>
        <div className={styles.libraryHead}>
          <Text as="h1" variant="display">
            {t('offline.library.downloadsOf', { name: profile.snapshot.name })}
          </Text>
          <SpatialGroup focusKey="offline-actions" className={styles.libraryActions}>
            <SpatialButton
              focusKey="offline-switch"
              variant="ghost"
              icon="back"
              onPress={leaveOfflineProfile}
            >
              {t('offline.library.switchProfile')}
            </SpatialButton>
          </SpatialGroup>
        </div>
        {ready.length === 0 ? (
          <Notice>{t('offline.library.noneReady')}</Notice>
        ) : (
          <SpatialGroup
            as="ul"
            focusKey="offline-downloads"
            className={styles.grid}
            aria-label={t('offline.library.list')}
          >
            {ready.map((download) => (
              <li key={download.id}>
                <DownloadCard download={download} focusKey={cardKey(download)} />
              </li>
            ))}
          </SpatialGroup>
        )}
        {pending.length > 0 && (
          <section className={styles.section} aria-labelledby="offline-pending-heading">
            <h2 id="offline-pending-heading" className={styles.sectionTitle}>
              {t('offline.library.pending')}
            </h2>
            <ul className={styles.pendingList}>
              {pending.map((download) => {
                const status = downloadStatus(t, download);
                return (
                  <li key={download.id} className={styles.pendingItem}>
                    <span>{downloadTitle(t, download.item)}</span>
                    <Chip tone={status.tone}>{status.text}</Chip>
                  </li>
                );
              })}
            </ul>
          </section>
        )}
      </div>
    </SpatialPage>
  );
}
