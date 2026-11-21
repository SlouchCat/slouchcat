import type { MyDevice } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, session, signedIn } from '@/test/api';

const thisBrowser: MyDevice = {
  id: 'a'.repeat(32),
  type: 'desktop',
  sessionKind: 'browser',
  signedInAt: '2026-11-01T10:00:00Z',
  lastSeenAt: '2026-11-21T09:00:00Z',
  downloads: 0,
  current: true,
};
const phone: MyDevice = {
  id: 'b'.repeat(32),
  name: 'Pixel 9',
  type: 'phone',
  sessionKind: 'app',
  signedInAt: '2026-11-10T10:00:00Z',
  lastSeenAt: '2026-11-20T18:00:00Z',
  downloads: 2,
  current: false,
};
const oldTablet: MyDevice = {
  id: 'c'.repeat(32),
  lastSeenAt: '2026-10-01T18:00:00Z',
  downloads: 1,
  current: false,
};

const list = () => screen.findByRole('list', { name: 'Geräte dieses Kontos' });

describe('my devices page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the devices with kind, this device, sign-in, last contact and downloads', async () => {
    mockApi({ ...signedIn, 'GET /account/devices': () => json([thisBrowser, phone, oldTablet]) });
    renderApp('/settings/devices');
    expect(await screen.findByRole('heading', { level: 1, name: 'Meine Geräte' })).toBeVisible();
    const items = within(await list()).getAllByRole('listitem');
    expect(items).toHaveLength(3);
    const [own, app, tablet] = items as [HTMLElement, HTMLElement, HTMLElement];
    // A device without a name is named by its type.
    expect(within(own).getByText('Computer')).toBeVisible();
    expect(within(own).getByText('Dieses Gerät')).toBeVisible();
    expect(within(own).getByText(/^Browser, angemeldet am/)).toBeVisible();
    expect(within(app).getByText('Pixel 9')).toBeVisible();
    expect(within(app).getByText(/^Handy-App, angemeldet am/)).toBeVisible();
    expect(within(app).getByText(/· 2 Downloads$/)).toBeVisible();
    expect(within(app).queryByText('Dieses Gerät')).toBeNull();
    // Only downloads hold it: no session, the hint instead.
    expect(within(tablet).getByText('Unbekanntes Gerät')).toBeVisible();
    expect(within(tablet).getByText('Abgemeldet, hält noch Downloads dieses Kontos')).toBeVisible();
    expect(
      within(tablet).getByRole('button', { name: 'Unbekanntes Gerät abmelden' }),
    ).toBeVisible();
  });

  it('says so when no device holds the account', async () => {
    mockApi({ ...signedIn, 'GET /account/devices': () => json([]) });
    renderApp('/settings/devices');
    expect(
      await screen.findByText('Kein Gerät hält eine Anmeldung oder Downloads dieses Kontos.'),
    ).toBeVisible();
  });

  it('signs a device out after the confirmation and reloads the list', async () => {
    const user = userEvent.setup();
    let devices = [thisBrowser, phone];
    const { calls } = mockApi({
      ...signedIn,
      'GET /account/devices': () => json(devices),
      [`DELETE /account/devices/${phone.id}`]: () => {
        devices = [thisBrowser];
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/settings/devices');
    await user.click(await screen.findByRole('button', { name: 'Pixel 9 abmelden' }));
    let dialog = await screen.findByRole('alertdialog', { name: '„Pixel 9“ abmelden?' });
    expect(within(dialog).queryByText(/gerade benutzt/)).toBeNull();
    // Cancel sends nothing.
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    await user.click(screen.getByRole('button', { name: 'Pixel 9 abmelden' }));
    dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Gerät abmelden' }));
    expect(await screen.findByText('„Pixel 9“ abgemeldet.')).toBeVisible();
    await waitFor(async () =>
      expect(within(await list()).getAllByRole('listitem')).toHaveLength(1),
    );
    expect(calls.filter((call) => call.method === 'DELETE').map((call) => call.path)).toEqual([
      `/account/devices/${phone.id}`,
    ]);
  });

  it('signs out here after signing out this device', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi({
      ...signedIn,
      'GET /account/devices': () => json([thisBrowser]),
      [`DELETE /account/devices/${thisBrowser.id}`]: () => new Response(null, { status: 204 }),
      'POST /auth/logout': () => new Response(null, { status: 204 }),
    });
    renderApp('/settings/devices');
    await user.click(await screen.findByRole('button', { name: 'Computer abmelden' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText(/Du wirst hier sofort abgemeldet/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Gerät abmelden' }));
    await waitFor(() => expect(calls.some((call) => call.path === '/auth/logout')).toBe(true));
  });

  it('shows another profile the list without sign-out and says why', async () => {
    mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile!, id: 2, isPrimary: false } }),
      'GET /account/devices': () => json([thisBrowser, phone]),
    });
    renderApp('/settings/devices');
    expect(await screen.findByText('Abmelden nur im Hauptprofil')).toBeVisible();
    expect(within(await list()).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.queryByRole('button', { name: /abmelden$/ })).toBeNull();
  });
});
