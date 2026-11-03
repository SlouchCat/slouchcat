import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ImageTile } from './ImageTile';

describe('ImageTile (concept 12, 11.2)', () => {
  it('shows the image as decoration, and the fallback with the title when it fails', () => {
    const { container } = render(<ImageTile src="/bild.jpg" title="Die Stunde der Dämmerung" />);
    const image = container.querySelector('img');
    expect(image).toHaveAttribute('alt', '');
    expect(screen.queryByText('Die Stunde der Dämmerung')).toBeNull();
    fireEvent.error(image as HTMLImageElement);
    expect(container.querySelector('img')).toBeNull();
    // The page names the title in text; the fallback's copy is not read twice.
    expect(screen.getByText('Die Stunde der Dämmerung')).toHaveAttribute('aria-hidden', 'true');
  });

  it('shows the fallback without an image', () => {
    const { container } = render(<ImageTile title="Heimvideo" shape="portrait" />);
    expect(container.querySelector('img')).toBeNull();
    expect(screen.getByText('Heimvideo')).toBeInTheDocument();
  });
});
