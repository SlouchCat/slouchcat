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

/** An entry of the sidebar by its label; a pill or badge may follow in its spoken name. */
export const adminEntry = (page: Page, name: string) =>
  adminNav(page).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });
