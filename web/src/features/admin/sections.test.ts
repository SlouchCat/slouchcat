import { describe, expect, it } from 'vitest';
import { adminSections, isSectionPath, type AdminSection } from './sections';

const section = (key: AdminSection['key']): AdminSection => {
  const found = adminSections.find((entry) => entry.key === key);
  if (!found) throw new Error(`no section ${key}`);
  return found;
};

/** The keys of the entries that are current on `pathname` with `item`. */
const current = (pathname: string, item?: unknown) =>
  adminSections.filter((entry) => isSectionPath(entry, pathname, item)).map((entry) => entry.key);

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

  it('compares the path of an entry without its search and fragment', () => {
    // «Profile» links to /admin/accounts?item=me#meine-profile.
    expect(isSectionPath(section('profiles'), '/admin/accounts', 'me')).toBe(true);
  });

  it('gives /admin/accounts to «Konten», and to «Profile» only with item=me', () => {
    expect(current('/admin/accounts')).toEqual(['accounts']);
    expect(current('/admin/accounts', '7')).toEqual(['accounts']);
    expect(current('/admin/accounts', 'me')).toEqual(['profiles']);
    // `item=me` on another page takes nothing from that page's entry.
    expect(current('/admin/devices', 'me')).toEqual(['devices']);
  });

  it('knows the entry outside /admin: «Benachrichtigungen» is /settings/notifications', () => {
    expect(current('/settings/notifications')).toEqual(['notifications']);
    expect(current('/settings/profiles')).toEqual([]);
  });
});
