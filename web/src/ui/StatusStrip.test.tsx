import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatusStrip } from './StatusStrip';

const tiles = [
  { key: 'running', label: 'Laufend', value: '1' },
  { key: 'queued', label: 'Wartend', value: '12' },
];

describe('StatusStrip', () => {
  it('is a group named by its label, without a heading', () => {
    render(<StatusStrip label="Zusammenfassung" tiles={tiles} />);
    const group = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(
      within(group)
        .getAllByRole('term')
        .map((term) => term.textContent),
    ).toEqual(['Laufend', 'Wartend']);
    expect(screen.queryByRole('heading')).toBeNull();
  });

  it('with `heading` the label is a level 2 heading that names the group', () => {
    render(<StatusStrip label="Serverlast" tiles={tiles} heading />);
    const heading = screen.getByRole('heading', { level: 2, name: 'Serverlast' });
    const group = screen.getByRole('group', { name: 'Serverlast' });
    expect(group).toHaveAttribute('aria-labelledby', heading.id);
    expect(group).not.toHaveAttribute('aria-label');
    expect(within(group).getAllByRole('term')).toHaveLength(2);
  });
});
