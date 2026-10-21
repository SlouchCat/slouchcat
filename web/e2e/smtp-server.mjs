// Local SMTP server for the Playwright suite (M8, A-6, A-7, E-8): the e2e server sends its mails
// here (SLOUCHCAT_SMTP_HOST/_PORT, _TLS=none) instead of a real mail server, and the specs read
// them over HTTP. start-server.mjs starts it next to the Slouchcat binary and stops it with it;
// `node e2e/smtp-server.mjs` runs it alone.
//
// SMTP on 127.0.0.1:<port> (four below the app's port), plain text only, no STARTTLS:
//   EHLO/HELO, AUTH PLAIN and AUTH LOGIN (SMTP_USER/SMTP_PASSWORD, else 535), MAIL FROM (only
//   after AUTH, else 530), RCPT TO, DATA (dot-unstuffed), RSET, NOOP, QUIT.
// HTTP on 127.0.0.1:<httpPort> (eight below the app's port):
//   GET  /__mails     [{id, from, to, headerFrom, headerTo, subject, text, html, receivedAt}]:
//                     envelope sender and recipients, the decoded subject (RFC 2047) and the
//                     text/plain and text/html parts (quoted-printable and base64 decoded)
//   POST /__reset     forgets every mail and the control state
//   POST /__control   {"rejectAuth": n} answers the next n AUTH attempts with 535 although the
//                     credentials are right (a failing send with the password in play, A-9)
import { createServer as createHttpServer } from 'node:http';
import { createServer as createTcpServer } from 'node:net';
import { fileURLToPath } from 'node:url';

/** Credentials the e2e server is started with (SLOUCHCAT_SMTP_USER/_PASSWORD); fake values. */
export const SMTP_USER = 'e2e';
export const SMTP_PASSWORD = 'e2e-smtp-secret-3f9a61c2b7d4e805';
/** Sender address of the e2e server (SLOUCHCAT_SMTP_FROM). */
export const SMTP_FROM = 'slouchcat@e2e.test';

const HOSTNAME = 'e2e.test';

/** Bytes of a string in an 8-bit transfer: the characters as UTF-8. */
const utf8 = (text) => Buffer.from(text, 'utf8');

function decodeBytes(bytes, charset = 'utf-8') {
  try {
    return new TextDecoder(charset).decode(bytes);
  } catch {
    return new TextDecoder('utf-8').decode(bytes);
  }
}

/** Quoted-printable (RFC 2045 6.7) to bytes; `underscores` for the Q encoding of RFC 2047. */
function quotedPrintable(text, underscores = false) {
  const source = text.replace(/=\r?\n/g, '');
  const parts = [];
  for (let index = 0; index < source.length; index += 1) {
    const char = source[index];
    const hex = source.slice(index + 1, index + 3);
    if (char === '=' && /^[0-9A-Fa-f]{2}$/.test(hex)) {
      parts.push(Buffer.from([parseInt(hex, 16)]));
      index += 2;
    } else if (underscores && char === '_') {
      parts.push(Buffer.from([0x20]));
    } else {
      parts.push(utf8(char));
    }
  }
  return Buffer.concat(parts);
}

/** Decodes the encoded words of a header (RFC 2047); whitespace between two of them goes. */
export function decodeHeader(value) {
  return value
    .replace(/(=\?[^?]+\?[BbQq]\?[^?]*\?=)\s+(?==\?)/g, '$1')
    .replace(/=\?([^?]+)\?([BbQq])\?([^?]*)\?=/g, (_word, charset, encoding, text) =>
      decodeBytes(
        encoding.toUpperCase() === 'B' ? Buffer.from(text, 'base64') : quotedPrintable(text, true),
        charset,
      ),
    );
}

/** Header fields (lower-case names, unfolded) and the body of a message or MIME part. */
function splitMessage(raw) {
  const match = /\r?\n\r?\n/.exec(raw);
  const head = match ? raw.slice(0, match.index) : raw;
  const body = match ? raw.slice(match.index + match[0].length) : '';
  const headers = {};
  for (const line of head.replace(/\r?\n[ \t]+/g, ' ').split(/\r?\n/)) {
    const colon = line.indexOf(':');
    if (colon <= 0) continue;
    const name = line.slice(0, colon).trim().toLowerCase();
    headers[name] ??= line.slice(colon + 1).trim();
  }
  return { headers, body };
}

/** `text/plain; charset="utf-8"` → {type: 'text/plain', params: {charset: 'utf-8'}}. */
function contentType(value = 'text/plain') {
  const [type = 'text/plain', ...rest] = value.split(';');
  const params = {};
  for (const part of rest) {
    const equals = part.indexOf('=');
    if (equals < 0) continue;
    params[part.slice(0, equals).trim().toLowerCase()] = part
      .slice(equals + 1)
      .trim()
      .replace(/^"(.*)"$/, '$1');
  }
  return { type: type.trim().toLowerCase(), params };
}

