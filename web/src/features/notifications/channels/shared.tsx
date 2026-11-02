import type { Channel, ChannelSetting, ChannelSettings, EventType } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { channelsQuery } from '../queries';

// The parts the blocks of the notification channels page share: the events, the draft of a
// channel, the page's confirmation, the outcome line and the focus helpers.

/** Events in the order of the checkboxes; `errors` is for the main account only. */
export const EVENTS: readonly EventType[] = [
  'new_content',
  'errors',
  'invitations',
  'announcements',
];

/** The icon in each channel's band, as on the integrations page. */
export const CHANNEL_ICON = { email: 'megaphone', telegram: 'external', push: 'bell' } as const;

/** What the switch, the address and the events of a channel hold before «Speichern». */
export interface ChannelDraft {
  enabled: boolean;
  events: EventType[];
  /** The e-mail address; empty for the other channels. */
  address: string;
}

/**
 * What the server holds for a channel, in the draft's shape: the events in checkbox order, the
 * account's address where the channel has none yet, so changing a field back is no change.
 */
export function channelDraftOf(setting: ChannelSetting, accountEmail: string): ChannelDraft {
  return {
    enabled: setting.enabled,
    events: EVENTS.filter((event) => setting.events.includes(event)),
    address: setting.channel === 'email' ? (setting.email?.address ?? accountEmail) : '',
  };
}

export type ChannelState = 'on' | 'off' | 'unavailable';

export const stateOf = (setting: ChannelSetting): ChannelState =>
  !setting.available ? 'unavailable' : setting.enabled ? 'on' : 'off';

/** A question the page asks before an action (one dialog for the whole page). */
export interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  tone: 'danger';
  onConfirm: () => void;
}

export type Ask = (confirmation: Confirmation) => void;

/** Puts a channel the server answered with into the cached list. */
export function useStoreChannel() {
  const queryClient = useQueryClient();
  return (next: ChannelSetting) =>
    queryClient.setQueryData<ChannelSettings>(channelsQuery.queryKey, (old) =>
      old
        ? { channels: old.channels.map((entry) => (entry.channel === next.channel ? next : entry)) }
        : old,
    );
}

/** Asks the server for the channels again (after a link, an unlink or a device change). */
export function useRefreshChannels() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
}

/** While a Telegram link code is open, the page asks every few seconds whether the bot got it. */
export const LINK_POLL_MS = 3000;

/** The order of the channels on the page does not depend on the server's. */
export const CHANNEL_ORDER: readonly Channel[] = ['email', 'telegram', 'push'];
