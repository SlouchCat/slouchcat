import type { Profile } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Notice, Page, Panel, ProfileAvatar, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, FormDialog } from '@/ui/aria';
import { describeLibraries, useLibraryOptions } from '../shared/useLibraryOptions';
import { PinSetDialog } from './PinSetDialog';
import { ProfileForm } from './ProfileForm';
import { manageProfilesQuery } from './queries';
import styles from './ProfileSettingsPage.module.css';

type Editing = Profile | 'new' | null;

/** Profile management (K-2) of the account's primary profile: list, create, edit, PIN, delete. */
export function ProfileSettingsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('profiles.settings.title'));
  const session = useActiveSession();
  const queryClient = useQueryClient();
  const isPrimary = session.profile.isPrimary;
  const profiles = useQuery({ ...manageProfilesQuery, enabled: isPrimary });
  const libraries = useLibraryOptions(isPrimary);
  const [editing, setEditing] = useState<Editing>(null);
  const [pinFor, setPinFor] = useState<Profile | null>(null);

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['profiles'] }),
      queryClient.invalidateQueries({ queryKey: ['auth'] }),
    ]);
  };
  const remove = useMutation({
    mutationFn: (id: number) => request(api.DELETE('/profiles/{id}', { params: { path: { id } } })),
    onSuccess: invalidate,
  });
  const removePin = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/profiles/{id}/pin', { params: { path: { id } } })),
    onSuccess: invalidate,
  });
  const actionError = remove.error ?? removePin.error;

  if (!isPrimary) {
    return (
      <Page>
        <Text as="h1" variant="display">
          {t('profiles.settings.title')}
        </Text>
        <div className={styles.gap}>
          <Notice
            title={t('profiles.settings.onlyPrimary.title')}
            actions={
              <ButtonLink href={paths.profiles} variant="secondary">
                {t('profiles.settings.onlyPrimary.action')}
              </ButtonLink>
            }
          >
            {t('profiles.settings.onlyPrimary.body')}
          </Notice>
        </div>
      </Page>
    );
  }

  const list = profiles.data ?? [];
  const closeForm = () => setEditing(null);

  return (
    <Page>
      <div className={styles.header}>
        <div className={styles.headerText}>
          <Text as="h1" variant="display">
            {t('profiles.settings.title')}
          </Text>
          <Text tone="secondary">{t('profiles.settings.intro')}</Text>
        </div>
        <AriaButton
          variant="primary"
          icon="plus"
          isDisabled={!profiles.isSuccess || !libraries.isReady}
          onPress={() => setEditing('new')}
        >
          {t('profiles.settings.add')}
        </AriaButton>
      </div>

      {actionError && <Notice tone="danger">{problemMessage(t, actionError)}</Notice>}
      {libraries.isError && (
        <Notice
          tone="danger"
          title={t('libraries.loadFailed')}
          actions={
            <AriaButton icon="refresh" onPress={libraries.retry}>
              {t('common.retry')}
            </AriaButton>
          }
        >
          {problemMessage(t, libraries.error)}
        </Notice>
      )}
      {profiles.isPending && <Skeleton shape="block" />}
      {profiles.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, profiles.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void profiles.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}

      <ul className={styles.list} aria-label={t('profiles.settings.list')}>
        {list.map((profile) => {
          const headingId = `profile-${profile.id}`;
          return (
            <li key={profile.id}>
              <Panel as="article" aria-labelledby={headingId}>
                <div className={styles.cardHead}>
                  <ProfileAvatar name={profile.name} accent={profile.accent} />
                  <div className={styles.cardTitle}>
                    <h2 id={headingId} className={styles.name}>
                      {profile.name}
                    </h2>
                    <div className={styles.chips}>
                      {profile.isPrimary && (
                        <Chip tone="info">{t('profiles.settings.primary')}</Chip>
                      )}
                      <Chip tone="outline">
                        {profile.maxRating > session.account.maxRating
                          ? t('profiles.settings.ratingAboveAccount', {
                              age: profile.maxRating,
                              max: session.account.maxRating,
                            })
                          : t('rating.fsk', { age: profile.maxRating })}
                      </Chip>
                      <Chip tone="info">{t(`language.${profile.language}`)}</Chip>
                      {profile.hasPin && (
                        <Chip tone="info" icon="lock">
                          {t('profiles.settings.hasPin')}
                        </Chip>
                      )}
                    </div>
                  </div>
                </div>
                <Text tone="secondary" variant="caption">
                  {describeLibraries(t, profile, libraries.options)}
                </Text>
                <div className={styles.actions}>
                  <AriaButton isDisabled={!libraries.isReady} onPress={() => setEditing(profile)}>
                    {t('profiles.settings.edit')}
                  </AriaButton>
                  <AriaButton variant="ghost" onPress={() => setPinFor(profile)}>
                    {profile.hasPin
                      ? t('profiles.settings.pin.change')
                      : t('profiles.settings.pin.set')}
                  </AriaButton>
                  {profile.hasPin && (
                    <ConfirmDialog
                      trigger={
                        <AriaButton variant="ghost">
                          {t('profiles.settings.pin.remove.action')}
                        </AriaButton>
                      }
                      title={t('profiles.settings.pin.remove.title', { name: profile.name })}
                      body={t('profiles.settings.pin.remove.body')}
                      confirmLabel={t('profiles.settings.pin.remove.confirm')}
                      cancelLabel={t('common.cancel')}
                      onConfirm={() => removePin.mutate(profile.id)}
                    />
                  )}
                  {!profile.isPrimary && (
                    <ConfirmDialog
                      trigger={
                        <AriaButton variant="ghost">
                          {t('profiles.settings.delete.action')}
                        </AriaButton>
                      }
                      title={t('profiles.settings.delete.title', { name: profile.name })}
                      body={t('profiles.settings.delete.body')}
                      confirmLabel={t('profiles.settings.delete.confirm')}
                      cancelLabel={t('common.cancel')}
                      tone="danger"
                      onConfirm={() => remove.mutate(profile.id)}
                    />
                  )}
                </div>
              </Panel>
            </li>
          );
        })}
      </ul>

      {editing !== null && (
        <FormDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) closeForm();
          }}
          title={
            editing === 'new'
              ? t('profiles.settings.form.createTitle')
              : t('profiles.settings.form.editTitle')
          }
        >
          <ProfileForm
            profile={editing === 'new' ? undefined : editing}
            profiles={list}
            session={session}
            libraries={libraries.options}
            onClose={closeForm}
          />
        </FormDialog>
      )}
      {pinFor && <PinSetDialog profile={pinFor} onClose={() => setPinFor(null)} />}
    </Page>
  );
}