function decodeBody(body, encoding = '7bit', charset = 'utf-8') {
  switch (encoding.toLowerCase()) {
    case 'base64':
      return decodeBytes(Buffer.from(body.replace(/\s+/g, ''), 'base64'), charset);
    case 'quoted-printable':
      return decodeBytes(quotedPrintable(body), charset);
    default:
      return body;
  }
}

/** The text/plain and text/html parts of a message, multipart nested at any depth. */
function textParts(raw, found = { text: null, html: null }) {
  const { headers, body } = splitMessage(raw);
  const { type, params } = contentType(headers['content-type']);
  if (type.startsWith('multipart/') && params.boundary) {
    const delimiter = `--${params.boundary}`;
    const sections = body.split(delimiter).slice(1);
    for (const section of sections) {
      if (section.startsWith('--')) break;
      textParts(section.replace(/^\r?\n/, ''), found);
    }
    return found;
  }
  const decoded = decodeBody(body, headers['content-transfer-encoding'], params.charset);
  const text = decoded.replace(/\r\n/g, '\n').replace(/\n$/, '');
  if (type === 'text/plain' && found.text === null) found.text = text;
  if (type === 'text/html' && found.html === null) found.html = text;
  return found;
}

/** A received mail as the HTTP side lists it. */
export function parseMail(raw, envelope) {
  const { headers } = splitMessage(raw);
  const { text, html } = textParts(raw);
  return {
    from: envelope.from,
    to: envelope.to,
    headerFrom: decodeHeader(headers.from ?? ''),
    headerTo: decodeHeader(headers.to ?? ''),
    subject: decodeHeader(headers.subject ?? ''),
    text: text ?? '',
    html,
  };
}

/** `<a@b>` (with optional parameters after it) → `a@b`. */
const address = (argument) => /<([^>]*)>/.exec(argument)?.[1] ?? argument.trim().split(/\s/)[0];

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

/**
 * Starts the SMTP mock on 127.0.0.1:`port` (SLOUCHCAT_E2E_SMTP_PORT, default 8093) and its HTTP
 * side on 127.0.0.1:`httpPort` (SLOUCHCAT_E2E_SMTP_HTTP_PORT, default 8089); resolves with both
 * ports and a `close` function.
 */
