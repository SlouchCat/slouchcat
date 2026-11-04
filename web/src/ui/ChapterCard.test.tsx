import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ChapterCard } from './ChapterCard';

describe('ChapterCard (concept 13, V-8)', () => {
  it('shows number, title and start time; pressing it jumps there', async () => {
    const onPress = vi.fn();
    render(
      <ChapterCard
        index={4}
        title="Das Archiv"
        startMs={3_600_000}
        durationMs={9_000_000}
        onPress={onPress}
      />,
    );
    const card = screen.getByRole('button', { name: /Das Archiv/ });
    expect(card).toHaveTextContent('5');
    expect(card).toHaveTextContent('1:00:00');
    expect(card).not.toHaveAttribute('aria-current');
    await userEvent.click(card);
    expect(onPress).toHaveBeenCalledOnce();
  });

  it('names an untitled chapter by its position, never by its time', () => {
    render(
      <ChapterCard
        index={2}
        startMs={1_409_283}
        durationMs={1_800_000}
        onPress={() => undefined}
      />,
    );
    expect(screen.getByRole('button', { name: /Kapitel 3/ })).toHaveTextContent('23:29');
  });

  it('shows the image instead of the number when there is one', () => {
    const { container } = render(
      <ChapterCard
        index={0}
        title="Anfang"
        startMs={0}
        durationMs={60_000}
        imageUrl="c.webp"
        onPress={() => undefined}
      />,
    );
    expect(container.querySelector('img')).toHaveAttribute('src', 'c.webp');
    expect(screen.getByRole('button').textContent).toBe('Anfang0:00');
  });

  it('marks the playing chapter, its progress and a replacement chapter', () => {
    const { container } = render(
      <ChapterCard
        index={1}
        startMs={600_000}
        durationMs={7_200_000}
        progress={140}
        auto
        isCurrent
        onPress={() => undefined}
      />,
    );
    const card = screen.getByRole('button', { name: /Kapitel 2/ });
    expect(card).toHaveAttribute('aria-current', 'true');
    expect(card).toHaveTextContent('automatisch');
    const played = container.querySelector<HTMLElement>('[aria-hidden="true"] > span');
    expect(played?.style.width).toBe('100%');
  });

  it('draws no progress for a chapter not reached yet', () => {
    const { container } = render(
      <ChapterCard index={5} startMs={0} durationMs={60_000} onPress={() => undefined} />,
    );
    expect(container.querySelector('[style]')).toBeNull();
    expect(screen.getByRole('button')).not.toHaveTextContent('automatisch');
  });
});
