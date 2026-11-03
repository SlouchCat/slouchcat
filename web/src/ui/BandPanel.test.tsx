/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { accentNames, tokens } from '@slouchcat/design-tokens';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { BandPanel } from './BandPanel';

describe('BandPanel (concept 12, 11.2)', () => {
  it('is a section named by its h2', () => {
    render(
      <BandPanel title="Webhooks" meta="2 eingerichtet">
        <p>Inhalt</p>
      </BandPanel>,
    );
    expect(screen.getByRole('region', { name: 'Webhooks' })).toBeVisible();
    expect(screen.getByRole('heading', { level: 2, name: 'Webhooks' })).toBeVisible();
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('keeps a hidden title as the heading and name of the section (S-9)', () => {
    render(
      <BandPanel
        title="Sicherungen"
        icon="archive"
        hideTitle
        tabs={{
          label: 'Bereiche',
          tabs: [{ id: 'list', label: 'Sicherungen', content: <p>Liste</p> }],
        }}
      />,
    );
    expect(screen.getByRole('region', { name: 'Sicherungen' })).toBeVisible();
    const heading = screen.getByRole('heading', { level: 2, name: 'Sicherungen' });
    expect(heading.className).toMatch(/hidden/);
    expect(screen.getByRole('tab', { name: 'Sicherungen' })).toBeVisible();
  });

  it('folds its body away with Enter and Space, keeping what was typed', async () => {
    const user = userEvent.setup();
    render(
      <BandPanel title="E-Mail-Server" collapsible>
        <input aria-label="Server" />
      </BandPanel>,
    );
    const toggle = screen.getByRole('button', { name: 'E-Mail-Server' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const field = screen.getByRole('textbox', { name: 'Server' });
    expect(toggle).toHaveAttribute('aria-controls', field.parentElement?.id);
    await user.type(field, 'smtp.example.test');
    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('textbox', { name: 'Server' })).toBeNull();
    await user.keyboard(' ');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByRole('textbox', { name: 'Server' })).toHaveValue('smtp.example.test');
  });

  it('carries a tab bar in its band', async () => {
    const user = userEvent.setup();
    render(
      <BandPanel
        title="Geräte"
        tabs={{
          label: 'Ansicht',
          tabs: [
            { id: 'devices', label: 'Geräte', count: '4', content: <p>Liste</p> },
            { id: 'caps', label: 'Fähigkeiten', content: <p>Codecs</p> },
          ],
        }}
      />,
    );
    expect(screen.getByRole('region', { name: 'Geräte' })).toBeVisible();
    expect(screen.getByRole('tab', { name: 'Geräte 4' })).toHaveAttribute('aria-selected', 'true');
    await user.click(screen.getByRole('tab', { name: 'Fähigkeiten' }));
    expect(screen.getByText('Codecs')).toBeVisible();
    expect(screen.queryByText('Liste')).toBeNull();
  });
});

// WCAG 2.x relative luminance and contrast ratio (as in packages/design-tokens/test).
function channels(hex: string): [number, number, number] {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
  if (!match) throw new Error(`expected an opaque #rrggbb colour, got ${hex}`);
  return match.slice(1).map((part) => parseInt(part, 16)) as [number, number, number];
}

function luminance(rgb: readonly number[]): number {
  const [r = 0, g = 0, b = 0] = rgb.map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(foreground: readonly number[], background: readonly number[]): number {
  const [light = 0, dark = 0] = [luminance(foreground), luminance(background)].sort(
    (a, b) => b - a,
  );
  return (light + 0.05) / (dark + 0.05);
}

const css = readFileSync(join(import.meta.dirname, 'BandPanel.module.css'), 'utf8');
const BAND_TINT = Number(
  /--sc-band-tint:\s*color-mix\(in srgb, var\(--sc-accent\) (\d+)%, transparent\)/.exec(css)?.[1],
);
const ground = channels(tokens.color.bg.elevated);
const textPrimary = channels(tokens.color.text.primary);

describe('band contrast for every profile accent (concept 11 F-9, at 12 %)', () => {
  it('reads the tint from the stylesheet', () => {
    expect(BAND_TINT).toBe(12);
  });

  it.each(accentNames)('%s: the title meets 4.5:1 and the icon 3:1 on its band', (name) => {
    const accent = channels(tokens.color.accent[name].base);
    const band = accent.map(
      (value, index) => (value * BAND_TINT + (ground[index] ?? 0) * (100 - BAND_TINT)) / 100,
    );
    expect(contrast(textPrimary, band)).toBeGreaterThanOrEqual(4.5);
    // The icon is a graphic next to the title (WCAG 1.4.11); the accent text of the sidebar's
    // band passes 4.5:1 at 16 %, so it does at 12 % as well.
    expect(contrast(accent, band)).toBeGreaterThanOrEqual(4.5);
  });
});
