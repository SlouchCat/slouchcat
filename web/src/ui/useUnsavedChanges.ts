import { useBlocker, type AnyRouter, type ShouldBlockFn } from '@tanstack/react-router';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

/** Search parameters whose change keeps the page and its drafts (S-14, S-17). */
const SAME_PAGE_KEYS: readonly string[] = ['tab', 'item'];

/** Whether two searches differ in anything but `keep`. */
function differsBeyond(
  current: Record<string, unknown>,
  next: Record<string, unknown>,
  keep: readonly string[],
): boolean {
  const keys = new Set([...Object.keys(current), ...Object.keys(next)]);
  return [...keys].some(
    (key) => !keep.includes(key) && JSON.stringify(current[key]) !== JSON.stringify(next[key]),
  );
}

interface UnsavedChangesOptions {
  /**
   * More search parameters whose change stays on the page. `tab` and `item` always do. A page
   * names its own parameters that select something on the same page and keep the drafts: the
   * scope `?library=` of the metadata page, `?add=` of the picks, `?days=` of the statistics.
   */
  keep?: readonly string[];
}

/** Props for ConfirmDialog: the question asked while a navigation waits. */
export interface UnsavedChangesDialog {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: string;
  body: string;
  confirmLabel: string;
  cancelLabel: string;
  tone: 'danger';
  /** Enter by habit stays on the page. */
  defaultFocus: 'cancel';
  onConfirm: () => void;
}

/**
 * Guards a page with unsaved input (concept 12, S-13): leaving it, by a link, the back button or
 * the sidebar, waits for a question (TanStack Router `useBlocker`); reloading or closing the tab
 * gets the browser's own question. Changing only `?tab=` or `?item=` on the same path is not
 * leaving: the drafts live in the page (useDrafts, S-17).
 *
 * The page renders the question with `<ConfirmDialog {...unsaved.dialog} />` and pauses its
 * polling while `pausePolling` is set, so a refetch never overwrites what is being typed.
 */
export function useUnsavedChanges(
  isDirty: boolean,
  { keep = [] }: UnsavedChangesOptions = {},
): { pausePolling: boolean; dialog: UnsavedChangesDialog } {
  const { t } = useTranslation();
  const keepKey = keep.join(',');
  const shouldBlockFn = useCallback<ShouldBlockFn<AnyRouter>>(
    ({ current, next }) =>
      current.pathname !== next.pathname ||
      differsBeyond(
        current.search as Record<string, unknown>,
        next.search as Record<string, unknown>,
        [...SAME_PAGE_KEYS, ...keepKey.split(',').filter(Boolean)],
      ),
    [keepKey],
  );
  const blocker = useBlocker<AnyRouter, true>({
    shouldBlockFn,
    enableBeforeUnload: isDirty,
    disabled: !isDirty,
    withResolver: true,
  });
  const asking = blocker.status === 'blocked';
  return {
    pausePolling: isDirty,
    dialog: {
      isOpen: asking,
      onOpenChange: (open) => {
        if (!open && blocker.status === 'blocked') blocker.reset();
      },
      title: t('ui.unsaved.title'),
      body: t('ui.unsaved.body'),
      confirmLabel: t('ui.unsaved.leave'),
      cancelLabel: t('ui.unsaved.stay'),
      tone: 'danger',
      defaultFocus: 'cancel',
      onConfirm: () => {
        if (blocker.status === 'blocked') blocker.proceed();
      },
    },
  };
}
