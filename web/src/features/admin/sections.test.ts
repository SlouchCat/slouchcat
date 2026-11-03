import { describe, expect, it } from 'vitest';
import { adminSections, isSectionPath, type AdminSection } from './sections';

const section = (key: AdminSection['key']): AdminSection => {
  const found = adminSections.find((entry) => entry.key === key);
  if (!found) throw new Error(`no section ${key}`);
  return found;
};

/** The keys of the entries that are current on `pathname`. */
const current = (pathname: string) =>
  adminSections.filter((entry) => isSectionPath(entry, pathname)).map((entry) => entry.key);

describe('isSectionPath', () => {
  it('matches the page and its sub-pages, not a page that only shares the prefix', () => {
    expect(isSectionPath(section('streams'), '/admin/streams')).toBe(true);
    expect(isSectionPath(section('streams'), '/admin/streams/ps-1')).toBe(true);
    expect(isSectionPath(section('streams'), '/admin/streamsx')).toBe(false);
    expect(isSectionPath(section('streams'), '/admin')).toBe(false);
  });

  it('counts the further prefixes: a title’s admin page belongs to «Pflege»', () => {
    expect(current('/admin/items/01JB8Z6Q4M7N9P2R3S5T7V9X01')).toEqual(['maintenance']);
    expect(current('/admin/maintenance')).toEqual(['maintenance']);
  });

  it('gives /admin/accounts to «Konten» alone', () => {
    expect(current('/admin/accounts')).toEqual(['accounts']);
  });

  it('knows the entry outside /admin: «Benachrichtigungen» is /settings/notifications', () => {
    expect(current('/settings/notifications')).toEqual(['notifications']);
    expect(current('/settings/profiles')).toEqual([]);
  });
});
