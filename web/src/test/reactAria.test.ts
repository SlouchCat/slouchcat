import { realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import { describe, expect, it } from 'vitest';

// setup.ts tears down react-aria's live announcer after every test with destroyAnnouncer, imported
// from `react-aria`, a direct devDependency for exactly that. The announcer is a module variable:
// the teardown only reaches it if `react-aria` here is the very copy react-aria-components uses.
// A version bump of one of the two without the other would give two copies, and the teardown would
// quietly miss the announcer the components write to ("document is not defined" again).
describe('react-aria', () => {
  it('is the copy react-aria-components resolves', () => {
    const own = createRequire(import.meta.url);
    const components = createRequire(own.resolve('react-aria-components'));
    const direct = realpathSync(own.resolve('react-aria/package.json'));
    const viaComponents = realpathSync(components.resolve('react-aria/package.json'));
    expect(direct).toBe(viaComponents);
  });
});
