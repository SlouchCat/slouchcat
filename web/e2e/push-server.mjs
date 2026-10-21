// Local Web Push service for the Playwright suite (M8, A-6): stands in for the push service of a
// browser (FCM, Mozilla autopush). Headless browsers cannot receive real pushes, so a spec asks
// this mock for a subscription, registers it with the server (POST /me/push-subscriptions) and
// reads what the server sent. start-server.mjs starts it next to the Slouchcat binary and stops it
// with it; `node e2e/push-server.mjs` runs it alone.
//
//   POST /__subscriptions   {"gone"?: true} → 201 {endpoint, keys: {p256dh, auth}}: a fresh P-256
//                           key pair and auth secret, as PushSubscription.toJSON() gives them; with
//                           `gone` the endpoint lies under /push/__gone/ and answers 410
//   GET  /__subscriptions   [{id, endpoint, auth}] (the e2e teardown scans the log for them)
//   POST /push/<id>         the push message (RFC 8030): needs `TTL`, `Content-Encoding: aes128gcm`
//                           and `Authorization: vapid t=<JWT>, k=<key>` (RFC 8292: ES256, verified
//                           with k; aud = this origin; exp in the future and at most 24 h ahead;
//                           sub mailto: or https:), else 400/401. The body is decrypted with the
//                           subscription's keys (RFC 8291, RFC 8188) and recorded. 201 Created.
//                           An id this mock never handed out answers 404.
//   POST /push/__gone/…     410 Gone (the subscription expired; the server must delete it)
//   GET  /__pushes          [{id, status, ttl, urgency, bodyLength, payload, error, receivedAt}]
//                           with `payload` the decrypted JSON (or text), `error` why not
//   POST /__reset           forgets the recorded pushes (subscriptions stay valid)
import {
  createDecipheriv,
  createECDH,
  createHmac,
  createPublicKey,
  randomBytes,
  verify,
} from 'node:crypto';
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

const b64url = (bytes) => Buffer.from(bytes).toString('base64url');
const fromB64url = (text) => Buffer.from(text, 'base64url');
const hmac = (key, data) => createHmac('sha256', key).update(data).digest();

/** Longest VAPID token lifetime a push service accepts (RFC 8292 section 2). */
const MAX_EXPIRY_S = 24 * 60 * 60;

/** Checks the `Authorization: vapid t=…, k=…` header; returns null when valid, else the reason. */
export function vapidProblem(header, origin, now = Date.now()) {
  const match = /^vapid\s+(.*)$/i.exec(header ?? '');
  if (!match?.[1]) return 'Authorization is not vapid';
  const params = Object.fromEntries(
    match[1].split(',').map((part) => {
      const equals = part.indexOf('=');
      return [part.slice(0, equals).trim().toLowerCase(), part.slice(equals + 1).trim()];
    }),
  );
  if (!params.t || !params.k) return 'vapid needs t and k';
  const [head, claims, signature] = params.t.split('.');
  if (!head || !claims || !signature) return 't is not a JWT';
  let jwtHeader;
  let payload;
  try {
    jwtHeader = JSON.parse(fromB64url(head).toString('utf8'));
    payload = JSON.parse(fromB64url(claims).toString('utf8'));
  } catch {
    return 't is not a JWT';
  }
  if (jwtHeader.alg !== 'ES256') return `alg ${jwtHeader.alg} is not ES256`;
  const point = fromB64url(params.k);
  if (point.length !== 65 || point[0] !== 4) return 'k is not an uncompressed P-256 point';
  const key = createPublicKey({
    key: {
      kty: 'EC',
      crv: 'P-256',
      x: b64url(point.subarray(1, 33)),
      y: b64url(point.subarray(33, 65)),
    },
    format: 'jwk',
  });
  const signed = verify(
    'sha256',
    Buffer.from(`${head}.${claims}`),
    { key, dsaEncoding: 'ieee-p1363' },
    fromB64url(signature),
  );
  if (!signed) return 'JWT signature does not verify with k';
  if (payload.aud !== origin) return `aud ${payload.aud} is not ${origin}`;
  const seconds = Math.floor(now / 1000);
  if (!Number.isInteger(payload.exp) || payload.exp <= seconds) return 'exp is not in the future';
  if (payload.exp > seconds + MAX_EXPIRY_S + 60) return 'exp is more than 24 h ahead';
  if (typeof payload.sub !== 'string' || !/^(mailto:|https:)/.test(payload.sub)) {
    return 'sub is not a mailto: or https: URL';
  }
  return null;
}

/**
 * Decrypts an aes128gcm push message (RFC 8291 with the content coding of RFC 8188) with the
 * subscription's private key and auth secret.
 */
