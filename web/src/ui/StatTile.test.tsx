import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatTile } from './StatTile';

describe('StatTile', () => {
  it('is a description list of its own without a link', () => {
    render(<StatTile label="Wartend" value="12" />);
    expect(screen.queryByRole('link')).toBeNull();
    expect(screen.getByRole('term')).toHaveTextContent('Wartend');
    expect(screen.getByRole('definition')).toHaveTextContent('12');
  });

  it('with `href` the whole tile is one link, named by its label, number and note', () => {
    render(
      <StatTile label="Aktive Streams" value="2" note="Gerade verbunden" href="/admin/streams" />,
    );
    const link = screen.getByRole('link', { name: 'Aktive Streams 2 Gerade verbunden' });
    expect(link).toHaveAttribute('href', '/admin/streams');
    expect(link).toContainElement(screen.getByRole('term'));
  });
});
