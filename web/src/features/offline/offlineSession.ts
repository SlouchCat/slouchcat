import { useSyncExternalStore } from 'react';

// The profile chosen in the offline library (/offline). It lives in memory only: a reload asks
// for the profile (and its PIN) again, and nothing in the URL can skip the PIN.

export interface OfflineChoice {
  accountId: number;
  profileId: number;
}

let choice: OfflineChoice | null = null;
const listeners = new Set<() => void>();

function emit(): void {
  for (const listener of listeners) listener();
}

export function chooseOfflineProfile(next: OfflineChoice): void {
  choice = next;
  emit();
}

export function leaveOfflineProfile(): void {
  choice = null;
  emit();
}

export const offlineChoice = (): OfflineChoice | null => choice;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useOfflineChoice(): OfflineChoice | null {
  return useSyncExternalStore(subscribe, offlineChoice, offlineChoice);
}
