import { createContext, createElement, useContext, type ReactNode } from 'react';

// The compact density of the administration (concept 12, 5.2; ADR 0020) covers the content area
// and the dialogs opened from an administration page. Dialogs and menus render into portals on
// <body>, outside the content area, so they cannot inherit it; AdminPage (and the overview, which
// has no AdminPage) sets this scope instead, and the dialogs and menus of web/src/ui put
// `data-density="compact"` on their own portal root. Other portals on /admin (the app's drawer,
// the viewers' announcement dialog) are not inside that scope and keep today's size.
//
// ui/density.css maps the attribute onto the component tokens, only under the administration's
// shell (`data-shell="admin"`) and with a mouse from 768 px.

const AdminDensityContext = createContext(false);

/** Marks everything below it as part of an administration page (set by AdminPage). */
export function AdminDensityScope({ children }: { children: ReactNode }) {
  return createElement(AdminDensityContext, { value: true }, children);
}

/** Attributes for the root of a portal (overlay, popover): compact inside an AdminDensityScope. */
export function useDensityPortalProps(): { 'data-density'?: 'compact' } {
  return useContext(AdminDensityContext) ? { 'data-density': 'compact' } : {};
}
