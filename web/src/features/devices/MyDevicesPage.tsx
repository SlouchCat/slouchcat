import type { MyDevice } from '@slouchcat/api-client';
import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession, useLogout } from '@/app/session';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Panel, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  ConfirmDialog,
  SectionHeader,
  StatusRegion,
  afterDialogsClosed,
  focusTitleIfLost,
  useStatusMessage,
} from '@/ui/aria';
import styles from './MyDevicesPage.module.css';

/** The devices of the own account («Meine Geräte», concept 27 4.5). */
export const myDevicesQuery = queryOptions({
  queryKey: ['account', 'devices'],
  queryFn: ({ signal }): Promise<MyDevice[]> => request(api.GET('/account/devices', { signal })),
});

const signOut = (deviceId: string) =>
  request(api.DELETE('/account/devices/{deviceId}', { params: { path: { deviceId } } }));

/**
 * «Meine Geräte» (concept 27 4.5, F-15): every device that holds a session or downloads of the
 * account, with its kind, «Dieses Gerät», the sign-in and the last contact; a device that only
 * holds downloads says so. «Abmelden» asks first and removes what the account has on the device
 * (the server decides what that is). Every profile reads the list; only the primary profile signs
 * devices out (like the notification channels), another profile sees why it cannot. Signing out
 * this device ends the own session: the page then signs out here as at «Abmelden».
 */
export function MyDevicesPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('settings.devices.title'));
  const session = useActiveSession();
  const canSignOut = session.profile.isPrimary;
  const queryClient = useQueryClient();
  const logout = useLogout();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const listId = useId();
  const [message, setMessage] = useStatusMessage();
  const [confirming, setConfirming] = useState<MyDevice | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const devices = useQuery({ ...myDevicesQuery, refetchOnWindowFocus: !confirmOpen });

  const name = (device: MyDevice) =>
    device.name ??
    (device.type ? t(`settings.devices.types.${device.type}`) : t('settings.devices.unnamed'));
  const date = (iso: string) => formatDateTime(iso, intlLocale(i18n.language));

  const remove = useMutation({
    mutationFn: (device: MyDevice) => signOut(device.id),
    onSuccess: async (_, device) => {
      if (device.current) {
        // The own session went with it: sign out here as well (the server answers without one).
        logout.mutate();
        return;
      }
      setMessage(t('settings.devices.signedOut', { name: name(device) }));
      await queryClient.invalidateQueries({ queryKey: myDevicesQuery.queryKey });
      // The row and its button are gone: the h1 takes the focus.
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
  });

  const ask = (device: MyDevice) => {
    remove.reset();
    setMessage(null);
    setConfirming(device);
    setConfirmOpen(true);
  };
  const onConfirmOpenChange = (open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  };

  const list = devices.data ?? [];
  return (
    <AdminPage
      title={t('settings.devices.title')}
      help={t('settings.devices.intro')}
      titleRef={titleRef}
    >
      {!canSignOut && (
        <Notice title={t('settings.devices.onlyPrimary.title')}>
          {t('settings.devices.onlyPrimary.body')}
        </Notice>
      )}
      <Panel as="section" aria-labelledby={listId}>
        <SectionHeader id={listId} title={t('settings.devices.list')} />
        {devices.isPending && <Skeleton shape="block" />}
        {devices.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, devices.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void devices.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {devices.isSuccess && list.length === 0 && (
          <Text tone="secondary">{t('settings.devices.none')}</Text>
        )}
        {list.length > 0 && (
          <ul className={styles.list} aria-labelledby={listId}>
            {list.map((device) => {
              const headingId = `device-${device.id}`;
              return (
                <li key={device.id} className={styles.device}>
                  <div className={styles.text}>
                    <span id={headingId} className={styles.name}>
                      {name(device)}
                    </span>
                    <Text tone="muted" variant="caption">
                      {device.sessionKind && device.signedInAt
                        ? t('settings.devices.signedIn', {
                            kind: t(`settings.devices.kinds.${device.sessionKind}`),
                            date: date(device.signedInAt),
                          })
                        : t('settings.devices.downloadsOnly')}
                    </Text>
                    <Text tone="muted" variant="caption">
                      {t('settings.devices.lastSeen', { date: date(device.lastSeenAt) })}
                      {device.downloads > 0 &&
                        ` · ${t('settings.devices.downloads', { count: device.downloads })}`}
                    </Text>
                  </div>
                  {device.current && <Chip tone="info">{t('settings.devices.thisDevice')}</Chip>}
                  {canSignOut && (
                    <AriaButton
                      variant="dangerQuiet"
                      aria-label={t('settings.devices.signOut.label', { name: name(device) })}
                      isDisabled={remove.isPending}
                      onPress={() => ask(device)}
                    >
                      {t('settings.devices.signOut.action')}
                    </AriaButton>
                  )}
                </li>
              );
            })}
          </ul>
        )}
        {remove.isError && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
        <StatusRegion message={message} />
      </Panel>
      {confirming && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={t('settings.devices.signOut.title', { name: name(confirming) })}
          body={
            confirming.current
              ? `${t('settings.devices.signOut.body')} ${t('settings.devices.signOut.current')}`
              : t('settings.devices.signOut.body')
          }
          confirmLabel={t('settings.devices.signOut.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => remove.mutate(confirming)}
        />
      )}
    </AdminPage>
  );
}
