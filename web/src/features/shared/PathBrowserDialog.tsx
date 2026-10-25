import type { BrowseRoot, DirectoryEntry, DirectoryListing } from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, FormDialog, PathList, type PathItem } from '@/ui/aria';
import { browseRootsQuery, directoryQuery } from './pathBrowser';
import styles from './PathBrowserDialog.module.css';

interface PathBrowserDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  /** The folder field's value; a path below a root opens the dialog in that folder. */
  currentPath: string;
  /** "Apply": the chosen absolute path goes into the folder field. */
  onApply: (path: string) => void;
  /** Focused after "Apply" instead of the button that opened the dialog (the folder field). */
  returnFocus?: RefObject<HTMLElement | null>;
}

/** Where the dialog stands: no root chosen yet, or a directory below one. */
interface Position {
  rootId: string;
  rel: string;
}

const NO_ROOT: Position = { rootId: '', rel: '' };

/**
 * The folder picker of the library form (idea 02). It starts on the browsable roots -- or in the
 * folder the field already names -- goes one level down per Enter and back up with Backspace;
 * "Apply" writes the marked folder, or the one the dialog stands in, into the field.
 *
 * Everything it shows comes from the server: which roots exist, what lies below them, which
 * folder belongs to a library already. The client decides no path (plan 2.1, principle 1).
 */
export function PathBrowserDialog({
  isOpen,
  onOpenChange,
  currentPath,
  onApply,
  returnFocus,
}: PathBrowserDialogProps) {
  const { t } = useTranslation();
  // null: the user has not navigated yet, so the dialog stands where the field points -- or on
  // the roots when the field is empty or names something below none of them. Derived, not an
  // effect: the start follows the roots as soon as they arrive, without a second render pass.
  const [position, setPosition] = useState<Position | null>(null);
  const [selected, setSelected] = useState<string | null>(null);

  const roots = useQuery({ ...browseRootsQuery, enabled: isOpen });
  const here = position ?? startPosition(roots.data, currentPath) ?? NO_ROOT;
  const atRoots = here.rootId === '';
  const dir = useInfiniteQuery(directoryQuery(here.rootId, here.rel, isOpen && !atRoots));
  const listing = lastPage(dir.data?.pages);
  // `keepPreviousData` holds the rows of the directory we came from on screen while the next one
  // loads, so the list keeps the focus. Everything but those rows -- the path, the warnings, the
  // way up and what "Apply" writes -- waits for the answer, or it would describe the wrong folder.
  const shown = listing?.rootId === here.rootId && listing.rel === here.rel ? listing : undefined;

  const close = () => {
    setPosition(null);
    setSelected(null);
    onOpenChange(false);
  };
  const goTo = (next: Position) => {
    setPosition(next);
    setSelected(null);
  };
  const enter = (id: string) => {
    if (atRoots) {
      const root = roots.data?.find((r) => r.id === id);
      if (root?.browsable) goTo({ rootId: root.id, rel: '' });
      return;
    }
    const entry = shown?.entries.find((e) => e.rel === id);
    if (entry && !entry.symlink) goTo({ rootId: here.rootId, rel: entry.rel });
  };
  // Back to the roots whenever the current directory is unknown -- still loading, or the server
  // refused it. Going nowhere would strand the keyboard in a folder it cannot leave.
  const up = () => {
    if (atRoots) return;
    if (!shown || shown.atRoot) {
      goTo(NO_ROOT);
      return;
    }
    goTo({ rootId: here.rootId, rel: shown.parentRel ?? '' });
  };
  // Backspace and Left go up, Right goes down: the two keys a file dialog is expected to answer
  // to. The ListBox brings the rest (arrows, Home, End, Enter); the dialog holds no text field,
  // so Backspace cannot swallow a keystroke meant for one.
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Backspace' || event.key === 'ArrowLeft') {
      event.preventDefault();
      up();
      return;
    }
    if (event.key === 'ArrowRight' && selected) {
      event.preventDefault();
      enter(selected);
    }
  };

  const items = atRoots ? rootItems(t, roots.data) : entryItems(t, listing?.entries);
  const chosen = chosenPath(atRoots, roots.data, shown, selected);
  const pending = atRoots ? roots.isPending : dir.isPending;
  const error = atRoots ? roots.error : dir.error;

  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) close();
      }}
      title={t('library.browse.title')}
      initialFocus='[role="option"]'
      returnFocus={returnFocus}
    >
      <div className={styles.body}>
        <div className={styles.head}>
          <p className={styles.path}>{atRoots ? t('library.browse.roots') : (shown?.path ?? '')}</p>
          {shown && (
            <Text tone="secondary" variant="caption">
              {[
                shown.fsType && t('admin.libraries.fsType', { type: shown.fsType }),
                shown.watch && t(`admin.libraries.watch.${shown.watch}`),
              ]
                .filter(Boolean)
                .join(t('format.separator'))}
            </Text>
          )}
        </div>
        {shown?.warning && (
          <Notice
            tone="warning"
            live={false}
            title={t(`setup.library.warning.${shown.warning}.title`)}
          >
            {t(`setup.library.warning.${shown.warning}.body`)}
          </Notice>
        )}
        {shown?.problem && (
          <Notice tone="warning" live={false}>
            {t(
              `library.browse.${shown.problem === 'not_readable' ? 'notReadable' : 'notDirectory'}`,
            )}
          </Notice>
        )}
        {shown?.truncated && (
          <Notice tone="warning" live={false}>
            {t('library.browse.truncated')}
          </Notice>
        )}
        {error && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
        {pending && <Skeleton shape="block" />}
        {!pending && !error && (
          <PathList
            // A fresh list per directory the server answered with, so the focus moves to the
            // first row of that directory. Keyed on the position instead, it would remount while
            // the previous rows are still on screen and focus one of those (PathList.autoFocus).
            key={atRoots ? 'roots' : `${listing?.rootId ?? ''}:${listing?.rel ?? ''}`}
            autoFocus
            label={t('library.browse.list')}
            items={items}
            selectedId={selected}
            onSelectionChange={setSelected}
            onAction={enter}
            onKeyDown={onKeyDown}
            emptyText={
              atRoots
                ? t('library.browse.emptyRoots', { name: 'SLOUCHCAT_BROWSE_ROOTS' })
                : t('library.browse.empty')
            }
          />
        )}
        {dir.hasNextPage && (
          <div>
            <AriaButton isPending={dir.isFetchingNextPage} onPress={() => void dir.fetchNextPage()}>
              {t('library.browse.loadMore')}
            </AriaButton>
          </div>
        )}
        <div className={styles.actions}>
          <AriaButton icon="back" isDisabled={atRoots} onPress={up}>
            {t('library.browse.upOneLevel')}
          </AriaButton>
          <div className={styles.spacer} />
          <AriaButton variant="ghost" onPress={close}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            variant="primary"
            isDisabled={chosen === ''}
            onPress={() => {
              onApply(chosen);
              close();
            }}
          >
            {t('library.browse.apply')}
          </AriaButton>
        </div>
      </div>
      <p className={styles.current}>
        {chosen === ''
          ? t('library.browse.appliesNothing')
          : t('library.browse.applies', { path: chosen })}
      </p>
    </FormDialog>
  );
}

