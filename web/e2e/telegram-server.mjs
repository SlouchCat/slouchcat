// Local Telegram Bot API for the Playwright suite (M8, A-6): the e2e server talks to it instead of
// api.telegram.org (SLOUCHCAT_TELEGRAM_BASE_URL, honoured with --dev only), so no test needs the
// internet or a real bot. start-server.mjs starts it next to the Slouchcat binary and stops it with
// it; `node e2e/telegram-server.mjs` runs it alone.
//
//   /bot<token>/getMe        {ok, result: {id, is_bot, first_name, username: BOT_USERNAME}}
//   /bot<token>/sendMessage  chat_id and text (JSON, form or query) → recorded, {ok, result: message}
//   /bot<token>/getUpdates   offset, limit, timeout (JSON, form or query) like the real API:
//                            updates below `offset` are confirmed and dropped; without one pending,
//                            the request waits up to `timeout` s (at most 25) for the next one
//   POST /__control          {"start": "CODE", "chatId": 4242} queues the message "/start CODE" from
//                            that chat (the user follows the link instruction); {"text", "chatId"}
//                            any message; {"updates": [...]} raw updates; {"drop": n} closes the
//                            connection of the next n bot requests without an answer (a network
//                            failure with the token in the request URL, A-9); {"fail": n} answers
//                            them with 502; with "method": "sendMessage" only requests of that
//                            method count (a poller's getUpdates passes)
//   GET  /__messages         [{chatId, text, parseMode, at}] sent by the bot
//   POST /__reset            forgets messages, queued updates and the failure counters
//
// Methods are case-insensitive like Telegram's. A wrong token answers 401 Unauthorized.
import { createServer } from 'node:http';
import { fileURLToPath } from 'node:url';

/** Bot token the e2e server is started with (SLOUCHCAT_TELEGRAM_BOT_TOKEN); a fake one. */
export const TELEGRAM_BOT_TOKEN = '7000000001:AAE2E_fakeBotToken_slouchcatMock9c3d';
/** Username of the mock bot (getMe); the settings page tells the user to write to it. */
export const BOT_USERNAME = 'slouchcat_e2e_bot';
/** Chat the specs link by default. */
export const CHAT_ID = 4242;

const BOT_ID = 7000000001;
/** Longest wait of a long poll; Telegram allows 50 s, the specs need much less. */
const MAX_POLL_S = 25;

function readBody(request) {
  return new Promise((resolve, reject) => {
    let body = '';
    request.setEncoding('utf8');
    request.on('data', (chunk) => {
      body += chunk;
    });
    request.on('end', () => resolve(body));
    request.on('error', reject);
  });
}

/** Parameters of a bot request: query string, then a JSON or form body. */
async function botParams(request, url) {
  const params = Object.fromEntries(url.searchParams);
  if (request.method !== 'POST') return params;
  const body = await readBody(request);
  if (!body) return params;
  const type = request.headers['content-type'] ?? '';
  if (type.includes('application/json')) return { ...params, ...JSON.parse(body) };
  return { ...params, ...Object.fromEntries(new URLSearchParams(body)) };
}

const sendJson = (response, status, body) => {
  response.writeHead(status, { 'Content-Type': 'application/json' });
  response.end(JSON.stringify(body));
};

const botError = (response, status, description) =>
  sendJson(response, status, { ok: false, error_code: status, description });

/**
 * Starts the Telegram mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_TELEGRAM_PORT, default 8091);
 * resolves with its port and a `close` function.
 */
