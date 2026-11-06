import type { Channel, ChannelSetting, ChannelSettings } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { Navigate } from '@tanstack/react-router';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Notice, Skeleton, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  afterDialogsClosed,
  focusTitleIfLost,
  useDrafts,
  useUnsavedChanges,
} from '@/ui/aria';
import { ChannelPanel } from './channels/ChannelPanel';
import { ChannelSummary } from './channels/ChannelSummary';
import {
  CHANNEL_ICON,
  CHANNEL_ORDER,
  channelDraftOf,
  LINK_POLL_MS,
  stateOf,
  type Confirmation,
} from './channels/shared';
import { channelsQuery } from './queries';
import styles from './channels/channels.module.css';

const linkOpen = (data: ChannelSettings | undefined) =>
  data?.channels.some(
    (channel) =>
      channel.channel === 'telegram' && !channel.telegram?.linked && channel.telegram?.linkCode,
  ) ?? false;

/** E-mail, Telegram, Web Push, whatever order the server lists them in. */
const ordered = (channels: readonly ChannelSetting[]) =>
  [...channels].sort((a, b) => CHANNEL_ORDER.indexOf(a.channel) - CHANNEL_ORDER.indexOf(b.channel));

/**
 * Notification channels of the account (A-6, E-8; concept 12, mockup Ben-A): e-mail, Telegram and
 * Web Push, each a block that folds away by its band, with its switch, its own part (address,
 * link, devices) and its events, saved and tested per channel. Unsaved input lives in the page
 * (useDrafts), so folding a block keeps it and leaving the page with it asks first (S-13, S-17);
 * the queries hold meanwhile and while the confirmation is open. Any profile of the account sees
 * the same settings; the in-app inbox is always on and not listed. The server decides what may be
 * chosen (`errors` for the main account only). Only the account's primary profile changes them:
 * another profile sees them without controls (BEN-5). Every account has this page, so it keeps
 * today's density (S-1). Inside the administration (`/admin/notifications`, concept 15, P-4) the
 * same page stands under the admin sidebar in the compact density, and an administrator gets a
 * link to the server's channels at the top.
 */