export function decryptPush(body, ecdh, authSecret) {
  if (body.length < 21) throw new Error('body shorter than the aes128gcm header');
  const salt = body.subarray(0, 16);
  const recordSize = body.readUInt32BE(16);
  const idLength = body[20];
  const senderKey = body.subarray(21, 21 + idLength);
  const records = body.subarray(21 + idLength);
  if (senderKey.length !== 65) throw new Error('keyid is not the sender public key');
  if (records.length > recordSize) throw new Error('more than one record');

  const receiverKey = ecdh.getPublicKey();
  const secret = ecdh.computeSecret(senderKey);
  const prkKey = hmac(authSecret, secret);
  const keyInfo = Buffer.concat([
    Buffer.from('WebPush: info\0', 'utf8'),
    receiverKey,
    senderKey,
    Buffer.from([1]),
  ]);
  const ikm = hmac(prkKey, keyInfo);
  const prk = hmac(salt, ikm);
  const cek = hmac(prk, Buffer.from('Content-Encoding: aes128gcm\0\x01', 'utf8')).subarray(0, 16);
  const nonce = hmac(prk, Buffer.from('Content-Encoding: nonce\0\x01', 'utf8')).subarray(0, 12);

  const decipher = createDecipheriv('aes-128-gcm', cek, nonce);
  decipher.setAuthTag(records.subarray(records.length - 16));
  const padded = Buffer.concat([
    decipher.update(records.subarray(0, records.length - 16)),
    decipher.final(),
  ]);
  // The last record ends with the delimiter 0x02 and zero padding.
  let end = padded.length - 1;
  while (end >= 0 && padded[end] === 0) end -= 1;
  if (padded[end] !== 2) throw new Error('padding delimiter of the last record missing');
  return padded.subarray(0, end).toString('utf8');
}

function readBody(request) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    request.on('data', (chunk) => chunks.push(chunk));
    request.on('end', () => resolve(Buffer.concat(chunks)));
    request.on('error', reject);
  });
}

const sendJson = (response, status, body) => {
  response.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
  response.end(JSON.stringify(body));
};

/**
 * Starts the push mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_PUSH_PORT, default 8092); resolves
 * with its port and a `close` function.
 */
export function startPushServer({
  port = Number(process.env.SLOUCHCAT_E2E_PUSH_PORT ?? 8092),
} = {}) {
  const origin = `http://127.0.0.1:${port}`;
  /** id → {ecdh, auth, endpoint} */
  const subscriptions = new Map();
  let pushes = [];

  const record = (entry) => pushes.push({ ...entry, receivedAt: new Date().toISOString() });

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', origin);
    void (async () => {
      if (url.pathname === '/__subscriptions' && request.method === 'POST') {
        const body = JSON.parse((await readBody(request)).toString('utf8') || '{}');
        const id = randomBytes(16).toString('hex');
        const ecdh = createECDH('prime256v1');
        ecdh.generateKeys();
        const auth = randomBytes(16);
        const endpoint = `${origin}/push/${body.gone ? '__gone/' : ''}${id}`;
        subscriptions.set(id, { ecdh, auth, endpoint });
        sendJson(response, 201, {
          endpoint,
          expirationTime: null,
          keys: { p256dh: b64url(ecdh.getPublicKey()), auth: b64url(auth) },
        });
        return;
      }
      if (url.pathname === '/__subscriptions' && request.method === 'GET') {
        sendJson(
          response,
          200,
          [...subscriptions].map(([id, entry]) => ({
            id,
            endpoint: entry.endpoint,
            auth: b64url(entry.auth),
          })),
        );
        return;
      }
      if (url.pathname === '/__pushes' && request.method === 'GET') {
        sendJson(response, 200, pushes);
        return;
      }
      if (url.pathname === '/__reset' && request.method === 'POST') {
        pushes = [];
        response.writeHead(204).end();
        return;
      }
      const match = /^\/push\/(__gone\/)?([0-9a-f]{32})$/.exec(url.pathname);
      if (!match || request.method !== 'POST') {
        response.writeHead(match ? 405 : 404).end();
        return;
      }
      const [, gone, id = ''] = match;
      const body = await readBody(request);
      const entry = {
        id,
        ttl: request.headers.ttl ?? null,
        urgency: request.headers.urgency ?? null,
        bodyLength: body.length,
        payload: null,
        error: null,
      };
      const reject = (status, reason) => {
        record({ ...entry, status, error: reason });
        sendJson(response, status, { reason });
      };
      if (!/^\d+$/.test(request.headers.ttl ?? '')) return reject(400, 'TTL header missing');
      if (request.headers['content-encoding'] !== 'aes128gcm') {
        return reject(400, 'Content-Encoding is not aes128gcm');
      }
      const vapid = vapidProblem(request.headers.authorization, origin);
      if (vapid) return reject(401, vapid);
      const subscription = subscriptions.get(id);
      if (!subscription) return reject(404, 'no such subscription');
      if (gone) return reject(410, 'subscription expired');

      try {
        const text = decryptPush(body, subscription.ecdh, subscription.auth);
        try {
          entry.payload = JSON.parse(text);
        } catch {
          entry.payload = text;
        }
      } catch (error) {
        // A real push service cannot read the message either: it delivers, the browser fails.
        entry.error = `decryption failed: ${error instanceof Error ? error.message : String(error)}`;
      }
      record({ ...entry, status: 201 });
      response.writeHead(201, { Location: `/m/${randomBytes(8).toString('hex')}` }).end();
    })().catch(() => {
      if (!response.headersSent) response.writeHead(500);
      response.end();
    });
  });

  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', () => {
      resolve({
        port,
        close: () =>
          new Promise((done) => {
            server.closeAllConnections();
            server.close(() => done());
          }),
      });
    });
  });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { port } = await startPushServer();
  console.log(`push mock on http://127.0.0.1:${port}`);
}
