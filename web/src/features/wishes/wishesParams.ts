// «Meine Wünsche» keeps its filter in the URL, like «Gesehen»: the browser's back button returns to
// the same view. Kept apart from the page: the router imports it for `validateSearch` without
// loading the page's chunk.

/** The filter chips besides «Alle» (listWishes `state`). */
export const WISH_FILTERS = ['open', 'available', 'closed'] as const;

export interface WishesParams {
  state?: (typeof WISH_FILTERS)[number];
}

/** Keeps only a known filter; anything else is dropped (`validateSearch`). */
export function validateWishesParams(search: Record<string, unknown>): WishesParams {
  const state = WISH_FILTERS.find((entry) => entry === search.state);
  return state ? { state } : {};
}
