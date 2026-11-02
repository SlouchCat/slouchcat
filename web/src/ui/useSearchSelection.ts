import { useNavigate, useSearch } from '@tanstack/react-router';
import { useCallback, useEffect } from 'react';

interface SearchSelectionOptions {
  /**
   * Replace an unknown or invalid value in the address with the fallback (replaced, not pushed),
   * so the address says what the page shows. Pass `false` while the ids are not known yet (the
   * lists they come from are still loading); the default is `false`.
   */
  normalise?: boolean;
}

/**
 * A selection kept in the address (concept 12, S-14): the tab of a page in `?tab=`, the selected
 * item of a MasterDetail in `?item=`, so it can be linked and survives a reload. A missing or
 * unknown value falls back to `fallback`; selecting the fallback drops the parameter, so the
 * page's plain address stays the default. The route validates the value with `adminSearch`
 * (app/router.tsx); other search parameters are kept.
 */
export function useSearchSelection(
  param: 'tab' | 'item',
  ids: readonly string[],
  fallback: string,
  { normalise = false }: SearchSelectionOptions = {},
): [selected: string, select: (id: string) => void] {
  const search: Record<string, unknown> = useSearch({ strict: false });
  const navigate = useNavigate();
  const value = search[param];
  const known = typeof value === 'string' && ids.includes(value);
  const selected = known ? value : fallback;
  const select = useCallback(
    (id: string) =>
      void navigate({
        to: '.',
        replace: true,
        search: ((previous: Record<string, unknown>) => ({
          ...previous,
          [param]: id === fallback ? undefined : id,
        })) as never,
      }),
    [navigate, param, fallback],
  );
  const invalid = normalise && value !== undefined && !known;
  useEffect(() => {
    if (invalid) select(fallback);
  }, [invalid, select, fallback]);
  return [selected, select];
}
