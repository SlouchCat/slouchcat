import type {
  Channel,
  ChannelSetting,
  ChannelSettings,
  ChannelSettingUpdate,
  PushSubscriptionRequest,
  PushSubscriptionSummary,
} from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

// Notification channels of the account (M8, A-6); the page loads them with its own chunk.

/** E-mail, Web Push and Telegram with their events (GET /me/channels). */
export const channelsQuery = queryOptions({
  queryKey: ['me', 'channels'],
  queryFn: ({ signal }): Promise<ChannelSettings> => request(api.GET('/me/channels', { signal })),
});

export const updateChannel = (
  channel: Channel,
  body: ChannelSettingUpdate,
): Promise<ChannelSetting> =>
  request(api.PUT('/me/channels/{channel}', { params: { path: { channel } }, body }));

export const testChannel = (channel: Channel): Promise<void> =>
  request(api.POST('/me/channels/{channel}/test', { params: { path: { channel } } }));

/** A new Telegram link code; an older one stops working. */
export const createTelegramLink = (): Promise<ChannelSetting> =>
  request(api.POST('/me/channels/telegram/link'));

export const deleteTelegramLink = (): Promise<void> =>
  request(api.DELETE('/me/channels/telegram/link'));

/** The server's VAPID public key (base64url), the browser's `applicationServerKey`. */
export const vapidKey = async (): Promise<string> =>
  (await request(api.GET('/push/vapid-key'))).publicKey;

export const createPushSubscription = (
  body: PushSubscriptionRequest,
): Promise<PushSubscriptionSummary> => request(api.POST('/me/push-subscriptions', { body }));

export const deletePushSubscription = (id: number): Promise<void> =>
  request(api.DELETE('/me/push-subscriptions/{id}', { params: { path: { id } } }));
