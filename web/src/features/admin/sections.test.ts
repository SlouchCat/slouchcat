import { describe, expect, it } from 'vitest';
import {
  adminAreas,
  adminSections,
  currentSection,
  firstSectionOf,
  isSectionPath,
  sectionsOf,
  type AdminSection,
} from './sections';

const section = (key: AdminSection['key']): AdminSection => {
  const found = adminSections.find((entry) => entry.key === key);
  if (!found) throw new Error(`no section ${key}`);
  return found;
};

/** The key of the page that is current on `pathname`. */
const current = (pathname: string) => currentSection(pathname)?.key;

describe('the areas (build plan section 1)', () => {
  it('holds the 21 pages in the order of the plan', () => {
    expect(
      Object.fromEntries(adminAreas.map((area) => [area, sectionsOf(area).map((s) => s.key)])),
    ).toEqual({
      media: ['libraries', 'metadata', 'maintenance', 'podcasts', 'radio', 'audiobookSources'],
      people: ['accounts', 'devices', 'streams'],
      home: ['homeRows', 'spotlight', 'picks', 'collections', 'myMedia', 'announcements'],
      automation: ['jobs', 'integrations', 'notifications'],
      system: ['stats', 'backup', 'server'],
    });
    expect(adminSections).toHaveLength(21);
    // The sections are listed area by area, so the sheet and the palette keep the plan's order.
    expect(adminSections.map((entry) => entry.key)).toEqual(
      adminAreas.flatMap((area) => sectionsOf(area).map((entry) => entry.key)),
    );
  });

  it('leads an area to its first page', () => {
    expect(adminAreas.map((area) => firstSectionOf(area).href)).toEqual([
      '/admin/libraries',
      '/admin/accounts',
      '/admin/home-rows',
      '/admin/jobs',
      '/admin/stats',
    ]);
  });
});

describe('isSectionPath', () => {
  it('compares bare paths: no section href carries a query or a fragment', () => {
    for (const entry of adminSections) {
      expect(entry.href).not.toMatch(/[?#]/);
      for (const prefix of entry.also ?? []) expect(prefix).not.toMatch(/[?#]/);
    }
  });

  it('matches the page and its sub-pages, not a page that only shares the prefix', () => {
    expect(isSectionPath(section('streams'), '/admin/streams')).toBe(true);
    expect(isSectionPath(section('streams'), '/admin/streams/ps-1')).toBe(true);
    expect(isSectionPath(section('streams'), '/admin/streamsx')).toBe(false);
    expect(isSectionPath(section('streams'), '/admin')).toBe(false);
  });
});

describe('currentSection', () => {
  it('counts the further prefixes: a title’s admin page belongs to «Pflege»', () => {
    expect(current('/admin/items/01JB8Z6Q4M7N9P2R3S5T7V9X01')).toBe('maintenance');
    expect(current('/admin/maintenance')).toBe('maintenance');
  });

  it('takes the longest matching path: the audiobook sources are not «Metadaten»', () => {
    expect(current('/admin/metadata/audiobooks')).toBe('audiobookSources');
    expect(current('/admin/metadata')).toBe('metadata');
  });

  it('gives a podcast’s page to «Podcasts» and /admin/accounts to «Konten» alone', () => {
    expect(current('/admin/podcasts/pod-1')).toBe('podcasts');
    expect(current('/admin/accounts')).toBe('accounts');
  });

  it('keeps «Benachrichtigungen» inside /admin; the account page under /settings is no entry', () => {
    expect(current('/admin/notifications')).toBe('notifications');
    expect(current('/settings/notifications')).toBeUndefined();
    expect(current('/settings/profiles')).toBeUndefined();
  });

  it('has no page on the overview', () => {
    expect(current('/admin')).toBeUndefined();
  });
});
