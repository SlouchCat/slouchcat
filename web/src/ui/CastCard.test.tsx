import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CastCard } from './CastCard';

describe('CastCard (concept 17 D-5)', () => {
  it('shows the initial without a portrait', () => {
    const { container } = render(<CastCard href="/people/1" name="sigourney Weaver" />);
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByRole('link', { name: /sigourney Weaver/ })).toHaveTextContent('S');
  });

  it('shows the portrait when the server named one', () => {
    const { container } = render(
      <CastCard href="/people/1" name="Sigourney Weaver" imageUrl="/api/v1/people/1/image?v=a" />,
    );
    expect(container.querySelector('img')).toHaveAttribute('src', '/api/v1/people/1/image?v=a');
  });

  it('falls back to the initial when the portrait does not load', () => {
    const { container } = render(
      <CastCard href="/people/1" name="Sigourney Weaver" imageUrl="/api/v1/people/1/image?v=a" />,
    );
    const image = container.querySelector('img');
    expect(image).not.toBeNull();
    if (image) fireEvent.error(image);
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByRole('link', { name: /Sigourney Weaver/ })).toHaveTextContent('S');
  });
});
