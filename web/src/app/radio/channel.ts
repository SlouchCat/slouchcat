// The tabs of one browser agree that only one of them plays radio (concept 16, 10.3): a tab that
// starts or resumes a station says so on the channel, every other tab pauses its own.

export const RADIO_CHANNEL = 'slouchcat-radio';

/** The one message of the channel: tab `tabId` plays radio now. */
export interface RadioClaim {
  type: 'claim';
  tabId: string;
}

const isClaim = (data: unknown): data is RadioClaim =>
  typeof data === 'object' &&
  data !== null &&
  (data as { type?: unknown }).type === 'claim' &&
  typeof (data as { tabId?: unknown }).tabId === 'string';

export interface RadioChannel {
  /** Tells the other tabs that this one plays now. */
  claim(): void;
  close(): void;
}

/**
 * Opens the channel for this tab; `onClaim` runs when another tab claims the radio. Without
 * BroadcastChannel (an old browser) the tabs do not coordinate and the functions do nothing.
 */
export function openRadioChannel(tabId: string, onClaim: () => void): RadioChannel {
  if (typeof BroadcastChannel === 'undefined') return { claim: () => {}, close: () => {} };
  const channel = new BroadcastChannel(RADIO_CHANNEL);
  channel.onmessage = (event: MessageEvent<unknown>) => {
    if (isClaim(event.data) && event.data.tabId !== tabId) onClaim();
  };
  return {
    claim: () => channel.postMessage({ type: 'claim', tabId } satisfies RadioClaim),
    close: () => channel.close(),
  };
}
