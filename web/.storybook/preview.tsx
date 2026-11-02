import '@slouchcat/design-tokens/tokens.css';
import '@/ui/global.css';
// The compact density of the administration (ADR 0020); the app loads it from K12-8 on.
import '@/ui/density.css';
import '@/i18n';
import type { Preview } from '@storybook/react-vite';
import { setLanguage } from '@/i18n';
import { loadSpatialNavigation } from '@/ui';
// Straight from its module: the '@/ui/aria' barrel would load every stylesheet before the stories
// and change the order of the cascade.
import { AdminDensityScope } from '@/ui/adminDensity';

const preview: Preview = {
  parameters: {
    // The app is dark only; global.css paints the body. Storybook's own backgrounds stay off.
    backgrounds: { disable: true },
    layout: 'padded',
    controls: { disable: true },
    actions: { disable: true },
  },
  // Density of the story (concept 12, ADR 0020): `default` is today's size everywhere; a story
  // with `globals: { density: 'compact' }` renders the way an administration page will from K12-8:
  // <html> carries data-shell="admin" like /admin, its root data-density="compact" (ui/density.css
  // maps the component tokens), and the AdminDensityScope puts the same attribute on the portals
  // of its dialogs and menus.
  globalTypes: {
    density: {
      description: 'Density of the administration (ADR 0020)',
      toolbar: { items: ['default', 'compact'] },
    },
  },
  initialGlobals: { density: 'default' },
  decorators: [
    (Story, context) => {
      const compact = context.globals.density === 'compact';
      if (compact) document.documentElement.dataset.shell = 'admin';
      else delete document.documentElement.dataset.shell;
      return compact ? (
        <div data-density="compact" style={{ display: 'contents' }}>
          <AdminDensityScope>
            <Story />
          </AdminDensityScope>
        </div>
      ) : (
        <Story />
      );
    },
  ],
  loaders: [
    async () => {
      // German like the e2e suite; spatial components need norigin before they render.
      await setLanguage('de');
      await loadSpatialNavigation();
      return {};
    },
  ],
};

export default preview;
