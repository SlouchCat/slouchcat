import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { PlayerCard } from './PlayerCard';

describe('PlayerCard (V-11)', () => {
  it('an episode: code, title, runtime and progress in its name; pressing it reports', async () => {
    const onPress = vi.fn();
    const { container } = render(
      <PlayerCard
        eyebrow="S01E02"
        title="Die lange Nacht"
        caption="48 Min."
        imageUrl="/still.webp"
        progress={{ percent: 140, label: '40 % gesehen' }}
        isCurrent
        onPress={onPress}
      />,
    );
    const card = screen.getByRole('button', {
      name: 'S01E02 Die lange Nacht 48 Min. 40 % gesehen',
    });
    expect(card).toHaveAttribute('aria-current', 'true');
    expect(container.querySelector('img')).toHaveAttribute('src', '/still.webp');
    // The bar stays within the frame.
    expect(container.querySelector<HTMLElement>('[style]')?.style.width).toBe('100%');
    await userEvent.click(card);
    expect(onPress).toHaveBeenCalledOnce();
  });

  it('a person: a 2:3 portrait, out of the Tab order when asked (concept 23 P-3)', () => {
    const { container } = render(
      <PlayerCard
        shape="person"
        title="rebecca Ferguson"
        caption="als Juliette"
        imageUrl="/portrait.webp"
        onPress={vi.fn()}
        excludeFromTabOrder
      />,
    );
    const card = screen.getByRole('button', { name: 'rebecca Ferguson als Juliette' });
    expect(card).toHaveAttribute('tabindex', '-1');
    expect(card).not.toHaveAttribute('aria-current');
    expect(card).toHaveAttribute('data-shape', 'person');
    const image = container.querySelector('img');
    expect(image).toHaveAttribute('src', '/portrait.webp');
    expect(image).toHaveAttribute('alt', '');
    // Portrait format: 2 wide, 3 high.
    expect(Number(image?.getAttribute('width')) / Number(image?.getAttribute('height'))).toBe(
      2 / 3,
    );
    expect(card).not.toHaveTextContent(/^R/);
  });

  it('a person without a portrait: the initial instead', () => {
    const { container } = render(
      <PlayerCard shape="person" title="rebecca Ferguson" onPress={vi.fn()} />,
    );
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByRole('button', { name: 'rebecca Ferguson' })).toHaveTextContent(/^R/);
  });

  it('a person whose portrait fails to load: the initial, and a new URL gets its own try', () => {
    const { container, rerender } = render(
      <PlayerCard
        shape="person"
        title="Sigourney Weaver"
        imageUrl="/gone.webp"
        onPress={vi.fn()}
      />,
    );
    fireEvent.error(container.querySelector('img')!);
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByRole('button', { name: 'Sigourney Weaver' })).toHaveTextContent(/^S/);
    rerender(
      <PlayerCard shape="person" title="Sigourney Weaver" imageUrl="/new.webp" onPress={vi.fn()} />,
    );
    expect(container.querySelector('img')).toHaveAttribute('src', '/new.webp');
  });
});
