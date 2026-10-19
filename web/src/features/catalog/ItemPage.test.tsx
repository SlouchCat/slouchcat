import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alienDetail, homePage, ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

describe('detail page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the title facts and a resume button at the stored position', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienDetail) });
    renderApp(`/items/${ids.alien}`);

    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    expect(screen.getByText('FSK 16')).toBeVisible();
    expect(screen.getByText('Horror, Science-Fiction')).toBeVisible();
    expect(screen.getByText('Im Weltraum hört dich niemand schreien.')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Weiterschauen ab 00:42:13' })).toHaveAttribute(
      'href',
      `/play/${ids.alien}`,
    );
    expect(screen.getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    // Same original title as the title: not repeated.
    expect(screen.queryByText(/Originaltitel/)).not.toBeInTheDocument();

    const file = screen.getByRole('region', { name: 'Datei' });
    expect(within(file).getByText('MP4')).toBeVisible();
    expect(within(file).getByText('1920 × 1080 · 1080p')).toBeVisible();
    expect(within(file).getByText('H264')).toBeVisible();
    expect(within(file).getByText('4,2 GB')).toBeVisible();
  });

  it('offers "Abspielen" without progress and marks unavailable files', async () => {
    const [file] = alienDetail.mediaFiles;
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () =>
        json({
          ...alienDetail,
          progress: undefined,
          originalTitle: 'Alien – Das unheimliche Wesen',
          mediaFiles: [{ ...file, available: false }],
        }),
    });
    renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('button', { name: 'Abspielen' })).toBeDisabled();
    expect(screen.getByText('nicht verfügbar')).toBeVisible();
    expect(screen.getByText('Originaltitel: Alien – Das unheimliche Wesen')).toBeVisible();
  });

  it('shows a not-found message on 404', async () => {
    mockApi({ ...signedIn, 'GET /items/*': () => problem(404, 'not_found') });
    renderApp(`/items/${ids.heat}`);
    expect(await screen.findByText('Titel nicht gefunden')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Startseite' })).toHaveAttribute('href', '/');
  });

  it('goes back to the start page with Escape when opened directly', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    await userEvent.setup().keyboard('{Escape}');
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
  });
});
