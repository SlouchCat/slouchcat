import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ActivityFeed } from './ActivityFeed';

describe('ActivityFeed', () => {
  it('lists the sentences in order, each with its machine-readable time', () => {
    render(
      <ActivityFeed
        label="Letzte Aktivität"
        entries={[
          {
            id: '2',
            icon: 'play',
            tone: 'accent',
            text: 'Anna hat «Glut» gestartet',
            time: 'vor 5 min',
            dateTime: '2026-11-01T12:40:00Z',
          },
          {
            id: '1',
            icon: 'power',
            text: 'Server gestartet, Version 1.7.0',
            time: 'gestern',
            dateTime: '2026-10-31T07:12:00Z',
          },
        ]}
      />,
    );
    const items = within(screen.getByRole('list', { name: 'Letzte Aktivität' })).getAllByRole(
      'listitem',
    );
    expect(items.map((item) => item.textContent)).toEqual([
      'Anna hat «Glut» gestartetvor 5 min',
      'Server gestartet, Version 1.7.0gestern',
    ]);
    expect(screen.getByText('vor 5 min')).toHaveAttribute('datetime', '2026-11-01T12:40:00Z');
    // A sentence cut by the line is shown whole as its title.
    expect(screen.getByText('Server gestartet, Version 1.7.0')).toHaveAttribute(
      'title',
      'Server gestartet, Version 1.7.0',
    );
  });
});
