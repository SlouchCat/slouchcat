import type { Profile } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState, type ReactNode, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { Chip, Notice, Panel, ProfileAvatar, Skeleton, Text } from '@/ui';
import {
  ActionMenu,
  AriaButton,
  ConfirmDialog,
  FormDialog,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
} from '@/ui/aria';
import { limitParts } from '../shared/LimitFields';
import { describeLibraries, useLibraryOptions } from '../shared/useLibraryOptions';
import { PinSetDialog } from './PinSetDialog';
import { ProfileForm } from './ProfileForm';
import { manageProfilesQuery } from './queries';
import { WatchReport } from './WatchReport';
import styles from './ProfileList.module.css';

type Editing = Profile | 'new' | null;

interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  tone: 'primary' | 'danger';
  onConfirm: () => void;
}

interface ProfileListProps {
  /**
   * The page's h1: it takes the focus when a deleted profile took the focused element with it
   * (ALL-2).
   */
  titleRef: RefObject<HTMLHeadingElement | null>;
  /**
   * The names are headings (h2) where the list is the page (`/settings/profiles`); inside the
   * own account of «Konten» they stand below an h3 and are plain text (S-3).
   */
  nameHeadings?: boolean;
  /** The page holds its queries (a dialog or unsaved input of its own): the list holds too. */
  hold?: boolean;
  /** The list's own dialogs are open (or no longer): the page holds its queries with them. */
  onHoldChange?: (held: boolean) => void;
  /** A profile was created, changed or deleted (the account list shows the names). */
  onChanged?: () => void;
  /** Lays out the add button and the list: in the page header, or in a block of a page. */
  children: (parts: { add: ReactNode; list: ReactNode }) => ReactNode;
}

/**
 * The profiles of the own account (K-2, PRO-1 to PRO-3), shared by `/settings/profiles` and the
 * own account in «Konten» (concept 12, KP-1): one card per profile with its settings, «Bearbeiten»
 * and «Schauzeit anzeigen» as buttons, the PIN and the deletion in «Weitere Aktionen» (S-6).
 * Create and edit in the dialog; the confirmations are the list's own, so a card that goes while
 * one is open does not take it along. The profile endpoints serve the own account only
 * (`x-visibility: account`), so this list never shows another account's profiles.
 */
