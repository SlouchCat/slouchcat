import { expect, type Locator, type Page } from '@playwright/test';

// The frame of the administration (redesign, build plan 2): the sidebar holds the overview and the
// five areas as a tree (Explorer), each area a button that folds its pages in and out below it;
// the pages of an area are also tabs above each of its pages.

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

/** An area's button in the sidebar's tree (aria-expanded); a pill or badge may follow its name. */
export const adminArea = (page: Page, name: string) =>
  adminNav(page).getByRole('button', { name: new RegExp(`^${name}(,|$)`) });

/**
 * Opens every closed collapsible in `scope` (concept 24, P-3: all of them start closed): the
 * toggles in headings, as DisclosureGroup and a collapsible BandPanel write them. `first` is one
 * of them; it is awaited, so the groups are drawn before the others are counted. Row toggles of a
 * table are not in a heading and stay as they are.
 */
export async function openGroups(scope: Page | Locator, first: Locator): Promise<void> {
  await expect(first).toBeVisible();
  const closed = scope.getByRole('heading').getByRole('button', { expanded: false });
  for (let count = await closed.count(); count > 0; count = await closed.count()) {
    await closed.first().click();
    await expect.poll(() => closed.count()).not.toBe(count);
  }
}
