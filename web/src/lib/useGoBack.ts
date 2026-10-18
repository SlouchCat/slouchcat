import { useCanGoBack, useRouter } from '@tanstack/react-router';
import { useCallback } from 'react';

/** Goes one step back in the app history, or to `fallback` when the page was opened directly. */
export function useGoBack(fallback: string): () => void {
  const router = useRouter();
  const canGoBack = useCanGoBack();
  return useCallback(() => {
    if (canGoBack) router.history.back();
    else void router.navigate({ href: fallback, replace: true });
  }, [canGoBack, fallback, router]);
}
