import type { LibrarySummary } from '@slouchcat/api-client';
import { queryOptions, useQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { api, request } from '@/api/client';

/** Libraries the account may hand out: all for the main account, the allowed ones for a guest. */
export const accountLibrariesQuery = queryOptions({
  queryKey: ['libraries'],
  queryFn: ({ signal }): Promise<LibrarySummary[]> => request(api.GET('/libraries', { signal })),
});

export interface LibraryOption {
  value: string;
  label: string;
}

export interface LibraryOptions {
  options: LibraryOption[];
  /** The list is known. Forms must not be offered (and nothing saved) before. */
  isReady: boolean;
  isPending: boolean;
  isError: boolean;
  error: unknown;
  retry: () => void;
}

/**
 * The library choices of a form. A form that sets library rights works only with `isReady`: an
 * empty or missing list must never be read as "no restriction", so pending and failed lists
 * leave the form off. `enabled: false` (a page the account may not use) asks nothing.
 */
export function useLibraryOptions(enabled = true): LibraryOptions {
  const libraries = useQuery({ ...accountLibrariesQuery, enabled });
  return {
    options: (libraries.data ?? []).map((library) => ({
      value: String(library.id),
      label: library.name,
    })),
    isReady: libraries.isSuccess,
    isPending: enabled && libraries.isPending,
    isError: libraries.isError,
    error: libraries.error,
    retry: () => void libraries.refetch(),
  };
}

/** Library ids from the checklist as numbers for a request body. */
export function libraryIdsOf(selection: readonly string[]): number[] {
  return selection.map(Number).filter(Number.isInteger);
}

/** "All libraries", the names of the chosen ones, or "No library", for lists and cards. */
export function describeLibraries(
  t: TFunction,
  rights: { allLibraries: boolean; libraryIds: readonly number[] },
  options: readonly LibraryOption[],
): string {
  if (rights.allLibraries) return t('libraries.all');
  if (rights.libraryIds.length === 0) return t('libraries.none');
  return rights.libraryIds
    .map(
      (id) =>
        options.find((option) => option.value === String(id))?.label ??
        t('libraries.named', { id }),
    )
    .join(', ');
}
