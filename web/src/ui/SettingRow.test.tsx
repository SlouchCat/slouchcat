import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { SettingRow } from './SettingRow';

describe('SettingRow (concept 12, S-7)', () => {
  it('names and describes the group of its control', () => {
    render(
      <SettingRow label="Aktiv" help="Die Reihe erscheint auf der Startseite.">
        <input type="checkbox" aria-label="Aktiv" />
      </SettingRow>,
    );
    const group = screen.getByRole('group', { name: 'Aktiv' });
    expect(group).toHaveAccessibleDescription('Die Reihe erscheint auf der Startseite.');
    expect(screen.getByRole('checkbox', { name: 'Aktiv' })).toBeVisible();
  });

  it('shows an addon beside the label, not in the group name', () => {
    render(
      <SettingRow label="TMDB" labelAddon={<span>Gespeichert</span>}>
        <input type="password" aria-label="TMDB-Schlüssel" />
      </SettingRow>,
    );
    expect(screen.getByRole('group', { name: 'TMDB' })).toBeVisible();
    expect(screen.getByText('Gespeichert')).toBeVisible();
  });
});
