import type {
  SessionInfo,
  SetupProgress,
  SetupProgressUpdate,
  SetupStep,
} from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import type { IconName } from '@/ui';

/**
 * The guided wizard of concept 22 part A: nine steps in one order (3.2). Steps 1 and 2 are public
 * and run before an account exists; the owner does the rest behind the login, through the
 * administration's endpoints, and the server keeps the state (getSetupProgress, 3.4).
 */
export const SETUP_STEPS = [
  'language',
  'account',
  'region',
  'services',
  'playback',
  'libraries',
  'household',
  'access',
  'summary',
] as const satisfies readonly SetupStep[];

/** The steps before the account (public routes `/setup/language`, `/setup/account`). */
export const PUBLIC_STEPS: readonly SetupStep[] = ['language', 'account'];

/** Steps without «Überspringen»: language and account are needed, a library too (F-1). */
export const REQUIRED_STEPS: readonly SetupStep[] = ['language', 'account', 'libraries', 'summary'];

/** The icon of a step in the step list while it is open. */
export const STEP_ICONS: Record<SetupStep, IconName> = {
  language: 'list',
  account: 'user',
  region: 'home',
  services: 'plug',
  playback: 'play',
  libraries: 'folder',
  household: 'users',
  access: 'server',
  summary: 'check',
};

export const setupPath = (step: SetupStep) => `/setup/${step}`;

/** The step of a route parameter; undefined for anything else. */
export const setupStepOf = (value: string): SetupStep | undefined =>
  SETUP_STEPS.find((step) => step === value);

/** True for every path of the wizard. */
export const isSetupPath = (pathname: string) =>
  pathname === '/setup' || pathname.startsWith('/setup/');

export const setupProgressQuery = queryOptions({
  queryKey: ['admin', 'setup'],
  queryFn: ({ signal }): Promise<SetupProgress> => request(api.GET('/admin/setup', { signal })),
  staleTime: 30_000,
});

export const updateSetupProgress = (body: SetupProgressUpdate): Promise<SetupProgress> =>
  request(api.PUT('/admin/setup', { body }));

/** The state of a step in the progress; open when the server does not list it. */
export const stepState = (progress: SetupProgress | undefined, step: SetupStep) =>
  progress?.steps.find((entry) => entry.step === step)?.state ?? 'open';

/** Where the owner goes on: the first open step after the account, else the summary. */
export function firstOpenStep(progress: SetupProgress): SetupStep {
  return (
    SETUP_STEPS.find(
      (step) => !PUBLIC_STEPS.includes(step) && stepState(progress, step) === 'open',
    ) ?? 'summary'
  );
}

/** The step before `step`; the language has none. */
export const previousStep = (step: SetupStep): SetupStep | undefined =>
  SETUP_STEPS[SETUP_STEPS.indexOf(step) - 1];

// «Später fortsetzen» (3.4, F-11): the owner goes to the start page until the next sign-in, which
// leads into the wizard again. The browser keeps the choice (localStorage, so a new tab and a
// reopened PWA keep it too) for the account; the login page clears it (clearSetupDeferral).
const LATER_KEY = 'slouchcat.setupLater';

/** «Später fortsetzen» was chosen in this browser since the account's last sign-in here. */
export function setupDeferred(session: SessionInfo): boolean {
  try {
    return localStorage.getItem(LATER_KEY) === String(session.account.id);
  } catch {
    return false;
  }
}

export function deferSetup(session: SessionInfo): void {
  try {
    localStorage.setItem(LATER_KEY, String(session.account.id));
  } catch {
    // Without storage the next navigation leads into the wizard again; nothing is lost.
  }
}

/** A new sign-in ends «Später fortsetzen» (adoptSession). */
export function clearSetupDeferral(): void {
  try {
    localStorage.removeItem(LATER_KEY);
  } catch {
    // Nothing stored, nothing to clear.
  }
}
