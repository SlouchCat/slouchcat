import type { ItemRef } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockApi, problem } from '@/test/api';
import { CoverWatchlist } from './CoverWatchlist';

const id = '44444444-4444-4444-8444-444444444444';
const film: ItemRef = { id, kind: 'movie', title: 'Alien', inWatchlist: false, images: {} };

/** The button over a title held in the query cache, as a list page holds it. */
function Cover({
  onRemoved,
  excludeFromTabOrder,
}: {
  onRemoved?: () => void;
  excludeFromTabOrder?: boolean;
}) {
  const { data } = useQuery<ItemRef>({
    queryKey: ['item', id],
    queryFn: () => new Promise<ItemRef>(() => undefined),
    staleTime: Infinity,
  });
  return data ? (
    <CoverWatchlist item={data} onRemoved={onRemoved} excludeFromTabOrder={excludeFromTabOrder} />
  ) : null;
}

function renderCover(item: ItemRef, props: Parameters<typeof Cover>[0] = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(['item', id], item);
  render(
    <QueryClientProvider client={queryClient}>
      <Cover {...props} />
    </QueryClientProvider>,
  );
  return queryClient;
}

describe('CoverWatchlist', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('marks a title and names the button by its state', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watchlist': () => new Response(null, { status: 204 }),
    });
    const onRemoved = vi.fn();
    renderCover(film, { onRemoved });
    const button = screen.getByRole('button', { name: 'Alien merken' });
    expect(button).toHaveAttribute('aria-pressed', 'false');
    expect(button).not.toHaveAttribute('tabindex');

    await userEvent.setup().click(button);
    const pressed = await screen.findByRole('button', { name: 'Alien von der Merkliste nehmen' });
    expect(pressed).toHaveAttribute('aria-pressed', 'true');
    await waitFor(() =>
      expect(calls.some((c) => c.method === 'PUT' && c.path === `/items/${id}/watchlist`)).toBe(
        true,
      ),
    );
    // Only a removal tells the page to move the focus on.
    expect(onRemoved).not.toHaveBeenCalled();
  });

  it('takes a title off and calls onRemoved once the server agreed', async () => {
    mockApi({ 'DELETE /items/*/watchlist': () => new Response(null, { status: 204 }) });
    const onRemoved = vi.fn();
    renderCover({ ...film, inWatchlist: true }, { onRemoved });
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Alien von der Merkliste nehmen' }));
    await waitFor(() => expect(onRemoved).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('button', { name: 'Alien merken' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('turns back and says why when the server refuses', async () => {
    mockApi({ 'PUT /items/*/watchlist': () => problem(500, 'internal') });
    const onRemoved = vi.fn();
    renderCover(film, { onRemoved });
    await userEvent.setup().click(screen.getByRole('button', { name: 'Alien merken' }));
    expect(await screen.findByRole('alert')).not.toBeEmptyDOMElement();
    expect(screen.getByRole('button', { name: 'Alien merken' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    expect(onRemoved).not.toHaveBeenCalled();
  });

  it('ignores a press while another button changes the same title', async () => {
    let answer: (response: Response) => void = () => undefined;
    const { calls } = mockApi({
      'PUT /items/*/watchlist': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
      'DELETE /items/*/watchlist': () => new Response(null, { status: 204 }),
    });
    // The same title twice (two rows of the start page): two buttons, two mutations.
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    queryClient.setQueryData(['item', id], film);
    render(
      <QueryClientProvider client={queryClient}>
        <Cover />
        <Cover />
      </QueryClientProvider>,
    );
    const user = userEvent.setup();
    const [first] = screen.getAllByRole('button', { name: 'Alien merken' });
    await user.click(first!);
    const pressed = await screen.findAllByRole('button', {
      name: 'Alien von der Merkliste nehmen',
    });
    for (const button of pressed) expect(button).toHaveAttribute('aria-disabled', 'true');
    await user.click(pressed[1]!);
    expect(calls.filter((c) => c.method === 'DELETE')).toHaveLength(0);

    answer(new Response(null, { status: 204 }));
    await waitFor(() => expect(pressed[1]).not.toHaveAttribute('aria-disabled'));
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(1);
  });

  it('is disabled offline', () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    renderCover(film);
    expect(screen.getByRole('button', { name: 'Alien merken' })).toBeDisabled();
  });

  it('leaves the Tab order for a poster of the spotlight row that is not the stop', () => {
    renderCover(film, { excludeFromTabOrder: true });
    expect(screen.getByRole('button', { name: 'Alien merken' })).toHaveAttribute('tabindex', '-1');
  });

  it.each(['season', 'episode', 'extra', 'podcast_episode'] as const)(
    'is not drawn on a %s',
    (kind) => {
      renderCover({ ...film, kind });
      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    },
  );

  it.each(['movie', 'series', 'audiobook', 'podcast'] as const)('is drawn on a %s', (kind) => {
    renderCover({ ...film, kind });
    expect(screen.getByRole('button', { name: 'Alien merken' })).toBeVisible();
  });
});
