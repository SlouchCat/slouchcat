import { afterEach, describe, expect, it, vi } from 'vitest';
import { afterDialogsClosed, afterRender, focusLost, focusTitleIfLost } from './focusAfter';

afterEach(() => {
  document.body.replaceChildren();
});

function setup() {
  const title = document.createElement('h1');
  title.tabIndex = -1;
  const button = document.createElement('button');
  document.body.append(title, button);
  return { title, button, titleRef: { current: title } };
}

describe('focusTitleIfLost', () => {
  it('moves the focus to the title when it is on the body', () => {
    const { title, titleRef } = setup();
    (document.activeElement as HTMLElement | null)?.blur();
    expect(focusLost()).toBe(true);
    focusTitleIfLost(titleRef);
    expect(document.activeElement).toBe(title);
  });

  it('moves the focus to the title when the focused control was removed', () => {
    const { title, button, titleRef } = setup();
    button.focus();
    button.remove();
    focusTitleIfLost(titleRef);
    expect(document.activeElement).toBe(title);
  });

  it('moves the focus to the title when the focused control turned disabled', () => {
    const { title, button, titleRef } = setup();
    button.focus();
    button.disabled = true;
    // jsdom keeps the focus on a control that turned disabled, as Firefox does.
    expect(document.activeElement).toBe(button);
    expect(focusLost()).toBe(true);
    focusTitleIfLost(titleRef);
    expect(document.activeElement).toBe(title);
  });

  it('leaves a focus on a live control alone', () => {
    const { button, titleRef } = setup();
    button.focus();
    expect(focusLost()).toBe(false);
    focusTitleIfLost(titleRef);
    expect(document.activeElement).toBe(button);
  });

  it('does nothing without a title', () => {
    setup();
    expect(() => focusTitleIfLost({ current: null })).not.toThrow();
  });
});

describe('afterRender and afterDialogsClosed', () => {
  it('afterRender resolves after a task and a frame', async () => {
    let done = false;
    const pending = afterRender().then(() => {
      done = true;
    });
    expect(done).toBe(false);
    await pending;
    expect(done).toBe(true);
  });

  it('afterRender does not wait for a frame that never comes (a hidden tab)', async () => {
    const frame = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => 0);
    try {
      const started = Date.now();
      await afterRender();
      expect(Date.now() - started).toBeLessThan(1000);
    } finally {
      frame.mockRestore();
    }
  });

  it('afterDialogsClosed waits until no overlay is exiting any more', async () => {
    const overlay = document.createElement('div');
    overlay.setAttribute('data-exiting', 'true');
    document.body.append(overlay);
    let done = false;
    const pending = afterDialogsClosed().then(() => {
      done = true;
    });
    for (let round = 0; round < 3; round += 1) await afterRender();
    expect(done).toBe(false);
    overlay.remove();
    await pending;
    expect(done).toBe(true);
  });

  it('afterDialogsClosed gives up after about one second', async () => {
    const overlay = document.createElement('div');
    overlay.setAttribute('data-exiting', 'true');
    document.body.append(overlay);
    await expect(afterDialogsClosed()).resolves.toBeUndefined();
  });
});
