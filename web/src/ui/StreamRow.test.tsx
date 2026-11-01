import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StreamRow } from './StreamRow';

const props = {
  title: 'Glut',
  profile: 'Anna',
  device: 'iPhone',
  mode: 'Direkt',
  progress: 42,
  progressLabel: 'Fortschritt Glut',
  progressText: '0:50:24 / 2:00:00',
};

describe('StreamRow', () => {
  it('shows title, profile and device, mode and the named progress', () => {
    const { container } = render(<StreamRow {...props} posterUrl="/images/1/poster" />);
    expect(screen.getByText('Glut')).toBeVisible();
    expect(screen.getByText('Anna · iPhone')).toBeVisible();
    expect(screen.getByText('Direkt')).toBeVisible();
    expect(screen.getByRole('progressbar', { name: 'Fortschritt Glut' })).toHaveAttribute(
      'aria-valuenow',
      '42',
    );
    // The poster is decoration: the title says what plays.
    expect(container.querySelector('img')).toHaveAttribute('alt', '');
  });

  it('shows the fallback tile without a poster', () => {
    const { container } = render(<StreamRow {...props} />);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('[aria-hidden="true"] > svg')).not.toBeNull();
  });
});
