// Local Radio Browser and test station for the Playwright suite (concept 16, webradio). The e2e
// binary (bin/slouchcat-e2e, build tag radiotest) reads its catalog from here
// (SLOUCHCAT_TEST_RADIO_CATALOG_URL) and may dial the loopback, which the production binary
// refuses. start-server.mjs starts it nine below the app's port and stops it with the server;
// `node e2e/radio-server.mjs` runs it alone. It answers over raw sockets, because a Shoutcast v1
// server answers `ICY 200 OK`, which no HTTP library writes.
//
//   GET /json/stations   testdata/radio/catalog.json with {{BASE}} replaced by this server's URL
//                        (query ignored); /json/servers answers [] (no mirror in the test build)
//   GET /mp3             Icecast MP3 (HTTP/1.0 200, audio/mpeg; the relay transcodes it)
//   GET /sc              Shoutcast v1 MP3 (ICY 200 OK)
//   GET /aac             Icecast ADTS AAC (HTTP/1.0 200, audio/aac; the relay copies it, B-1)
//   GET /drop            cuts every open stream (the relay reconnects) and answers 404, so a
//                        station whose stream is /drop is unreachable
//
// The mounts loop testdata/radio/tone-mp3-128.mp3 and tone-aac-64.aac at their bitrate and, when
// the request sends `Icy-MetaData: 1`, put a StreamTitle into the stream every 10 s (icy-metaint
// 8192). Without the tone file a mount answers 503. Every other path is 404.
import { readFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { fileURLToPath } from 'node:url';

const RADIO = fileURLToPath(new URL('../../testdata/radio/', import.meta.url));
const METAINT = 8192;
const TITLE_SECONDS = 10;
const TICK_MS = 100;

const mounts = {
  '/mp3': {
    status: 'HTTP/1.0 200 OK',
    type: 'audio/mpeg',
    file: 'tone-mp3-128.mp3',
    kbps: 128,
    name: 'Testradio Eins',
  },
  '/sc': {
    status: 'ICY 200 OK',
    type: 'audio/mpeg',
    file: 'tone-mp3-128.mp3',
    kbps: 128,
    name: 'Testradio Zwei',
  },
  '/aac': {
    status: 'HTTP/1.0 200 OK',
    type: 'audio/aac',
    file: 'tone-aac-64.aac',
    kbps: 64,
    name: 'Testradio Drei',
  },
};

/** The metadata block of one StreamTitle: a length byte (×16) and the padded text. */
function metaBlock(title) {
  const text = Buffer.from(`StreamTitle='${title.replaceAll("'", '')}';`, 'utf8');
  const blocks = Math.ceil(text.length / 16);
  const out = Buffer.alloc(1 + blocks * 16);
  out[0] = blocks;
  text.copy(out, 1);
  return out;
}

/**
 * Starts the server on 127.0.0.1:`port` (SLOUCHCAT_E2E_RADIO_PORT, default 8088); resolves with its
 * port and a `close` function.
 */
export function startRadioServer({
  port = Number(process.env.SLOUCHCAT_E2E_RADIO_PORT ?? 8088),
} = {}) {
  const base = `http://127.0.0.1:${port}`;
  const catalog = Buffer.from(
    readFileSync(`${RADIO}catalog.json`, 'utf8').replaceAll('{{BASE}}', base),
    'utf8',
  );
  const streams = new Set();

  const answer = (socket, status, type, body) => {
    socket.end(
      Buffer.concat([
        Buffer.from(
          `HTTP/1.1 ${status}\r\nContent-Type: ${type}\r\nContent-Length: ${body.length}\r\n` +
            'Cache-Control: no-store\r\nConnection: close\r\n\r\n',
        ),
        body,
      ]),
    );
  };

  const stream = (socket, mount, metadata) => {
    let audio;
    try {
      audio = readFileSync(`${RADIO}${mount.file}`);
    } catch {
      answer(socket, '503 Service Unavailable', 'text/plain', Buffer.from(`${mount.file} missing`));
      return;
    }
    const head = [
      mount.status,
      `Content-Type: ${mount.type}`,
      `icy-name: ${mount.name}`,
      `icy-br: ${mount.kbps}`,
      ...(metadata ? [`icy-metaint: ${METAINT}`] : []),
      'Cache-Control: no-cache',
      '',
      '',
    ].join('\r\n');
    socket.write(head);
    streams.add(socket);
    const perTick = Math.round((mount.kbps * 1000) / 8 / (1000 / TICK_MS));
    let offset = 0; // position in the audio file
    let sinceMeta = 0; // audio bytes since the last metadata block
    let sent = ''; // the title last put into the stream
    const started = Date.now();
    const send = (bytes) => {
      const parts = [];
      while (bytes > 0) {
        const room = metadata ? Math.min(bytes, METAINT - sinceMeta) : bytes;
        const take = Math.min(room, audio.length - offset);
        parts.push(audio.subarray(offset, offset + take));
        offset = (offset + take) % audio.length;
        bytes -= take;
        sinceMeta += take;
        if (metadata && sinceMeta === METAINT) {
          // A new title every TITLE_SECONDS; an unchanged one is the empty block (length 0).
          const due = `Slouchcat Testton - Titel ${Math.floor((Date.now() - started) / 1000 / TITLE_SECONDS) + 1}`;
          parts.push(due === sent ? Buffer.from([0]) : metaBlock(due));
          sent = due;
          sinceMeta = 0;
        }
      }
      socket.write(Buffer.concat(parts));
    };
    send(perTick * 10); // one second ahead, like a real server's burst
    const timer = setInterval(() => send(perTick), TICK_MS);
    const stop = () => {
      clearInterval(timer);
      streams.delete(socket);
    };
    socket.on('close', stop);
    socket.on('error', stop);
  };

  const server = createServer((socket) => {
    let request = '';
    socket.setEncoding('latin1');
    socket.on('error', () => socket.destroy());
    const onData = (chunk) => {
      request += chunk;
      const end = request.indexOf('\r\n\r\n');
      if (end < 0) {
        if (request.length > 16384) socket.destroy();
        return;
      }
      socket.off('data', onData);
      const [line, ...headers] = request.slice(0, end).split('\r\n');
      const [method, target = '/'] = line.split(' ');
      const path = new URL(target, base).pathname;
      const metadata = headers.some((h) => /^icy-metadata:\s*1\s*$/i.test(h));
      if (method !== 'GET') {
        answer(socket, '405 Method Not Allowed', 'text/plain', Buffer.from('GET only'));
      } else if (path === '/json/stations') {
        answer(socket, '200 OK', 'application/json', catalog);
      } else if (path === '/json/servers') {
        answer(socket, '200 OK', 'application/json', Buffer.from('[]'));
      } else if (mounts[path]) {
        stream(socket, mounts[path], metadata);
      } else if (path === '/drop') {
        for (const s of streams) s.destroy();
        streams.clear();
        answer(socket, '404 Not Found', 'text/plain', Buffer.from('dropped'));
      } else {
        answer(socket, '404 Not Found', 'text/plain', Buffer.from('Not Found'));
      }
    };
    socket.on('data', onData);
  });

  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, '127.0.0.1', () => {
      resolve({
        port,
        close: () =>
          new Promise((done) => {
            for (const s of streams) s.destroy();
            server.close(() => done());
          }),
      });
    });
  });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { port } = await startRadioServer();
  console.log(`radio server on http://127.0.0.1:${port}`);
}
