import { expect, type Locator, type Page } from '@playwright/test';

/** Accessible-ish description of the focused element, for assertions and failure messages. */
export async function focused(
  page: Page,
): Promise<{ tag: string; name: string; href: string | null }> {
  return page.evaluate(() => {
    const element = document.activeElement;
    if (!element || element === document.body) return { tag: 'body', name: '', href: null };
    const label = element instanceof HTMLInputElement ? element.labels?.[0]?.textContent : null;
    const name = element.getAttribute('aria-label') ?? label ?? '';
    return {
      tag: element.tagName.toLowerCase(),
      name: (name || element.textContent || '').trim().replace(/\s+/g, ' '),
      href: element.getAttribute('href'),
    };
  });
}

/** P-15: the focused element shows a ring (computed outline is not `none` and wider than 0). */
export async function expectVisibleFocus(page: Page): Promise<void> {
  const outline = await page.evaluate(() => {
    const element = document.activeElement;
    if (!element || element === document.body) return null;
    const style = getComputedStyle(element);
    return { style: style.outlineStyle, width: parseFloat(style.outlineWidth) };
  });
  expect(outline, 'an element has focus').not.toBeNull();
  expect(outline?.style).not.toBe('none');
  expect(outline?.width ?? 0).toBeGreaterThan(0);
}

/** Expects `locator` to have focus and a visible ring. */
export async function expectFocusOn(page: Page, locator: Locator): Promise<void> {
  await expect(locator).toBeFocused();
  await expectVisibleFocus(page);
}

/** Presses Tab until `locator` has focus; fails after `max` presses. Checks the ring on the way. */
export async function tabTo(page: Page, locator: Locator, max = 40): Promise<void> {
  for (let presses = 0; presses < max; presses += 1) {
    await page.keyboard.press('Tab');
    if (await locator.evaluate((element) => element === document.activeElement)) {
      await expectVisibleFocus(page);
      return;
    }
  }
  throw new Error(`Tab did not reach the element within ${max} presses`);
}

/** Marks the last element of the page's Tab sequence with `data-e2e-last-tabbable`. */
async function markLastTabbable(page: Page): Promise<void> {
  await page.evaluate(() => {
    document.querySelector('[data-e2e-last-tabbable]')?.removeAttribute('data-e2e-last-tabbable');
    const candidates = document.querySelectorAll<HTMLElement>(
      'a[href], button, input, select, textarea, [tabindex]',
    );
    const tabbable = [...candidates].filter((element) => {
      if (element.tabIndex < 0 || element.matches(':disabled')) return false;
      if (element instanceof HTMLInputElement && element.type === 'radio' && !element.checked) {
        return false;
      }
      const style = getComputedStyle(element);
      return (
        style.visibility !== 'hidden' &&
        style.display !== 'none' &&
        element.getClientRects().length > 0
      );
    });
    tabbable.at(-1)?.setAttribute('data-e2e-last-tabbable', '');
  });
}

/**
 * No focus trap (P-15): from the current position, Tab must reach the last element of the page's
 * Tab sequence and then leave the page (Chromium moves focus to the browser UI, headless Firefox
 * keeps it on the last element) or wrap around. A trap cycles inside a region instead.
 */
export async function expectNoFocusTrap(page: Page, max = 80): Promise<void> {
  await markLastTabbable(page);
  const seen: string[] = [];
  for (let presses = 0; presses < max; presses += 1) {
    const atEnd = await page.evaluate(
      () =>
        document.activeElement === document.body ||
        document.activeElement?.hasAttribute('data-e2e-last-tabbable') === true,
    );
    if (atEnd) return;
    await page.keyboard.press('Tab');
    const now = await focused(page);
    seen.push(`${now.tag}:${now.name}`);
  }
  throw new Error(
    `Tab did not reach the end of the page within ${max} presses: ${seen.slice(-10).join(' | ')}`,
  );
}

/** Starts keyboard navigation from the top of the document, as after a fresh page load. */
export async function resetFocus(page: Page): Promise<void> {
  await page.evaluate(() => {
    (document.activeElement as HTMLElement | null)?.blur();
    window.scrollTo(0, 0);
  });
}
