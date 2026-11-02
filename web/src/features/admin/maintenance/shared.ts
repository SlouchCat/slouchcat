import type { DuplicateGroup, Issue, IssueKind } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { issueKinds } from '../queries';

// The parts the maintenance page and its tabs share: tab ids, keys, the outcome line and the
// page's confirmation.

export const DUPLICATES = 'duplicates';

/** The tabs in their order: one per kind of issue, then the duplicates (PFL-1). */
export const TABS: readonly string[] = [...issueKinds, DUPLICATES];

/** Selection key of an issue: unplayable issues are per file, the others per title. */
export const issueKey = (issue: Issue) => `${issue.itemId}:${issue.mediaFileId ?? ''}`;

/** Key of a duplicate group: its titles, which no other group shares. */
export const groupKey = (group: DuplicateGroup) => group.items.map((item) => item.itemId).join('-');

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

/** Resolves once React has rendered what the last state change brought. */
export const afterRender = () =>
  new Promise<void>((resolve) => {
    window.setTimeout(() => window.requestAnimationFrame(() => resolve()), 0);
  });
