import type { components, SessionInfo } from '@slouchcat/api-client';
import { QueryClient } from '@tanstack/react-query';
import { createMemoryHistory } from '@tanstack/react-router';
import { render } from '@testing-library/react';
import { beforeAll, vi } from 'vitest';
import { App } from '@/app/App';
import { createAppRouter } from '@/app/router';

// Every page of the app is reached through lazyRouteComponent(() => import(...)) in the router, so
// the first render in a worker waits for Vite to transform that module graph. That wait falls
// inside findBy's one-second window and has nothing to do with what a test asserts: AdminHomePage
// went red that way on a documentation-only commit (run 39767863551) and ProfileLimits on the
// commit after it (run 39837140392), both with a body that had not been drawn yet. Resolving the
// page modules before the first test of a file moves the transform out of every assertion window.
// It lives here and not in setup.ts because only the tests of the whole app (renderApp, and
// renderAdminPage in admin.ts) go through the router: in setup.ts every one of the ~120 test files
// loaded every page, 22 % of the setup time, also the files that render a single component.
const lazyPages = {
  ...import.meta.glob('../features/*/*Page.tsx'),
  ...import.meta.glob('../features/admin/AdminLayout.tsx'),
};

beforeAll(async () => {
  await Promise.all(Object.values(lazyPages).map((load) => load()));
});

type Problem = components['schemas']['Problem'];

/** A recorded API call: method, path below /api/v1, parsed JSON body (or null). */
export interface Call {
  method: string;
  path: string;
  body: unknown;
  keepalive: boolean;
}

type Reply =
  Response | (() => Response | Promise<Response>) | ((call: Call) => Response | Promise<Response>);

/** Route table "METHOD /path" → reply; paths are relative to /api/v1 and may contain `*`. */
export type Routes = Record<string, Reply>;

export const json = (body: unknown, status = 200) => Response.json(body, { status });

export const problem = (
  status: number,
  code: string,
  extra: Partial<Problem> & { extensions?: Record<string, number> } = {},
) =>
  Response.json({ type: 'about:blank', title: code, status, code, ...extra } satisfies Problem, {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  });

export const session: SessionInfo = {
  account: {
    id: 1,
    email: 'owner@example.test',
    role: 'admin',
    isOwner: true,
    maxRating: 18,
    allLibraries: true,
    libraryIds: [],
  },
  profile: {
    id: 1,
    name: 'Marco',
    maxRating: 18,
    language: 'de',
    allLibraries: true,
    libraryIds: [],
    hasPin: false,
    hasOfflinePin: false,
    isPrimary: true,
    limits: {},
    effectiveLimits: {},
  },
};

/** Default answers for the guard: logged in (the guard asks `/setup` only without a session). */
export const signedIn: Routes = {
  'GET /auth/session': () => json(session),
};

function matches(pattern: string, key: string): boolean {
  const regex = new RegExp(
    `^${pattern.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '[^/]+')}$`,
  );
  return regex.test(key);
}

/** Replaces fetch with the route table and records every call. */
export function mockApi(routes: Routes) {
  const calls: Call[] = [];
  const fetchMock = vi.fn(async (input: Request) => {
    const url = new URL(input.url);
    const path = url.pathname.replace(/^\/api\/v1/, '');
    const text = input.method === 'GET' ? '' : await input.clone().text();
    const call: Call = {
      method: input.method,
      path,
      body: text ? (JSON.parse(text) as unknown) : null,
      keepalive: input.keepalive,
    };
    calls.push(call);
    const key = `${input.method} ${path}`;
    const entry = Object.entries(routes).find(([pattern]) => matches(pattern, key));
    if (!entry) return problem(404, 'not_found');
    const reply = entry[1];
    return typeof reply === 'function' ? reply(call) : reply.clone();
  });
  vi.stubGlobal('fetch', fetchMock);
  return { calls, fetchMock };
}

/** Renders the whole app (router, guard, providers) at `path`. */
export function renderApp(path: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const history = createMemoryHistory({ initialEntries: [path] });
  const router = createAppRouter(queryClient, history);
  const view = render(<App router={router} queryClient={queryClient} />);
  return { ...view, router, queryClient, history };
}
