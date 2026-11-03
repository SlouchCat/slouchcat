import type userEvent from '@testing-library/user-event';

/**
 * Puts `text` into a field by paste: one input event instead of one per key. On a page that
 * re-renders a lot per key (all blocks of Integrationen, the account forms), typing key by key
 * took most of the 5 s test timeout on a heavily loaded machine. Use it where a test is about
 * what is stored and shown; keep `user.type` where the per-key behaviour is under test.
 */
export async function fill(
  user: ReturnType<typeof userEvent.setup>,
  field: HTMLElement,
  text: string,
): Promise<void> {
  await user.clear(field);
  await user.click(field);
  await user.paste(text);
}
