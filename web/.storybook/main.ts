import type { StorybookConfig } from '@storybook/react-vite';

// Storybook for the shared UI (web/src/ui, A-2): one story file per component, screenshot-checked
// by e2e-storybook/ against committed baselines. No addons: stories only, no docs pages.
const config: StorybookConfig = {
  stories: ['../src/ui/**/*.stories.tsx'],
  staticDirs: ['./public'],
  framework: {
    name: '@storybook/react-vite',
    // A Vite config of its own: the app's (PWA, start-script loader, one-chunk output) is for the
    // shell, not for the story iframe. Relative to web/, where every task runs.
    options: { builder: { viteConfigPath: '.storybook/vite.config.ts' } },
  },
  core: { disableTelemetry: true, disableWhatsNewNotifications: true },
  // react-docgen feeds the docs addon, which is not installed.
  typescript: { reactDocgen: false },
};

export default config;
