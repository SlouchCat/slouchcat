import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { TaskRow } from './TaskRow';

describe('TaskRow', () => {
  it('shows type and subject, the named progress, its figure and the remaining time', () => {
    render(
      <TaskRow
        type="Keyframe-Index"
        subject="Oppenheimer (2023)"
        progress={30}
        progressLabel="Fortschritt Keyframe-Index"
        progressText="12 / 40"
        remaining="noch 3 min"
      />,
    );
    expect(screen.getByText('Keyframe-Index · Oppenheimer (2023)')).toBeVisible();
    expect(screen.getByRole('progressbar', { name: 'Fortschritt Keyframe-Index' })).toHaveAttribute(
      'aria-valuenow',
      '30',
    );
    expect(screen.getByText('12 / 40')).toBeVisible();
    expect(screen.getByText('noch 3 min')).toBeVisible();
  });

  it('shows the type alone without a subject', () => {
    render(<TaskRow type="Bibliotheksscan" progress={8} progressLabel="Fortschritt Scan" />);
    expect(screen.getByText('Bibliotheksscan')).toBeVisible();
  });
});
