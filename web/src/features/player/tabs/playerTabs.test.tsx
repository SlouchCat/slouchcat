import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import i18n from '@/i18n';
import { usePlayerTabs } from './playerTabs';

const t = i18n.t.bind(i18n);

describe('usePlayerTabs: «Besetzung» (concept 23 P-3)', () => {
  it('shows each person with the portrait the server listed, in portrait columns', () => {
    const tabs = usePlayerTabs({
      t,
      chapters: [],
      positionMs: 0,
      durationMs: 60_000,
      onChapter: vi.fn(),
      onClose: vi.fn(),
      onPerson: vi.fn(),
      cast: [
        {
          personId: 7,
          name: 'Lena Muster',
          role: 'cast',
          character: 'Kommissarin Weber',
          image: { url: '/images/people/7?v=1&p=1', width: 400, height: 600 },
        },
        { personId: 8, name: 'Noah Beispiel', role: 'creator' },
      ],
    });
    const cast = tabs.find((tab) => tab.id === 'cast')?.cards;
    expect(cast?.variant).toBe('portrait');
    const { container } = render(
      <>
        {cast?.render(0, false)}
        {cast?.render(1, true)}
      </>,
    );
    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/api/v1/images/people/7?v=1&p=1',
    );
    expect(container.querySelectorAll('img')).toHaveLength(1);
    expect(screen.getByRole('button', { name: /^Noah Beispiel/ })).toHaveTextContent(/^N/);
  });
});
