import { screen, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';

/**
 * Opens every closed collapsible the page shows (concept 24, P-3: all of them start closed): the
 * toggle in a heading, as DisclosureGroup and a collapsible BandPanel write it. Runs until none is
 * left, so a group inside a band that just opened opens as well. Row toggles of a DataTable and
 * menu buttons are not in a heading and stay as they are.
 */
export async function openFolded(user: UserEvent) {
  for (;;) {
    const closed = screen
      .queryAllByRole('heading')
      .flatMap((heading) => within(heading).queryAllByRole('button', { expanded: false }));
    const [first] = closed;
    if (!first) return;
    await user.click(first);
  }
}
