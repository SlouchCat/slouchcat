// ULID (26 characters, Crockford base 32): 48 bits of milliseconds and 80 random bits. The client
// names one offline playback with it (OfflineEvent.playbackId), like the server names its rows.

const ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ';

export function ulid(
  now = Date.now(),
  random: (bytes: Uint8Array<ArrayBuffer>) => void = fill,
): string {
  let time = '';
  let rest = Math.max(0, Math.floor(now));
  for (let index = 0; index < 10; index += 1) {
    time = ALPHABET[rest % 32] + time;
    rest = Math.floor(rest / 32);
  }
  const bytes = new Uint8Array(16);
  random(bytes);
  let tail = '';
  for (let index = 0; index < 16; index += 1) tail += ALPHABET[(bytes[index] ?? 0) % 32];
  return time + tail;
}

function fill(bytes: Uint8Array<ArrayBuffer>): void {
  crypto.getRandomValues(bytes);
}

export const isUlid = (value: string) => /^[0-9A-HJKMNP-TV-Z]{26}$/.test(value);
