import type { ProfileSummary } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { isReturnTarget } from '@/app/returnTarget';
import { sessionQuery, useLogout, useSelectProfile } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Button,
  Logo,
  Notice,
  ProfileTile,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  Text,
} from '@/ui';
import { PinDialog } from './PinDialog';
import { myProfilesQuery } from './queries';
import styles from './ProfilePickerPage.module.css';

const route = getRouteApi('/_authed/profiles');

const tileKey = (profile: ProfileSummary) => `profile-${profile.id}`;

/** The small line under a name: "Primary profile", "active", or both. */
function captionOf(t: TFunction, profile: ProfileSummary, isActive: boolean): string | undefined {
  if (profile.isPrimary && isActive) return t('profiles.picker.primaryActive');
  if (profile.isPrimary) return t('profiles.picker.primary');
  return isActive ? t('profiles.picker.active') : undefined;
}

/** "Wer schaut?" (K-4): every profile of the account as a tile; arrow keys move, Enter picks. */
export function ProfilePickerPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('profiles.picker.title'));
  const router = useRouter();
  const { next } = route.useSearch();
  const session = useQuery(sessionQuery).data;
  const profiles = useQuery(myProfilesQuery);
  const select = useSelectProfile();
  const logout = useLogout();
  const [pinFor, setPinFor] = useState<ProfileSummary | null>(null);

  const activeId = session?.profile?.id;
  // `next` comes from the URL; the router's own search validation does not reach this far (parent
  // routes pass the raw value on), so it is checked again here.
  const done = () => void router.navigate({ href: isReturnTarget(next) ? next : paths.home });

  const pick = (profile: ProfileSummary) => {
    if (select.isPending) return;
    if (profile.id === activeId) {
      done();
    } else if (profile.hasPin) {
      setPinFor(profile);
    } else {
      void select.select({ id: profile.id }).then((outcome) => {
        if (outcome.ok) done();
        // The PIN was set after the list was loaded: ask for it now.
        else if (isApiError(outcome.error, 403) && outcome.error.code === 'pin_required') {
          setPinFor(profile);
        }
      });
    }
  };

  const list = profiles.data ?? [];
  const initial = list.find((profile) => profile.id === activeId) ?? list[0];
  const manageProfiles = session?.profile?.isPrimary === true;
  const pickError = select.error && !pinFor ? select.error : null;

  return (
    <SpatialPage defaultFocusKey={initial ? tileKey(initial) : 'actions'}>
      <main className={styles.page}>
        <div className={styles.logo}>
          <Logo name={t('app.name')} />
        </div>
        <div className={styles.content}>
          <div className={styles.head}>
            <Text as="h1" variant="display">
              {t('profiles.picker.title')}
            </Text>
            <Text tone="secondary">{t('profiles.picker.intro')}</Text>
          </div>

          {profiles.isPending && (
            <div className={styles.tiles} aria-busy="true">
              {[0, 1, 2].map((index) => (
                <div key={index} className={styles.skeletonTile}>
                  <Skeleton shape="avatar" />
                  <Skeleton shape="line" />
                </div>
              ))}
            </div>
          )}
          {profiles.isError && (
            <Notice
              tone="danger"
              title={t('profiles.picker.loadFailed')}
              actions={
                <Button icon="refresh" onPress={() => void profiles.refetch()}>
                  {t('common.retry')}
                </Button>
              }
            >
              {problemMessage(t, profiles.error)}
            </Notice>
          )}
          {pickError && <Notice tone="danger">{problemMessage(t, pickError)}</Notice>}

          {list.length > 0 && (
            <SpatialGroup
              as="ul"
              focusKey="profiles"
              className={styles.tiles}
              aria-label={t('profiles.picker.list')}
            >
              {list.map((profile) => (
                <li key={profile.id}>
                  <ProfileTile
                    focusKey={tileKey(profile)}
                    name={profile.name}
                    accent={profile.accent}
                    hasPin={profile.hasPin}
                    isActive={profile.id === activeId}
                    autoFocus={profile.id === initial?.id}
                    lockedLabel={t('profiles.picker.locked')}
                    caption={captionOf(t, profile, profile.id === activeId)}
                    onPress={() => pick(profile)}
                  />
                </li>
              ))}
            </SpatialGroup>
          )}

          <SpatialGroup focusKey="actions" className={styles.actions}>
            {manageProfiles && (
              <SpatialButtonLink href={paths.profileSettings} variant="secondary">
                {t('profiles.picker.manage')}
              </SpatialButtonLink>
            )}
            <SpatialButton variant="ghost" icon="logout" onPress={() => logout.mutate()}>
              {t('nav.logout')}
            </SpatialButton>
          </SpatialGroup>
        </div>
      </main>
      {pinFor && (
        <PinDialog
          key={pinFor.id}
          profile={pinFor}
          onClose={() => setPinFor(null)}
          onSelected={done}
        />
      )}
    </SpatialPage>
  );
}