export function startSmtpServer({
  port = Number(process.env.SLOUCHCAT_E2E_SMTP_PORT ?? 8093),
  httpPort = Number(process.env.SLOUCHCAT_E2E_SMTP_HTTP_PORT ?? 8089),
} = {}) {
  let mails = [];
  let nextId = 1;
  let rejectAuth = 0;
  const sockets = new Set();

  const credentialsValid = (user, password) => {
    if (rejectAuth > 0) {
      rejectAuth -= 1;
      return false;
    }
    return user === SMTP_USER && password === SMTP_PASSWORD;
  };

  const smtp = createTcpServer((socket) => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
    socket.on('error', () => socket.destroy());
    socket.setEncoding('utf8');
    const reply = (line) => socket.write(`${line}\r\n`);
    let buffer = '';
    let authed = false;
    let envelope = null;
    let data = null; // lines of the message while in DATA
    let pending = null; // continuation of AUTH: (line) => void

    const authPlain = (encoded) => {
      const [, user = '', password = ''] = Buffer.from(encoded, 'base64')
        .toString('utf8')
        .split('\0');
      authed = credentialsValid(user, password);
      reply(
        authed
          ? '235 2.7.0 Authentication successful'
          : '535 5.7.8 Authentication credentials invalid',
      );
    };
    const authLogin = (encodedUser) => {
      const user = Buffer.from(encodedUser, 'base64').toString('utf8');
      reply('334 UGFzc3dvcmQ6');
      pending = (line) => {
        const password = Buffer.from(line, 'base64').toString('utf8');
        authed = credentialsValid(user, password);
        reply(
          authed
            ? '235 2.7.0 Authentication successful'
            : '535 5.7.8 Authentication credentials invalid',
        );
      };
    };

    const command = (line) => {
      if (pending) {
        const next = pending;
        pending = null;
        if (line === '*') reply('501 5.0.0 Authentication cancelled');
        else next(line);
        return;
      }
      const [verb = '', ...rest] = line.split(' ');
      const argument = rest.join(' ');
      switch (verb.toUpperCase()) {
        case 'EHLO':
          envelope = null;
          socket.write(
            [
              `250-${HOSTNAME}`,
              '250-8BITMIME',
              '250-SMTPUTF8',
              '250-SIZE 10485760',
              '250 AUTH PLAIN LOGIN',
            ].join('\r\n') + '\r\n',
          );
          return;
        case 'HELO':
          envelope = null;
          reply(`250 ${HOSTNAME}`);
          return;
        case 'AUTH': {
          const [mechanism = '', initial] = argument.split(' ');
          if (authed) reply('503 5.5.1 Already authenticated');
          else if (mechanism.toUpperCase() === 'PLAIN') {
            if (initial) authPlain(initial);
            else {
              reply('334 ');
              pending = authPlain;
            }
          } else if (mechanism.toUpperCase() === 'LOGIN') {
            if (initial) authLogin(initial);
            else {
              reply('334 VXNlcm5hbWU6');
              pending = authLogin;
            }
          } else reply('504 5.5.4 Unrecognized authentication type');
          return;
        }
        case 'STARTTLS':
          reply('454 4.7.0 TLS not available');
          return;
        case 'MAIL':
          if (!authed) reply('530 5.7.0 Authentication required');
          else if (!/^FROM:/i.test(argument)) reply('501 5.5.4 Syntax: MAIL FROM:<address>');
          else {
            envelope = { from: address(argument.slice(5)), to: [] };
            reply('250 2.1.0 OK');
          }
          return;
        case 'RCPT':
          if (!envelope) reply('503 5.5.1 MAIL first');
          else if (!/^TO:/i.test(argument)) reply('501 5.5.4 Syntax: RCPT TO:<address>');
          else {
            envelope.to.push(address(argument.slice(3)));
            reply('250 2.1.5 OK');
          }
          return;
        case 'DATA':
          if (!envelope || envelope.to.length === 0) reply('503 5.5.1 RCPT first');
          else {
            data = [];
            reply('354 End data with <CR><LF>.<CR><LF>');
          }
          return;
        case 'RSET':
          envelope = null;
          reply('250 2.0.0 OK');
          return;
        case 'NOOP':
          reply('250 2.0.0 OK');
          return;
        case 'VRFY':
          reply('252 2.1.5 Cannot verify');
          return;
        case 'QUIT':
          reply('221 2.0.0 Bye');
          socket.end();
          return;
        default:
          reply('502 5.5.2 Command not recognized');
      }
    };

    const line = (text) => {
      if (data === null) {
        command(text);
        return;
      }
      if (text !== '.') {
        data.push(text.startsWith('.') ? text.slice(1) : text);
        return;
      }
      const id = nextId;
      nextId += 1;
      mails.push({
        id,
        ...parseMail(data.join('\r\n'), envelope),
        receivedAt: new Date().toISOString(),
      });
      data = null;
      envelope = null;
      reply(`250 2.0.0 OK queued as ${id}`);
    };

    socket.on('data', (chunk) => {
      buffer += chunk;
      let end;
      while ((end = buffer.indexOf('\n')) >= 0) {
        line(buffer.slice(0, end).replace(/\r$/, ''));
        buffer = buffer.slice(end + 1);
      }
    });
    reply(`220 ${HOSTNAME} ESMTP Slouchcat e2e mock`);
  });

  const http = createHttpServer((request, response) => {
    const url = new URL(request.url ?? '/', `http://127.0.0.1:${httpPort}`);
    void (async () => {
      if (request.method === 'GET' && url.pathname === '/__mails') {
        response.writeHead(200, {
          'Content-Type': 'application/json',
          'Cache-Control': 'no-store',
        });
        response.end(JSON.stringify(mails));
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__reset') {
        mails = [];
        rejectAuth = 0;
        response.writeHead(204).end();
        return;
      }
      if (request.method === 'POST' && url.pathname === '/__control') {
        const body = JSON.parse((await readBody(request)) || '{}');
        if (!Number.isInteger(body.rejectAuth) || body.rejectAuth < 0) {
          response.writeHead(400).end();
          return;
        }
        rejectAuth = body.rejectAuth;
        response.writeHead(204).end();
        return;
      }
      response.writeHead(404).end();
    })().catch(() => {
      if (!response.headersSent) response.writeHead(500);
      response.end();
    });
  });

  const listen = (server, at) =>
    new Promise((resolve, reject) => {
      server.once('error', reject);
      server.listen(at, '127.0.0.1', () => resolve());
    });

  return Promise.all([listen(smtp, port), listen(http, httpPort)]).then(() => ({
    port,
    httpPort,
    close: () =>
      Promise.all([
        new Promise((done) => {
          for (const socket of sockets) socket.destroy();
          smtp.close(() => done());
        }),
        new Promise((done) => {
          http.closeAllConnections();
          http.close(() => done());
        }),
      ]).then(() => undefined),
  }));
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { port, httpPort } = await startSmtpServer();
  console.log(`smtp mock on 127.0.0.1:${port}, mails on http://127.0.0.1:${httpPort}/__mails`);
}
