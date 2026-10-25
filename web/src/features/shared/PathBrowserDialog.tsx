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
  /** The folder the dialog starts in; a path below a root opens that root. */
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
 * The folder picker of the library form (idea 02). It starts on the browsable roots, goes one
 * level down per Enter and back up with Backspace; "Apply" writes the path of the directory the
 * dialog stands in -- or of the marked row -- into the folder field.
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
  const [position, setPosition] = useState<Position>(NO_ROOT);
  const [selected, setSelected] = useState<string | null>(null);

  const roots = useQuery({ ...browseRootsQuery, enabled: isOpen });
  const atRoots = position.rootId === '';
  const dir = useInfiniteQuery(directoryQuery(position.rootId, position.rel, isOpen && !atRoots));
  const listing = lastPage(dir.data?.pages);

  const close = () => {
    setPosition(NO_ROOT);
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
    const entry = listing?.entries.find((e) => e.rel === id);
    if (entry && !entry.symlink) goTo({ rootId: position.rootId, rel: entry.rel });
  };
  const up = () => {
    if (atRoots) return;
    if (!listing || listing.atRoot) {
      goTo(NO_ROOT);
      return;
    }
    goTo({ rootId: position.rootId, rel: listing.parentRel ?? '' });
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
  const chosen = chosenPath(atRoots, roots.data, listing, selected);
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
          <p className={styles.path}>
            {atRoots ? t('library.browse.roots') : (listing?.path ?? '')}
          </p>
          {!atRoots && listing && (
            <Text tone="secondary" variant="caption">
              {[
                listing.fsType && t('admin.libraries.fsType', { type: listing.fsType }),
                listing.watch && t(`admin.libraries.watch.${listing.watch}`),
              ]
                .filter(Boolean)
                .join(t('format.separator'))}
            </Text>
          )}
        </div>
        {!atRoots && listing?.warning && (
          <Notice
            tone="warning"
            live={false}
            title={t(`setup.library.warning.${listing.warning}.title`)}
          >
            {t(`setup.library.warning.${listing.warning}.body`)}
          </Notice>
        )}
        {listing?.problem && (
          <Notice tone="warning" live={false}>
            {t(
              `library.browse.${listing.problem === 'not_readable' ? 'notReadable' : 'notDirectory'}`,
            )}
          </Notice>
        )}
        {listing?.truncated && (
          <Notice tone="warning" live={false}>
            {t('library.browse.truncated')}
          </Notice>
        )}
        {error && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
        {pending && <Skeleton shape="block" />}
        {!pending && !error && (
          <PathList
            // A fresh list per directory, so the focus moves to its first row (PathList.autoFocus).
            key={`${position.rootId}:${position.rel}`}
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
      {currentPath !== '' && (
        <p className={styles.current}>{t('library.browse.current', { path: currentPath })}</p>
      )}
    </FormDialog>
  );
}

/** The newest page carries the state of the directory; the entries of all pages are shown. */
function lastPage(pages: DirectoryListing[] | undefined): DirectoryListing | undefined {
  if (!pages || pages.length === 0) return undefined;
  const last = pages.at(-1);
  if (!last) return undefined;
  return { ...last, entries: pages.flatMap((page) => page.entries) };
}

function rootItems(t: TFunction, roots: BrowseRoot[] | undefined): PathItem[] {
  return (roots ?? []).map((root) => ({
    id: root.id,
    name: root.path,
    note: root.libraryName
      ? t('library.browse.inLibrary', { name: root.libraryName })
      : root.browsable === false
        ? t('library.browse.closedRoot')
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
