// Base64url (RFC 4648 section 5, padding optional) to bytes: the server's VAPID public key becomes
// the `applicationServerKey` of PushManager.subscribe, which wants the raw bytes.

/** The bytes of a base64url string; throws on characters outside the alphabet. */
export function base64UrlToBytes(value: string): Uint8Array<ArrayBuffer> {
  const trimmed = value.trim().replace(/=+$/, '');
  if (!/^[A-Za-z0-9_-]*$/.test(trimmed) || trimmed.length % 4 === 1) {
    throw new Error('not base64url');
  }
  const base64 = trimmed.replace(/-/g, '+').replace(/_/g, '/');
  const binary = atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, '='));
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
  return bytes;
}
