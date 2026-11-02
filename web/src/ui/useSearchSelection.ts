import { useNavigate, useSearch } from '@tanstack/react-router';
import { useCallback } from 'react';

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
): [selected: string, select: (id: string) => void] {
  const search: Record<string, unknown> = useSearch({ strict: false });
  const navigate = useNavigate();
  const value = search[param];
  const selected = typeof value === 'string' && ids.includes(value) ? value : fallback;
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
  return [selected, select];
}