/** The newest page carries the state of the directory; the entries of all pages are shown. */
function lastPage(pages: DirectoryListing[] | undefined): DirectoryListing | undefined {
  const last = pages?.at(-1);
  if (!last) return undefined;
  return { ...last, entries: (pages ?? []).flatMap((page) => page.entries) };
}

/**
 * The root and the path below it that `path` names, or undefined when it lies below none of the
 * browsable roots. The longest matching root wins, so a library root below a configured one
 * opens as itself.
 */
function startPosition(roots: BrowseRoot[] | undefined, path: string): Position | undefined {
  if (path === '' || !roots) return undefined;
  let best: Position | undefined;
  let bestLength = -1;
  for (const root of roots) {
    if (root.browsable === false || root.path.length <= bestLength) continue;
    if (path === root.path) {
      best = { rootId: root.id, rel: '' };
      bestLength = root.path.length;
    } else if (path.startsWith(`${root.path.replace(/\/$/, '')}/`)) {
      best = { rootId: root.id, rel: path.slice(root.path.replace(/\/$/, '').length + 1) };
      bestLength = root.path.length;
    }
  }
  return best;
}

function rootItems(t: TFunction, roots: BrowseRoot[] | undefined): PathItem[] {
  return (roots ?? []).map((root) => ({
    id: root.id,
    name: root.path,
    // A root that cannot be entered says so first: it is almost always a library root too, and
    // "belongs to library X" alone would leave Enter looking broken.
    note:
      root.browsable === false
        ? t('library.browse.closedRoot')
        : root.libraryName
          ? t('library.browse.inLibrary', { name: root.libraryName })
          : undefined,
    closed: root.browsable === false,
  }));
}

function entryItems(t: TFunction, entries: DirectoryEntry[] | undefined): PathItem[] {
  return (entries ?? []).map((entry) => ({
    id: entry.rel,
    name: entry.name,
    note: entry.symlink
      ? t('library.browse.symlink')
      : entry.libraryName
        ? t('library.browse.inLibrary', { name: entry.libraryName })
        : undefined,
    closed: entry.symlink === true,
  }));
}

/** "Apply" takes the marked row, or the directory the dialog stands in. */
function chosenPath(
  atRoots: boolean,
  roots: BrowseRoot[] | undefined,
  listing: DirectoryListing | undefined,
  selected: string | null,
): string {
  if (atRoots) {
    return (selected && roots?.find((root) => root.id === selected)?.path) || '';
  }
  if (selected) {
    const entry = listing?.entries.find((e) => e.rel === selected);
    if (entry?.path) return entry.path;
  }
  return listing?.path ?? '';
}
