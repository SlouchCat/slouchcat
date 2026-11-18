import { useMutation, useQueryClient } from '@tanstack/react-query';
import { dashboardQuery, seerrKeys, syncWishesNow, wishKeys } from '../queries';

/**
 * «Jetzt abgleichen» (syncWishesNow), on «Anfragen» and in the band «Abgleich» of the Seerr view:
 * queues the sync, then reads the wishes, Seerr's counters (the queue) and the overview again.
 */
export function useSyncWishes({
  onMutate,
  onQueued,
}: {
  onMutate: () => void;
  onQueued: () => void;
}) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: syncWishesNow,
    onMutate,
    onSuccess: async () => {
      onQueued();
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: seerrKeys.all }),
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      ]);
    },
  });
}