export function ChannelSettingsPage({ inAdmin = false }: { inAdmin?: boolean } = {}) {
  const { t } = useTranslation();
  useDocumentTitle(t('settings.notifications.title'));
  const session = useActiveSession();
  const isAdmin = session.account.role === 'admin';
  const isPrimary = session.profile.isPrimary;
  const accountEmail = session.account.email;
  const titleRef = useRef<HTMLHeadingElement>(null);

  // The drafts need what the server holds before the query knows whether to hold for them: a
  // passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...channelsQuery, enabled: false }).data;
  const originals = useMemo(
    () =>
      new Map(
        (cached?.channels ?? []).map(
          (setting) => [setting.channel, channelDraftOf(setting, accountEmail)] as const,
        ),
      ),
    [cached, accountEmail],
  );
  const drafts = useDrafts(useCallback((id: string) => originals.get(id as Channel), [originals]));
  const unsaved = useUnsavedChanges(drafts.dirty);

  // The page's one confirmation: a block that changes while it is open cannot take it along.
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const hold = confirmOpen || unsaved.pausePolling;
  // Only the primary profile opens a link code; the poll for it runs while nothing holds.
  const polls = isPrimary && !hold;
  const channels = useQuery({
    ...channelsQuery,
    refetchInterval: (query) => (polls && linkOpen(query.state.data) ? LINK_POLL_MS : false),
    refetchOnWindowFocus: !hold,
  });

  // Folded blocks; the drafts stay in the page, so folding loses nothing.
  const [folded, setFolded] = useState<ReadonlySet<Channel>>(() => new Set());
  const setExpanded = (channel: Channel, expanded: boolean) =>
    setFolded((current) => {
      const next = new Set(current);
      if (expanded) next.delete(channel);
      else next.add(channel);
      return next;
    });

  /** The focus fell to the body (its button went with a change): the h1 takes it. */
  const refocusTitle = useCallback(() => focusTitleIfLost(titleRef), []);
  const focusTitle = useCallback(() => titleRef.current?.focus(), []);
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = (open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(refocusTitle);
  };

  /** A short line for the band of a folded block: what the block holds. */
  const summary = (setting: ChannelSetting): string | undefined => {
    if (!setting.available) return undefined;
    // The address as typed while it is unsaved: the band shows what «Speichern» would store.
    if (setting.channel === 'email') {
      return (
        (isPrimary ? drafts.valueOf('email')?.address : setting.email?.address) ?? accountEmail
      );
    }
    if (setting.channel === 'telegram') {
      if (setting.telegram?.linked) return t('settings.notifications.telegram.linked');
      return setting.telegram?.linkCode
        ? t('settings.notifications.telegram.codeOpen')
        : t('settings.notifications.telegram.notLinked');
    }
    return t('settings.notifications.push.count', {
      count: setting.push?.subscriptions.length ?? 0,
    });
  };

  return (
    <AdminPage
      title={t('settings.notifications.title')}
      help={t('settings.notifications.intro')}
      titleRef={titleRef}
      liveSeconds={polls && linkOpen(channels.data) ? LINK_POLL_MS / 1000 : undefined}
      actions={
        inAdmin && isAdmin ? (
          <TextLink href={paths.adminIntegrations}>
            {t('settings.notifications.serverChannels')}
          </TextLink>
        ) : undefined
      }
    >
      {!isPrimary && (
        <Notice
          title={t('settings.notifications.onlyPrimary.title')}
          actions={
            <ButtonLink href={paths.profiles} variant="secondary">
              {t('settings.notifications.onlyPrimary.action')}
            </ButtonLink>
          }
        >
          {t('settings.notifications.onlyPrimary.body')}
        </Notice>
      )}
      {channels.isPending && <Skeleton shape="block" />}
      {channels.isError && !channels.data && (
        <Notice
          tone="danger"
          title={problemMessage(t, channels.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void channels.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {channels.data && (
        <ul className={styles.list} aria-label={t('settings.notifications.list')}>
          {ordered(channels.data.channels).map((setting) => {
            const { channel } = setting;
            const state = stateOf(setting);
            const expanded = !folded.has(channel);
            const shown = expanded ? undefined : summary(setting);
            const dirty = isPrimary && drafts.isDirty(channel);
            return (
              <li key={channel}>
                <BandPanel
                  collapsible
                  isExpanded={expanded}
                  onExpandedChange={(open) => setExpanded(channel, open)}
                  icon={CHANNEL_ICON[channel]}
                  title={t(`settings.notifications.channel.${channel}`)}
                  meta={
                    <>
                      {dirty && <Chip tone="warning">{t('settings.notifications.unsaved')}</Chip>}
                      {shown && <span className={styles.summary}>{shown}</span>}
                      <Chip tone={state === 'on' ? 'success' : 'outline'}>
                        {t(`settings.notifications.state.${state}`)}
                      </Chip>
                    </>
                  }
                >
                  {isPrimary ? (
                    <ChannelPanel
                      setting={setting}
                      isAdmin={isAdmin}
                      values={drafts.valueOf(channel) ?? channelDraftOf(setting, accountEmail)}
                      dirty={drafts.isDirty(channel)}
                      onChange={(next) => drafts.set(channel, next)}
                      // The draft is spent when it still says what was sent; what was changed
                      // while the save ran stays a draft (decision 18, addendum 3).
                      onSaved={(sent, saved) =>
                        drafts.set(channel, (current) =>
                          JSON.stringify(current) === JSON.stringify(sent)
                            ? channelDraftOf(saved, accountEmail)
                            : current,
                        )
                      }
                      ask={ask}
                      polling={polls}
                      focusTitleIfLost={refocusTitle}
                      focusTitle={focusTitle}
                    />
                  ) : (
                    <ChannelSummary setting={setting} />
                  )}
                </BandPanel>
              </li>
            );
          })}
        </ul>
      )}
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
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}

/**
 * The same page as a child of the administration's layout (route `/admin/notifications`). Anyone
 * but an administrator's primary profile has no administration around it (AdminLayout) and goes to
 * the page under the settings instead.
 */
export function AdminChannelSettingsPage() {
  const session = useActiveSession();
  if (session.account.role !== 'admin' || !session.profile.isPrimary) {
    return <Navigate to={paths.notificationSettings} replace />;
  }
  return <ChannelSettingsPage inAdmin />;
}