export function ProfileList({
  titleRef,
  nameHeadings = true,
  hold = false,
  onHoldChange,
  onChanged,
  children,
}: ProfileListProps) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<Editing>(null);
  const [pinFor, setPinFor] = useState<Profile | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const ownHeld = editing !== null || pinFor !== null || confirmOpen;
  const held = hold || ownHeld;
  useEffect(() => {
    onHoldChange?.(ownHeld);
  }, [onHoldChange, ownHeld]);
  // A list that goes (its row folded) holds nothing any more.
  useEffect(() => () => onHoldChange?.(false), [onHoldChange]);
  const profiles = useQuery({ ...manageProfilesQuery, refetchOnWindowFocus: !held });
  const libraries = useLibraryOptions();
  // Profiles whose watch-time report is open (K-5); several at once.
  const [reports, setReports] = useState<ReadonlySet<number>>(() => new Set());
  const toggleReport = (id: number) =>
    setReports((open) => {
      const next = new Set(open);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['profiles'] }),
      queryClient.invalidateQueries({ queryKey: ['auth'] }),
    ]);
  };
  const remove = useMutation({
    mutationFn: (id: number) => request(api.DELETE('/profiles/{id}', { params: { path: { id } } })),
    onSuccess: async () => {
      await invalidate();
      onChanged?.();
      // The card and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterRender();
      titleRef.current?.focus();
    },
  });
  const removePin = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/profiles/{id}/pin', { params: { path: { id } } })),
    onSuccess: invalidate,
  });
  const actionError = remove.error ?? removePin.error;

  const ask = (next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  };
  const onConfirmOpenChange = (open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The menu button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  };
  const onAction = (profile: Profile, action: string) => {
    if (action === 'pin') setPinFor(profile);
    if (action === 'removePin') {
      ask({
        title: t('profiles.settings.pin.remove.title', { name: profile.name }),
        body: t('profiles.settings.pin.remove.body'),
        confirmLabel: t('profiles.settings.pin.remove.confirm'),
        tone: 'primary',
        onConfirm: () => removePin.mutate(profile.id),
      });
    }
    if (action === 'delete') {
      ask({
        title: t('profiles.settings.delete.title', { name: profile.name }),
        body: t('profiles.settings.delete.body'),
        confirmLabel: t('profiles.settings.delete.confirm'),
        tone: 'danger',
        onConfirm: () => remove.mutate(profile.id),
      });
    }
  };

  const list = profiles.data ?? [];
  const closeForm = () => setEditing(null);
  const add = (
    <AriaButton
      variant="primary"
      icon="plus"
      isDisabled={!profiles.isSuccess || !libraries.isReady}
      onPress={() => setEditing('new')}
    >
      {t('profiles.settings.add')}
    </AriaButton>
  );

  const content = (
    <>
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
          const name = nameHeadings ? (
            <h2 id={headingId} className={styles.name}>
              {profile.name}
            </h2>
          ) : (
            <span id={headingId} className={styles.name}>
              {profile.name}
            </span>
          );
          return (
            <li key={profile.id}>
              <Panel as="article" aria-labelledby={headingId}>
                <div className={styles.cardHead}>
                  <ProfileAvatar name={profile.name} accent={profile.accent} />
                  <div className={styles.cardTitle}>
                    {name}
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
                      {profile.metadataLanguage && (
                        <Chip tone="outline">
                          {t('profiles.settings.metadataLanguageChip', {
                            language:
                              new Intl.DisplayNames([profile.language], { type: 'language' }).of(
                                profile.metadataLanguage,
                              ) ?? profile.metadataLanguage,
                          })}
                        </Chip>
                      )}
                      {profile.hasPin && (
                        <Chip tone="info" icon="lock">
                          {t('profiles.settings.hasPin')}
                        </Chip>
                      )}
                      {limitParts(t, profile.effectiveLimits).map((part) => (
                        <Chip key={part} tone="outline">
                          {part}
                        </Chip>
                      ))}
                    </div>
                  </div>
                </div>
                <Text tone="secondary" variant="caption">
                  {describeLibraries(t, profile, libraries.options)}
                </Text>
                <div className={styles.actions}>
                  <AriaButton
                    isDisabled={!libraries.isReady}
                    aria-describedby={headingId}
                    onPress={() => setEditing(profile)}
                  >
                    {t('profiles.settings.edit')}
                  </AriaButton>
                  <AriaButton
                    variant="ghost"
                    aria-expanded={reports.has(profile.id)}
                    aria-describedby={headingId}
                    onPress={() => toggleReport(profile.id)}
                  >
                    {reports.has(profile.id)
                      ? t('profiles.settings.report.hide')
                      : t('profiles.settings.report.show')}
                  </AriaButton>
                  <ActionMenu
                    rowName={profile.name}
                    onAction={(action) => onAction(profile, action)}
                    items={[
                      {
                        id: 'pin',
                        label: profile.hasPin
                          ? t('profiles.settings.pin.change')
                          : t('profiles.settings.pin.set'),
                        icon: 'lock',
                      },
                      ...(profile.hasPin
                        ? [{ id: 'removePin', label: t('profiles.settings.pin.remove.action') }]
                        : []),
                      // The primary profile cannot be deleted.
                      ...(profile.isPrimary
                        ? []
                        : [
                            {
                              id: 'delete',
                              label: t('profiles.settings.delete.action'),
                              icon: 'trash' as const,
                              danger: true,
                            },
                          ]),
                    ]}
                  />
                </div>
                {reports.has(profile.id) && <WatchReport profile={profile} />}
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
            onSaved={onChanged}
            onClose={closeForm}
          />
        </FormDialog>
      )}
      {pinFor && <PinSetDialog profile={pinFor} onClose={() => setPinFor(null)} />}
      {confirmation && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={confirmation.title}
          body={confirmation.body}
          confirmLabel={confirmation.confirmLabel}
          cancelLabel={t('common.cancel')}
          tone={confirmation.tone}
          onConfirm={confirmation.onConfirm}
        />
      )}
    </>
  );

  return children({ add, list: content });
}