export function startTelegramServer({
  port = Number(process.env.SLOUCHCAT_E2E_TELEGRAM_PORT ?? 8091),
} = {}) {
  let updates = [];
  let messages = [];
  let nextUpdateId = 100;
  let nextMessageId = 1;
  let drop = 0;
  let fail = 0;
  /** The method drop and fail apply to (lower case), or null for every bot request. */
  let failMethod = null;
  /** Long polls waiting for an update. */
  const waiters = new Set();

  const now = () => Math.floor(Date.now() / 1000);
  const queue = (update) => {
    updates.push({ update_id: nextUpdateId, ...update });
    nextUpdateId += 1;
    for (const wake of waiters) wake();
  };
  const textUpdate = (chatId, text) => {
    const command = /^\/\w+/.exec(text);
    queue({
      message: {
        message_id: nextMessageId++,
        date: now(),
        chat: { id: chatId, type: 'private', first_name: 'E2E' },
        from: { id: chatId, is_bot: false, first_name: 'E2E', language_code: 'de' },
        text,
        ...(command
          ? { entities: [{ type: 'bot_command', offset: 0, length: command[0].length }] }
          : {}),
      },
    });
  };

  const getUpdates = async (params, response) => {
    const offset = params.offset === undefined ? undefined : Number(params.offset);
    if (offset !== undefined && Number.isFinite(offset)) {
      updates = updates.filter((update) => update.update_id >= offset);
    }
    const limit = Math.min(Math.max(Number(params.limit ?? 100) || 100, 1), 100);
    const timeout = Math.min(Math.max(Number(params.timeout ?? 0) || 0, 0), MAX_POLL_S);
    if (updates.length === 0 && timeout > 0) {
      await new Promise((resolve) => {
        const done = () => {
          clearTimeout(timer);
          waiters.delete(done);
          response.off('close', done);
          resolve();
        };
        const timer = setTimeout(done, timeout * 1000);
        waiters.add(done);
        // A client that gives up (server shutdown, its own timeout) ends the wait.
        response.on('close', done);
      });
    }
    return updates.slice(0, limit);
  };

  const server = createServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${port}`);
    void (async () => {
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        const chatId = Number.isInteger(body.chatId) ? body.chatId : CHAT_ID;
        if (typeof body.start === 'string') textUpdate(chatId, `/start ${body.start}`);
        if (typeof body.text === 'string') textUpdate(chatId, body.text);
        if (Array.isArray(body.updates)) for (const update of body.updates) queue(update);
        if (Number.isInteger(body.drop)) drop = body.drop;
        if (Number.isInteger(body.fail)) fail = body.fail;
        if (body.drop !== undefined || body.fail !== undefined) {
          failMethod = typeof body.method === 'string' ? body.method.toLowerCase() : null;
        }
        response.writeHead(204).end();
        return;
      }
      if (request.method === 'GET' && url.pathname === '/__messages') {
        sendJson(response, 200, messages);
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__reset') {
        updates = [];
        messages = [];
        drop = 0;
        fail = 0;
        failMethod = null;
        response.writeHead(204).end();
        return;
      }

      const match = /^\/bot([^/]+)\/([A-Za-z]+)$/.exec(url.pathname);
      if (!match) {
        botError(response, 404, 'Not Found');
        return;
      }
      const [, token, method = ''] = match;
      if (token !== TELEGRAM_BOT_TOKEN) {
        botError(response, 401, 'Unauthorized');
        return;
      }
      const targeted = failMethod === null || failMethod === method.toLowerCase();
      if (targeted && drop > 0) {
        drop -= 1;
        request.socket.destroy();
        return;
      }
      if (targeted && fail > 0) {
        fail -= 1;
        botError(response, 502, 'Bad Gateway');
        return;
      }
      let params;
      try {
        params = await botParams(request, url);
      } catch {
        botError(response, 400, "Bad Request: can't parse JSON");
        return;
      }
      switch (method.toLowerCase()) {
        case 'getme':
          sendJson(response, 200, {
            ok: true,
            result: {
              id: BOT_ID,
              is_bot: true,
              first_name: 'Slouchcat E2E',
              username: BOT_USERNAME,
              can_join_groups: false,
              can_read_all_group_messages: false,
              supports_inline_queries: false,
            },
          });
          return;
        case 'sendmessage': {
          const chatId = params.chat_id;
          const text = params.text;
          if (chatId === undefined || chatId === '') {
            botError(response, 400, 'Bad Request: chat_id is empty');
            return;
          }
          if (typeof text !== 'string' || text.trim() === '') {
            botError(response, 400, 'Bad Request: message text is empty');
            return;
          }
          messages.push({
            chatId: String(chatId),
            text,
            parseMode: params.parse_mode ?? null,
            at: new Date().toISOString(),
          });
          sendJson(response, 200, {
            ok: true,
            result: {
              message_id: nextMessageId++,
              date: now(),
              chat: { id: Number(chatId), type: 'private', first_name: 'E2E' },
              from: {
                id: BOT_ID,
                is_bot: true,
                first_name: 'Slouchcat E2E',
                username: BOT_USERNAME,
              },
              text,
            },
          });
          return;
        }
        case 'getupdates': {
          const result = await getUpdates(params, response);
          if (!response.destroyed) sendJson(response, 200, { ok: true, result });
          return;
        }
        default:
          botError(response, 404, 'Not Found: method not found');
      }
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
            for (const wake of waiters) wake();
            server.closeAllConnections();
            server.close(() => done());
          }),
      });
    });
  });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { port } = await startTelegramServer();
  console.log(`telegram mock on http://127.0.0.1:${port}`);
}
