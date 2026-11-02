import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DashCard } from './DashCard';

describe('DashCard', () => {
  it('is a section named by its h2, with the footer after the body', () => {
    render(
      <DashCard icon="server" title="Server" footer={<a href="/admin/jobs">Hintergrundjobs</a>}>
        <p>Inhalt</p>
      </DashCard>,
    );
    const card = screen.getByRole('region', { name: 'Server' });
    expect(screen.getByRole('heading', { level: 2, name: 'Server' }).id).toBe(
      card.getAttribute('aria-labelledby'),
    );
    expect(card).toHaveTextContent(/^ServerInhaltHintergrundjobs$/);
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('reads the number by its spoken form and announces it when live', () => {
    render(
      <DashCard icon="warning" title="Hinweise" count="3" countLabel="3 Hinweise" countLive>
        <p>Inhalt</p>
      </DashCard>,
    );
    const status = screen.getByRole('status');
    expect(status).toHaveTextContent('3 Hinweise');
    expect(screen.getByText('3')).toHaveAttribute('aria-hidden', 'true');
  });
});
