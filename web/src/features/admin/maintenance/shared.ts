import type {
  DuplicateGroup,
  Issue,
  IssueKind,
  RelinkPair,
  RelinkUserData,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { issueKinds } from '../queries';

// The parts the maintenance page and its tabs share: tab ids, keys, the outcome line and the
// page's confirmation.

export const DUPLICATES = 'duplicates';
export const RELINK = 'relink';

/**
 * The tabs in their order: one per kind of issue, the duplicates (PFL-1), then the orphaned user
 * data (ADR 0023).
 */
export const TABS: readonly string[] = [...issueKinds, DUPLICATES, RELINK];

/** Selection key of an issue: unplayable issues are per file, the others per title. */
export const issueKey = (issue: Issue) => `${issue.itemId}:${issue.mediaFileId ?? ''}`;

/** Key of a duplicate group: its titles, which no other group shares. */
export const groupKey = (group: DuplicateGroup) => group.items.map((item) => item.itemId).join('-');

/**
 * The whole path of a file or folder in the container (concept 20, P-10): the library root's
 * folder, then the path below it. A folder ends with «/».
 */
export function fullPath(rootPath: string | undefined, path: string, folder = false): string {
  const root = (rootPath ?? '').replace(/\/+$/, '');
  const joined = path === '' ? root : root === '' ? path : `${root}/${path}`;
  return folder ? `${joined}/` : joined;
}

/** Key of a pair of a deleted title and its live copy. */
export const pairKey = (pair: RelinkPair) => `${pair.deletedId}:${pair.liveId}`;

/** The tables of the user data in the order the summary names them. */
const userDataKeys = [
  'progress',
  'progressEvents',
  'progressDays',
  'playbackSessions',
  'watchlist',
  'collectionItems',
  'notifications',
  'userPicks',
  'downloads',
  'activityEvents',
] as const satisfies readonly (keyof RelinkUserData)[];

/**
 * The user data of the pairs, summed per table: «3 Fortschritte, 140 Wiedergabe-Ereignisse».
 * Tables without a row are left out; no row at all gives an empty string.
 */
export const userDataSummary = (t: TFunction, pairs: readonly RelinkPair[]) =>
  userDataKeys
    .map((key) => [key, pairs.reduce((sum, pair) => sum + pair.userData[key], 0)] as const)
    .filter(([, count]) => count > 0)
    .map(([key, count]) => t(`admin.maintenance.relink.data.${key}`, { count }))
    .join(', ');

/** Open-state key of a library's section in a tab. */
export const libraryKey = (kind: IssueKind, libraryId: number) => `${kind}:${libraryId}`;

export const titleWithYear = (t: TFunction, title: string, year: number | undefined) =>
  year !== undefined ? t('admin.titleYear', { title, year }) : title;

/** Outcome line of a tab after an action. */
export type Outcome = { tone: 'info' | 'danger'; text: string };

/** A confirmation the page opens itself: the rows that asked for it may go meanwhile. */
export interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  tone?: 'danger';
  onConfirm: () => void;
}
