import type { Locator } from '@playwright/test';

/**
 * Clicks a react-aria radio or checkbox through its label: the real input is visually hidden, so
 * a pointer can only reach the label around it.
 */
export async function choose(
  scope: Locator,
  role: 'radio' | 'checkbox',
  name: string,
): Promise<void> {
  await scope
    .locator('label')
    .filter({ has: scope.page().getByRole(role, { name, exact: true }) })
    .click();
}
