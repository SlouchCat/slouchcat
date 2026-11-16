import type { Page } from '@playwright/test';

// The frame of the administration (redesign, build plan 2): the sidebar holds the overview and the
// five areas, the pages of an area are tabs above each of its pages.

/** The administration's sidebar (from 768 px) or the sheet's copy (on a phone). */
export const adminNav = (page: Page) => page.getByRole('navigation', { name: 'Verwaltung' });

/** The tabs of the open page's area ("Seiten im Bereich Medien"). */
export const areaTabs = (page: Page) =>
  page.getByRole('navigation', { name: /^Seiten im Bereich / });

/** A page's tab by its title; a number or pill may follow in its spoken name. */
export const areaTab = (page: Page, name: string) =>
  areaTabs(page).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });

/**
 * Tab presses from the top of an admin page (`resetFocus`) to the main action of its head, such
 * as «Bibliothek hinzufügen»; only the guard of `tabTo`, the order is checked in
 * admin-libraries.spec.ts. Counted on 15 November 2026 in Chromium and Firefox: skip link, menu,
 * logo, the two links of the breadcrumb, «Bereich suchen», notifications, profile, the sidebar
 * (one stop), the area tabs (one stop), the action: 11. Before the redesign it was 9 (the search
 * link in place of breadcrumb and «Bereich suchen», the help button before the action), and the
 * old guard of 10 still held where the sidebar, loaded lazily (AppLayout), was not there yet,
 * which is why Chromium passed with it and Firefox did not.
 */
export const TABS_TO_PAGE_ACTION = 11;

/** An entry of the sidebar by its label; a pill or badge may follow in its spoken name. */
export const adminEntry = (page: Page, name: string) =>
  adminNav(page).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });
