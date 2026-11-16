import type { Library, LibrarySaved } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useState, type ReactNode } from 'react';
import { librariesQuery } from '../queries';
import type { SaveSummary } from './libraryForm';
import { LibraryForm } from './LibraryForm';

interface Editing {
  id: number;
  rootId?: number;
  /** Counts the fillings of the form: a new opening and a reload after `stale` start afresh. */
  session: number;
  reloaded: boolean;
}

/**
 * «Bearbeiten» of a library from any page (Bibliotheken, Hörbuch-Quellen, Metadaten; concept 24
 * 4.4, 4.5): `start` opens the form of a library, with the card of `rootId` open (exception 1),
 * and `render` gives the form to put on the page. Each opening mounts the form afresh, so it starts at the
 * library's values; a stale form (409 `stale`) loads the libraries again and is filled anew with
 * the notice that it was.
 */
export function useLibraryEdit(onSaved: (saved: LibrarySaved, summary: SaveSummary) => void): {
  start: (id: number, rootId?: number) => void;
  /** The form of the library being edited, out of the page's list of libraries. */
  render: (libraries: readonly Library[] | undefined) => ReactNode;
  isOpen: boolean;
} {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState<Editing | null>(null);
  const [isOpen, setOpen] = useState(false);
  const start = useCallback((id: number, rootId?: number) => {
    setEditing((current) => ({
      id,
      rootId,
      session: (current?.session ?? 0) + 1,
      reloaded: false,
    }));
    setOpen(true);
  }, []);
  const render = (libraries: readonly Library[] | undefined) => {
    const library = libraries?.find((entry) => entry.id === editing?.id);
    return library && editing ? (
      <LibraryForm
        key={editing.session}
        mode="edit"
        library={library}
        openRootId={editing.rootId}
        reloaded={editing.reloaded}
        isOpen={isOpen}
        onOpenChange={setOpen}
        onSaved={onSaved}
        onStale={() =>
          void queryClient
            .fetchQuery({ ...librariesQuery, staleTime: 0 })
            .finally(() =>
              setEditing(
                (current) =>
                  current && { ...current, session: current.session + 1, reloaded: true },
              ),
            )
            .catch(() => undefined)
        }
      />
    ) : null;
  };
  return { start, render, isOpen };
}
