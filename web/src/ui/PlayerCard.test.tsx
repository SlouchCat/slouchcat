import { render, screen } from '@testing-library/react';
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

  it('a person: the initial instead of an image, out of the Tab order when asked', () => {
    const { container } = render(
      <PlayerCard
        shape="person"
        title="rebecca Ferguson"
        caption="als Juliette"
        imageUrl="/ignored.webp"
        onPress={vi.fn()}
        excludeFromTabOrder
      />,
    );
    const card = screen.getByRole('button', { name: 'rebecca Ferguson als Juliette' });
    expect(card).toHaveAttribute('tabindex', '-1');
    expect(card).not.toHaveAttribute('aria-current');
    expect(container.querySelector('img')).toBeNull();
    expect(card).toHaveTextContent(/^R/);
  });
});
